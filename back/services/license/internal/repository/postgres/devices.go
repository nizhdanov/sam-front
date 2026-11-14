package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Devices struct{ pool *pgxpool.Pool }

func NewDevices(pool *pgxpool.Pool) *Devices { return &Devices{pool: pool} }

const deviceSelect = `
	SELECT d.id, d.serial_number, d.hostname, d.employee_id, e.full_name, e.department, d.operating_system,
	       d.last_inventory_at,
	       (SELECT COUNT(*) FROM installations i WHERE i.device_id = d.id AND i.is_active),
	       (SELECT COUNT(*) FROM compliance_findings f WHERE f.device_id = d.id AND f.status <> 'RESOLVED'),
	       d.created_at, d.updated_at
	FROM devices d
	LEFT JOIN employees e ON e.id = d.employee_id`

func scanDevice(row scanner) (domain.Device, error) {
	var d domain.Device
	err := row.Scan(&d.ID, &d.SerialNumber, &d.Hostname, &d.EmployeeID, &d.EmployeeName, &d.Department,
		&d.OperatingSystem, &d.LastInventoryAt, &d.Installations, &d.OpenFindings, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (r *Devices) List(ctx context.Context, f domain.DeviceFilter, page httpx.PageRequest) ([]domain.Device, int, error) {
	var w where
	if f.Search != "" {
		w.add("(d.serial_number ILIKE ? OR d.hostname ILIKE ? OR e.full_name ILIKE ?)", "%"+f.Search+"%")
	}
	if f.SerialNumber != "" {
		w.add("d.serial_number = ?", f.SerialNumber)
	}
	if f.EmployeeID != nil {
		w.add("d.employee_id = ?", *f.EmployeeID)
	}
	if f.Department != "" {
		w.add("e.department = ?", f.Department)
	}
	if f.HasViolations != nil {
		cond := "EXISTS (SELECT 1 FROM compliance_findings f WHERE f.device_id = d.id AND f.status <> 'RESOLVED')"
		if !*f.HasViolations {
			cond = "NOT " + cond
		}
		w.raw(cond)
	}
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM devices d LEFT JOIN employees e ON e.id = d.employee_id`+w.String(), w.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count devices: %w", err)
	}
	query := deviceSelect + w.String() + ` ORDER BY d.serial_number LIMIT ` + w.arg(page.Limit) + ` OFFSET ` + w.arg(page.Offset())
	rows, err := db.Query(ctx, query, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list devices: %w", err)
	}
	items, err := collect(rows, scanDevice)
	return items, total, err
}

func (r *Devices) Get(ctx context.Context, id uuid.UUID) (domain.Device, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, deviceSelect+` WHERE d.id = $1`, id)
	return one(rows, err, scanDevice, "device")
}

func (r *Devices) LockBySerial(ctx context.Context, serial string) (uuid.UUID, *time.Time, error) {
	db := pg.DB(ctx, r.pool)
	if _, err := db.Exec(ctx, `INSERT INTO devices (id, serial_number) VALUES ($1, $2) ON CONFLICT (serial_number) DO NOTHING`,
		uuid.New(), serial); err != nil {
		return uuid.Nil, nil, fmt.Errorf("ensure device: %w", err)
	}
	var id uuid.UUID
	var last *time.Time
	if err := db.QueryRow(ctx, `SELECT id, last_inventory_at FROM devices WHERE serial_number = $1 FOR UPDATE`, serial).Scan(&id, &last); err != nil {
		return uuid.Nil, nil, fmt.Errorf("lock device: %w", err)
	}
	return id, last, nil
}

func (r *Devices) MarkInventoried(ctx context.Context, id uuid.UUID, at time.Time, snapshotID uuid.UUID) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `UPDATE devices SET last_inventory_at = $2, last_snapshot_id = $3, updated_at = NOW() WHERE id = $1`,
		id, at, snapshotID)
	if err != nil {
		return fmt.Errorf("mark device inventoried: %w", err)
	}
	return nil
}

func (r *Devices) Update(ctx context.Context, id uuid.UUID, p domain.DevicePatch) error {
	var s set
	if p.Hostname != nil {
		s.add("hostname", nonEmpty(*p.Hostname))
	}
	if p.OperatingSystem != nil {
		s.add("operating_system", nonEmpty(*p.OperatingSystem))
	}
	if p.EmployeeID.Set {
		s.add("employee_id", p.EmployeeID.Value)
	}
	if s.empty() {
		return nil
	}
	query, args := s.update("devices", id)
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, query, args...)
	if pg.IsForeignKeyViolation(err) {
		return apperr.Invalid("employee does not exist")
	}
	if err != nil {
		return fmt.Errorf("update device: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("device not found")
	}
	return nil
}

func (r *Devices) MarkProcessed(ctx context.Context, inventoryID, deviceID uuid.UUID, collectedAt time.Time, result domain.SyncResult) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO processed_snapshots (inventory_id, device_id, collected_at, result_status) VALUES ($1, $2, $3, $4)`,
		inventoryID, deviceID, collectedAt, result)
	if err != nil {
		return fmt.Errorf("mark snapshot processed: %w", err)
	}
	return nil
}

func (r *Devices) Processed(ctx context.Context, inventoryID uuid.UUID) (bool, error) {
	var exists bool
	err := pg.DB(ctx, r.pool).QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM processed_snapshots WHERE inventory_id = $1)`, inventoryID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check processed snapshot: %w", err)
	}
	return exists, nil
}
