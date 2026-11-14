package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pg "sam/pkg/postgres"
	"sam/services/processor/internal/domain"
)

type Snapshots struct{ pool *pgxpool.Pool }

func NewSnapshots(pool *pgxpool.Pool) *Snapshots { return &Snapshots{pool: pool} }

func (r *Snapshots) Insert(ctx context.Context, snapshots []domain.Snapshot) error {
	batch := &pgx.Batch{}
	for _, s := range snapshots {
		software, err := json.Marshal(s.Software)
		if err != nil {
			return err
		}
		batch.Queue(`
			INSERT INTO inventory_snapshots (id, import_id, serial_number, collected_at, snapshot_type, software, status)
			VALUES ($1, $2, $3, $4, $5, $6, 'PENDING')`,
			s.ID, s.ImportID, s.SerialNumber, s.CollectedAt, s.Type, software)
	}
	if err := pg.DB(ctx, r.pool).SendBatch(ctx, batch).Close(); err != nil {
		return fmt.Errorf("insert snapshots: %w", err)
	}
	return nil
}

func (r *Snapshots) Unpublished(ctx context.Context, importID uuid.UUID) ([]domain.Snapshot, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		SELECT id, import_id, serial_number, collected_at, snapshot_type, status
		FROM inventory_snapshots
		WHERE import_id = $1 AND (status = 'PENDING' OR (status = 'QUEUED' AND published_at IS NULL)
		      OR (status = 'PROCESSING' AND lease_until < NOW()))
		ORDER BY serial_number`, importID)
	if err != nil {
		return nil, fmt.Errorf("unpublished snapshots: %w", err)
	}
	return pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Snapshot, error) {
		var s domain.Snapshot
		err := row.Scan(&s.ID, &s.ImportID, &s.SerialNumber, &s.CollectedAt, &s.Type, &s.Status)
		return s, err
	})
}

func (r *Snapshots) ResetFailed(ctx context.Context, importID uuid.UUID) (int64, error) {
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, `
		UPDATE inventory_snapshots SET status = 'PENDING', attempts = 0, error_message = NULL, attempt_token = NULL, lease_until = NULL
		WHERE import_id = $1 AND status = 'FAILED'`, importID)
	if err != nil {
		return 0, fmt.Errorf("reset failed snapshots: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (r *Snapshots) MarkPublished(ctx context.Context, id uuid.UUID) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		UPDATE inventory_snapshots
		SET published_at = NOW(), status = CASE WHEN status = 'PENDING' THEN 'QUEUED' ELSE status END
		WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("mark snapshot published: %w", err)
	}
	return nil
}

func (r *Snapshots) Progress(ctx context.Context, importID uuid.UUID) (domain.Progress, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `SELECT status, COUNT(*) FROM inventory_snapshots WHERE import_id = $1 GROUP BY status`, importID)
	if err != nil {
		return domain.Progress{}, fmt.Errorf("import progress: %w", err)
	}
	defer rows.Close()
	var p domain.Progress
	for rows.Next() {
		var status domain.SnapshotStatus
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return p, err
		}
		p.Total += n
		switch status {
		case domain.SnapshotPending:
			p.Pending = n
		case domain.SnapshotQueued:
			p.Queued = n
		case domain.SnapshotProcessing:
			p.Processing = n
		case domain.SnapshotCompleted:
			p.Completed = n
		case domain.SnapshotFailed:
			p.Failed = n
		case domain.SnapshotSuperseded:
			p.Superseded = n
		}
	}
	return p, rows.Err()
}
