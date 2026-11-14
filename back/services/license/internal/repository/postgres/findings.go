package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Findings struct{ pool *pgxpool.Pool }

func NewFindings(pool *pgxpool.Pool) *Findings { return &Findings{pool: pool} }

const findingSelect = `
	SELECT f.id, f.finding_type, f.severity, f.status, f.device_id, d.serial_number, f.product_id, p.canonical_name,
	       f.installation_id, f.details, f.acknowledged_by, f.acknowledge_comment, f.acknowledged_at, f.created_at, f.last_detected_at, f.resolved_at
	FROM compliance_findings f
	LEFT JOIN devices d ON d.id = f.device_id
	LEFT JOIN software_products p ON p.id = f.product_id`

func scanFinding(row scanner) (domain.Finding, error) {
	var f domain.Finding
	var details []byte
	err := row.Scan(&f.ID, &f.Type, &f.Severity, &f.Status, &f.DeviceID, &f.SerialNumber, &f.ProductID, &f.ProductName,
		&f.InstallationID, &details, &f.AcknowledgedBy, &f.AckComment, &f.AcknowledgedAt, &f.CreatedAt, &f.LastDetectedAt, &f.ResolvedAt)
	if err != nil {
		return f, err
	}
	return f, json.Unmarshal(details, &f.Details)
}

func findingWhere(f domain.FindingFilter) where {
	var w where
	if len(f.Types) > 0 {
		w.add("f.finding_type = ANY(?)", f.Types)
	}
	if f.Severity != "" {
		w.add("f.severity = ?", f.Severity)
	}
	if f.Status != "" {
		w.add("f.status = ?", f.Status)
	} else {
		w.raw("f.status <> 'RESOLVED'")
	}
	if f.DeviceID != nil {
		w.add("f.device_id = ?", *f.DeviceID)
	}
	if f.ProductID != nil {
		w.add("f.product_id = ?", *f.ProductID)
	}
	if f.SerialNumber != "" {
		w.add("d.serial_number = ?", f.SerialNumber)
	}
	return w
}

func (r *Findings) List(ctx context.Context, f domain.FindingFilter, page httpx.PageRequest) ([]domain.Finding, int, error) {
	w := findingWhere(f)
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM compliance_findings f LEFT JOIN devices d ON d.id = f.device_id`+w.String(), w.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count findings: %w", err)
	}
	query := findingSelect + w.String() + `
		ORDER BY CASE f.severity WHEN 'CRITICAL' THEN 0 WHEN 'HIGH' THEN 1 WHEN 'MEDIUM' THEN 2 ELSE 3 END, f.last_detected_at DESC
		LIMIT ` + w.arg(page.Limit) + ` OFFSET ` + w.arg(page.Offset())
	rows, err := db.Query(ctx, query, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list findings: %w", err)
	}
	items, err := collect(rows, scanFinding)
	return items, total, err
}

func (r *Findings) Get(ctx context.Context, id uuid.UUID) (domain.Finding, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, findingSelect+` WHERE f.id = $1`, id)
	return one(rows, err, scanFinding, "finding")
}

func (r *Findings) Acknowledge(ctx context.Context, id uuid.UUID, by, comment string) error {
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, `
		UPDATE compliance_findings
		SET status = 'ACKNOWLEDGED', acknowledged_by = $2, acknowledge_comment = $3, acknowledged_at = NOW()
		WHERE id = $1 AND status = 'OPEN'`, id, by, comment)
	if err != nil {
		return fmt.Errorf("acknowledge finding: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.Conflict("finding is not open")
	}
	return nil
}

func (r *Findings) Summary(ctx context.Context, f domain.FindingFilter) (domain.ComplianceSummary, error) {
	w := findingWhere(f)
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		SELECT f.status, f.severity, f.finding_type, COUNT(*)
		FROM compliance_findings f LEFT JOIN devices d ON d.id = f.device_id`+w.String()+`
		GROUP BY f.status, f.severity, f.finding_type`, w.args...)
	if err != nil {
		return domain.ComplianceSummary{}, fmt.Errorf("findings summary: %w", err)
	}
	defer rows.Close()
	s := domain.ComplianceSummary{BySeverity: map[domain.Severity]int{}, ByType: map[domain.FindingType]int{}}
	for rows.Next() {
		var status domain.FindingStatus
		var severity domain.Severity
		var typ domain.FindingType
		var n int
		if err := rows.Scan(&status, &severity, &typ, &n); err != nil {
			return s, err
		}
		switch status {
		case domain.FindingOpen:
			s.Open += n
		case domain.FindingAcknowledged:
			s.Acknowledged += n
		}
		s.BySeverity[severity] += n
		s.ByType[typ] += n
	}
	return s, rows.Err()
}

func (r *Findings) Apply(ctx context.Context, desired []domain.Finding, types []domain.FindingType, s domain.Scope, now time.Time) (int64, error) {
	db := pg.DB(ctx, r.pool)
	keys := make([]string, 0, len(desired))
	if len(desired) > 0 {
		batch := &pgx.Batch{}
		for _, f := range desired {
			details, err := json.Marshal(f.Details)
			if err != nil {
				return 0, err
			}
			keys = append(keys, f.Key)
			batch.Queue(`
				INSERT INTO compliance_findings
					(id, finding_key, finding_type, severity, status, device_id, product_id, installation_id, details, last_detected_at)
				VALUES ($1, $2, $3, $4, 'OPEN', $5, $6, $7, $8, $9)
				ON CONFLICT (finding_key) DO UPDATE SET
					severity = EXCLUDED.severity,
					details = EXCLUDED.details,
					device_id = EXCLUDED.device_id,
					product_id = EXCLUDED.product_id,
					last_detected_at = EXCLUDED.last_detected_at,
					status = CASE WHEN compliance_findings.status = 'RESOLVED' THEN 'OPEN' ELSE compliance_findings.status END,
					acknowledged_by = CASE WHEN compliance_findings.status = 'RESOLVED' THEN NULL ELSE compliance_findings.acknowledged_by END,
					acknowledge_comment = CASE WHEN compliance_findings.status = 'RESOLVED' THEN NULL ELSE compliance_findings.acknowledge_comment END,
					acknowledged_at = CASE WHEN compliance_findings.status = 'RESOLVED' THEN NULL ELSE compliance_findings.acknowledged_at END,
					resolved_at = NULL`,
				uuid.New(), f.Key, f.Type, f.Severity, f.DeviceID, f.ProductID, f.InstallationID, details, now)
		}
		if err := db.SendBatch(ctx, batch).Close(); err != nil {
			return 0, fmt.Errorf("upsert findings: %w", err)
		}
	}

	var w where
	w.raw("status <> 'RESOLVED'")
	w.add("finding_type = ANY(?)", types)
	w.add("NOT (finding_key = ANY(?))", keys)
	if !s.All {
		deviceArg, productArg := w.arg(uuids(s.DeviceIDs)), w.arg(uuids(s.ProductIDs))
		w.raw("(device_id = ANY(" + deviceArg + ") OR product_id = ANY(" + productArg + "))")
	}
	tag, err := db.Exec(ctx, `UPDATE compliance_findings SET status = 'RESOLVED', resolved_at = `+w.arg(now)+w.String(), w.args...)
	if err != nil {
		return 0, fmt.Errorf("resolve findings: %w", err)
	}
	return tag.RowsAffected(), nil
}
