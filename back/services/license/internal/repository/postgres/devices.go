package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Devices struct{ pool *pgxpool.Pool }

func NewDevices(pool *pgxpool.Pool) *Devices { return &Devices{pool: pool} }

const deviceSelect = `
	SELECT d.id, d.serial_number, d.device_type, d.status, d.hostname, d.operating_system, d.responsible, d.location,
	       d.last_inventory_at,
	       (SELECT COUNT(*) FROM installations i WHERE i.device_id = d.id AND i.is_active),
	       (SELECT COUNT(*) FROM compliance_findings f WHERE f.device_id = d.id AND f.status = 'OPEN'),
	       d.created_at, d.updated_at
	FROM devices d`

func scanDevice(row scanner) (domain.Device, error) {
	var d domain.Device
	err := row.Scan(&d.ID, &d.SerialNumber, &d.Type, &d.Status, &d.Hostname, &d.OperatingSystem, &d.Responsible,
		&d.Location, &d.LastInventoryAt, &d.Installations, &d.OpenFindings, &d.CreatedAt, &d.UpdatedAt)
	return d, err
}

func (r *Devices) Get(ctx context.Context, id uuid.UUID) (domain.Device, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, deviceSelect+` WHERE d.id = $1`, id)
	return one(rows, err, scanDevice, "device")
}

func (r *Devices) BySerial(ctx context.Context, serial string) (domain.Device, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, deviceSelect+` WHERE d.serial_number = $1`, serial)
	return one(rows, err, scanDevice, "device")
}

func (r *Devices) History(ctx context.Context, id uuid.UUID) ([]domain.DeviceImport, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		SELECT inventory_id, collected_at, result_status, processed_at
		FROM processed_snapshots WHERE device_id = $1 ORDER BY collected_at DESC, processed_at DESC`, id)
	if err != nil {
		return nil, fmt.Errorf("device history: %w", err)
	}
	return collect(rows, func(row scanner) (domain.DeviceImport, error) {
		var h domain.DeviceImport
		err := row.Scan(&h.InventoryID, &h.CollectedAt, &h.Result, &h.ProcessedAt)
		return h, err
	})
}

func (r *Devices) LockBySerial(ctx context.Context, serial string) (uuid.UUID, *time.Time, error) {
	db := pg.DB(ctx, r.pool)
	if _, err := db.Exec(ctx, `
		INSERT INTO devices (id, serial_number, device_type, status) VALUES ($1, $2, $3, 'NEW')
		ON CONFLICT (serial_number) DO NOTHING`, uuid.New(), serial, domain.DetectDeviceType(serial)); err != nil {
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
	if p.Type != nil {
		s.add("device_type", *p.Type)
	}
	if p.Status != nil {
		s.add("status", *p.Status)
	}
	if p.Hostname.Set {
		s.add("hostname", p.Hostname.Value)
	}
	if p.Responsible.Set {
		s.add("responsible", p.Responsible.Value)
	}
	if p.Location.Set {
		s.add("location", p.Location.Value)
	}
	if s.empty() {
		return nil
	}
	query, args := s.update("devices", id)
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, query, args...)
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
