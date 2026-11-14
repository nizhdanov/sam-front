package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Installations struct{ pool *pgxpool.Pool }

func NewInstallations(pool *pgxpool.Pool) *Installations { return &Installations{pool: pool} }

func (r *Installations) ActiveProductIDs(ctx context.Context, deviceID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		SELECT DISTINCT product_id FROM installations WHERE device_id = $1 AND is_active AND product_id IS NOT NULL`, deviceID)
	if err != nil {
		return nil, fmt.Errorf("device products: %w", err)
	}
	return collect(rows, scanID)
}

func (r *Installations) CountActive(ctx context.Context, deviceID uuid.UUID) (int, error) {
	var n int
	err := pg.DB(ctx, r.pool).QueryRow(ctx, `SELECT COUNT(*) FROM installations WHERE device_id = $1 AND is_active`, deviceID).Scan(&n)
	return n, err
}

func (r *Installations) Upsert(ctx context.Context, deviceID uuid.UUID, seenAt time.Time, items []domain.InstallationInput) error {
	batch := &pgx.Batch{}
	for _, it := range items {
		batch.Queue(`
			INSERT INTO installations (id, device_id, product_id, version_id, source_key, raw_name, raw_publisher, raw_version,
			                           match_status, is_active, first_seen_at, last_seen_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, TRUE, $10, $10)
			ON CONFLICT (device_id, source_key) DO UPDATE SET
				product_id = EXCLUDED.product_id,
				version_id = EXCLUDED.version_id,
				match_status = EXCLUDED.match_status,
				is_active = TRUE,
				removed_at = NULL,
				last_seen_at = GREATEST(installations.last_seen_at, EXCLUDED.last_seen_at)`,
			uuid.New(), deviceID, it.ProductID, it.VersionID, it.SourceKey, it.RawName,
			nonEmpty(it.RawPublisher), nonEmpty(it.RawVersion), it.MatchStatus, seenAt)
	}
	if err := pg.DB(ctx, r.pool).SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("upsert installations: %w", err)
	}
	return nil
}

func (r *Installations) DeactivateMissing(ctx context.Context, deviceID uuid.UUID, keep []string, at time.Time) error {
	if keep == nil {
		keep = []string{}
	}
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		UPDATE installations SET is_active = FALSE, removed_at = $3
		WHERE device_id = $1 AND is_active AND NOT (source_key = ANY($2))`, deviceID, keep, at)
	if err != nil {
		return fmt.Errorf("deactivate removed installations: %w", err)
	}
	return nil
}

func (r *Installations) Assign(ctx context.Context, key string, productID uuid.UUID, versionID *uuid.UUID) ([]uuid.UUID, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		UPDATE installations SET product_id = $2, version_id = $3, match_status = 'MATCHED'
		WHERE source_key = $1 RETURNING device_id`, key, productID, versionID)
	if err != nil {
		return nil, fmt.Errorf("assign installations: %w", err)
	}
	return collect(rows, scanID)
}

func (r *Installations) Unassign(ctx context.Context, key string, status domain.MatchStatus) ([]uuid.UUID, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		UPDATE installations SET product_id = NULL, version_id = NULL, match_status = $2
		WHERE source_key = $1 RETURNING device_id`, key, status)
	if err != nil {
		return nil, fmt.Errorf("unassign installations: %w", err)
	}
	return collect(rows, scanID)
}

func (r *Installations) Facts(ctx context.Context, s domain.Scope) ([]domain.InstallationFacts, error) {
	query := `
		SELECT i.id, i.device_id, i.product_id, p.canonical_name, p.status, p.current_version, v.lifecycle_status, v.version,
		       a.commercial_allowed, i.raw_name, i.raw_version, i.match_status
		FROM installations i
		LEFT JOIN software_products p ON p.id = i.product_id
		LEFT JOIN software_versions v ON v.id = i.version_id
		LEFT JOIN license_agreements a ON a.id = p.agreement_id
		WHERE i.is_active`
	var args []any
	if !s.All {
		query += ` AND (i.device_id = ANY($1) OR i.product_id = ANY($2))`
		args = append(args, uuids(s.DeviceIDs), uuids(s.ProductIDs))
	}
	rows, err := pg.DB(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("installation facts: %w", err)
	}
	return collect(rows, func(row scanner) (domain.InstallationFacts, error) {
		var f domain.InstallationFacts
		err := row.Scan(&f.ID, &f.DeviceID, &f.ProductID, &f.ProductName, &f.Status, &f.CurrentVersion, &f.Lifecycle, &f.Version,
			&f.CommercialAllowed, &f.RawName, &f.RawVersion, &f.MatchStatus)
		return f, err
	})
}

func (r *Installations) ProductFacts(ctx context.Context, s domain.Scope) ([]domain.ProductFacts, error) {
	query := `
		SELECT p.id, p.canonical_name, a.cost_model, COUNT(DISTINCT i.device_id)
		FROM software_products p
		LEFT JOIN license_agreements a ON a.id = p.agreement_id
		LEFT JOIN installations i ON i.product_id = p.id AND i.is_active`
	var args []any
	if !s.All {
		query += ` WHERE p.id = ANY($1)`
		args = append(args, uuids(s.ProductIDs))
	}
	query += ` GROUP BY p.id, p.canonical_name, a.cost_model ORDER BY p.canonical_name`
	rows, err := pg.DB(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("product usage: %w", err)
	}
	return collect(rows, func(row scanner) (domain.ProductFacts, error) {
		var p domain.ProductFacts
		err := row.Scan(&p.ID, &p.Name, &p.CostModel, &p.Devices)
		return p, err
	})
}

func (r *Installations) DeactivateByKey(ctx context.Context, key string, at time.Time) ([]uuid.UUID, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		UPDATE installations SET is_active = FALSE, removed_at = $2
		WHERE source_key = $1 AND is_active RETURNING device_id`, key, at)
	if err != nil {
		return nil, fmt.Errorf("deactivate installations: %w", err)
	}
	return collect(rows, scanID)
}
