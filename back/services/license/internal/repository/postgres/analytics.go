package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Analytics struct{ pool *pgxpool.Pool }

func NewAnalytics(pool *pgxpool.Pool) *Analytics { return &Analytics{pool: pool} }

func (r *Analytics) Dashboard(ctx context.Context, employeeID *uuid.UUID) (domain.Dashboard, error) {
	var d domain.Dashboard
	db := pg.DB(ctx, r.pool)
	err := db.QueryRow(ctx, `
		WITH scoped AS (
			SELECT id, employee_id FROM devices WHERE $1::uuid IS NULL OR employee_id = $1
		)
		SELECT
			(SELECT COUNT(*) FROM scoped),
			(SELECT COUNT(*) FROM scoped WHERE employee_id IS NOT NULL),
			(SELECT COUNT(DISTINCT i.product_id) FROM installations i JOIN scoped s ON s.id = i.device_id WHERE i.is_active),
			(SELECT COUNT(*) FROM installations i JOIN scoped s ON s.id = i.device_id WHERE i.is_active),
			(SELECT COUNT(*) FROM installations i JOIN scoped s ON s.id = i.device_id WHERE i.is_active AND i.product_id IS NULL),
			(SELECT COUNT(*) FROM matching_candidates WHERE status = 'PENDING' AND $1::uuid IS NULL)`, employeeID).
		Scan(&d.Devices.Total, &d.Devices.Assigned, &d.Software.ProductsTotal, &d.Software.InstallationsTotal,
			&d.Software.UnknownInstallations, &d.Software.PendingReview)
	if err != nil {
		return d, fmt.Errorf("dashboard: %w", err)
	}
	d.Devices.Unassigned = d.Devices.Total - d.Devices.Assigned

	rows, err := db.Query(ctx, `
		SELECT f.severity, COUNT(*)
		FROM compliance_findings f LEFT JOIN devices dv ON dv.id = f.device_id
		WHERE f.status <> 'RESOLVED' AND ($1::uuid IS NULL OR (dv.employee_id = $1 AND f.finding_type <> ALL($2)))
		GROUP BY f.severity`, employeeID, domain.ProductFindingTypes)
	if err != nil {
		return d, fmt.Errorf("dashboard findings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var severity domain.Severity
		var n int
		if err := rows.Scan(&severity, &n); err != nil {
			return d, err
		}
		d.Compliance.OpenFindings += n
		switch severity {
		case domain.SeverityCritical:
			d.Compliance.Critical = n
		case domain.SeverityHigh:
			d.Compliance.High = n
		case domain.SeverityMedium:
			d.Compliance.Medium = n
		case domain.SeverityLow:
			d.Compliance.Low = n
		}
	}
	return d, rows.Err()
}

func (r *Analytics) Software(ctx context.Context, limit int) ([]domain.SoftwareUsage, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		SELECT p.id, p.canonical_name, p.publisher, p.policy_status, COUNT(DISTINCT i.device_id), COUNT(i.id)
		FROM software_products p
		JOIN installations i ON i.product_id = p.id AND i.is_active
		GROUP BY p.id
		ORDER BY COUNT(DISTINCT i.device_id) DESC, p.canonical_name
		LIMIT $1`, limit)
	if err != nil {
		return nil, fmt.Errorf("software analytics: %w", err)
	}
	return collect(rows, func(row scanner) (domain.SoftwareUsage, error) {
		var s domain.SoftwareUsage
		err := row.Scan(&s.ProductID, &s.Name, &s.Publisher, &s.PolicyStatus, &s.Devices, &s.Installations)
		return s, err
	})
}

func (r *Analytics) Departments(ctx context.Context) ([]domain.DepartmentStats, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		SELECT COALESCE(e.department, 'Unassigned') AS department,
		       COUNT(DISTINCT e.id),
		       COUNT(DISTINCT d.id),
		       COUNT(DISTINCT i.id),
		       COUNT(DISTINCT f.id)
		FROM devices d
		LEFT JOIN employees e ON e.id = d.employee_id
		LEFT JOIN installations i ON i.device_id = d.id AND i.is_active
		LEFT JOIN compliance_findings f ON f.device_id = d.id AND f.status <> 'RESOLVED'
		GROUP BY 1
		ORDER BY 1`)
	if err != nil {
		return nil, fmt.Errorf("department analytics: %w", err)
	}
	return collect(rows, func(row scanner) (domain.DepartmentStats, error) {
		var s domain.DepartmentStats
		err := row.Scan(&s.Department, &s.Employees, &s.Devices, &s.Installations, &s.OpenFindings)
		return s, err
	})
}
