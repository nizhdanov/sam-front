package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Employees struct{ pool *pgxpool.Pool }

func NewEmployees(pool *pgxpool.Pool) *Employees { return &Employees{pool: pool} }

const employeeSelect = `
	SELECT e.id, e.external_id, e.full_name, e.department, e.position, e.email, e.is_active,
	       (SELECT COUNT(*) FROM devices d WHERE d.employee_id = e.id),
	       e.created_at, e.updated_at
	FROM employees e`

func scanEmployee(row scanner) (domain.Employee, error) {
	var e domain.Employee
	err := row.Scan(&e.ID, &e.ExternalID, &e.FullName, &e.Department, &e.Position, &e.Email, &e.IsActive, &e.Devices,
		&e.CreatedAt, &e.UpdatedAt)
	return e, err
}

func (r *Employees) List(ctx context.Context, search, department string, page httpx.PageRequest) ([]domain.Employee, int, error) {
	var w where
	if search != "" {
		w.add("(e.full_name ILIKE ? OR e.email ILIKE ?)", "%"+search+"%")
	}
	if department != "" {
		w.add("e.department = ?", department)
	}
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM employees e`+w.String(), w.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count employees: %w", err)
	}
	query := employeeSelect + w.String() + ` ORDER BY e.full_name LIMIT ` + w.arg(page.Limit) + ` OFFSET ` + w.arg(page.Offset())
	rows, err := db.Query(ctx, query, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list employees: %w", err)
	}
	items, err := collect(rows, scanEmployee)
	return items, total, err
}

func (r *Employees) Get(ctx context.Context, id uuid.UUID) (domain.Employee, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, employeeSelect+` WHERE e.id = $1`, id)
	return one(rows, err, scanEmployee, "employee")
}

func (r *Employees) Update(ctx context.Context, id uuid.UUID, p domain.EmployeePatch) error {
	var s set
	if p.FullName != nil {
		s.add("full_name", strings.TrimSpace(*p.FullName))
	}
	if p.Department != nil {
		s.add("department", nonEmpty(*p.Department))
	}
	if p.Position != nil {
		s.add("position", nonEmpty(*p.Position))
	}
	if p.Email != nil {
		s.add("email", nonEmpty(*p.Email))
	}
	if p.IsActive != nil {
		s.add("is_active", *p.IsActive)
	}
	if s.empty() {
		return nil
	}
	query, args := s.update("employees", id)
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update employee: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("employee not found")
	}
	return nil
}
