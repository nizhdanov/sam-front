package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Analytics struct{ pool *pgxpool.Pool }

func NewAnalytics(pool *pgxpool.Pool) *Analytics { return &Analytics{pool: pool} }

func (r *Analytics) Dashboard(ctx context.Context, expiringWithinDays int) (domain.Dashboard, error) {
	d := domain.Dashboard{ExpiringWithinDays: expiringWithinDays}
	err := pg.DB(ctx, r.pool).QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM devices),
			(SELECT COUNT(*) FROM devices WHERE status = 'NEW'),
			(SELECT COUNT(*) FROM installations WHERE is_active),
			(SELECT COUNT(*) FROM installations WHERE is_active AND product_id IS NOT NULL),
			(SELECT COUNT(*) FROM matching_candidates WHERE status = 'PENDING'),
			(SELECT COUNT(*) FROM compliance_findings WHERE status = 'OPEN' AND finding_type = 'FORBIDDEN_SOFTWARE'),
			(SELECT COUNT(DISTINCT device_id) FROM compliance_findings WHERE status = 'OPEN' AND finding_type = 'FORBIDDEN_SOFTWARE'),
			(SELECT COUNT(*) FROM compliance_findings WHERE status = 'OPEN' AND finding_type = 'OUTDATED_SOFTWARE'),
			(SELECT COUNT(DISTINCT device_id) FROM compliance_findings WHERE status = 'OPEN' AND finding_type = 'OUTDATED_SOFTWARE'),
			(SELECT COUNT(*) FROM license_entitlements
			 WHERE is_active AND expires_at > NOW() AND expires_at <= NOW() + make_interval(days => $1))`, expiringWithinDays).
		Scan(&d.Devices, &d.NewDevices, &d.Installations, &d.Recognized, &d.PendingReview,
			&d.Forbidden.Installations, &d.Forbidden.Devices, &d.Outdated.Installations, &d.Outdated.Devices,
			&d.ExpiringContracts)
	if err != nil {
		return d, fmt.Errorf("dashboard: %w", err)
	}
	if d.Installations > 0 {
		d.RecognizedPercent = float64(d.Recognized*10000/d.Installations) / 100
	}
	return d, nil
}

func (r *Analytics) FreeSoftware(ctx context.Context) ([]domain.FreeSoftware, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		SELECT p.id::text, p.canonical_name, p.publisher, p.in_bpo, p.status, p.current_version,
		       (SELECT COUNT(DISTINCT i.device_id) FROM installations i WHERE i.product_id = p.id AND i.is_active),
		       (SELECT COUNT(DISTINCT f.device_id) FROM compliance_findings f
		        WHERE f.product_id = p.id AND f.finding_type = 'OUTDATED_SOFTWARE' AND f.status <> 'RESOLVED')
		FROM software_products p
		LEFT JOIN license_agreements a ON a.id = p.agreement_id
		WHERE p.archived_at IS NULL AND (a.cost_model = 'FREE' OR p.in_bpo)
		ORDER BY p.canonical_name`)
	if err != nil {
		return nil, fmt.Errorf("free software: %w", err)
	}
	return collect(rows, func(row scanner) (domain.FreeSoftware, error) {
		var f domain.FreeSoftware
		err := row.Scan(&f.ProductID, &f.ProductName, &f.Publisher, &f.InBPO, &f.Status, &f.CurrentVersion, &f.Devices, &f.OutdatedDevices)
		return f, err
	})
}
