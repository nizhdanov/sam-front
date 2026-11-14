package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	"sam/services/worker/internal/domain"
)

type Snapshots struct{ pool *pgxpool.Pool }

func NewSnapshots(pool *pgxpool.Pool) *Snapshots { return &Snapshots{pool: pool} }

func (r *Snapshots) Get(ctx context.Context, id uuid.UUID) (domain.Snapshot, error) {
	var s domain.Snapshot
	var kind string
	var software []byte
	err := r.pool.QueryRow(ctx, `
		SELECT id, import_id, serial_number, collected_at, snapshot_type, software, status, attempts
		FROM inventory_snapshots WHERE id = $1`, id).
		Scan(&s.ID, &s.ImportID, &s.SerialNumber, &s.CollectedAt, &kind, &software, &s.Status, &s.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, apperr.NotFound("snapshot not found")
	}
	if err != nil {
		return s, fmt.Errorf("get snapshot: %w", err)
	}
	s.Full = kind == "FULL"
	if err := json.Unmarshal(software, &s.Software); err != nil {
		return s, apperr.Wrap(err, apperr.KindInvalid, "snapshot software is malformed")
	}
	return s, nil
}

func (r *Snapshots) Claim(ctx context.Context, id uuid.UUID, lease time.Duration) (domain.Lease, error) {
	l := domain.Lease{SnapshotID: id, Token: uuid.New()}
	err := r.pool.QueryRow(ctx, `
		UPDATE inventory_snapshots
		SET status = 'PROCESSING', attempts = attempts + 1, attempt_token = $2,
		    lease_until = NOW() + make_interval(secs => $3), processing_started_at = NOW(), error_message = NULL
		WHERE id = $1
		  AND (status IN ('PENDING', 'QUEUED') OR (status = 'PROCESSING' AND lease_until < NOW()))
		RETURNING attempts`, id, l.Token, lease.Seconds()).Scan(&l.Attempt)
	if err == nil {
		return l, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return l, fmt.Errorf("claim snapshot: %w", err)
	}
	var status domain.Status
	if err := r.pool.QueryRow(ctx, `SELECT status FROM inventory_snapshots WHERE id = $1`, id).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return l, apperr.NotFound("snapshot not found")
		}
		return l, fmt.Errorf("read snapshot status: %w", err)
	}
	if status.Final() {
		return l, domain.ErrAlreadyDone
	}
	return l, domain.ErrLeaseHeld
}

func (r *Snapshots) Extend(ctx context.Context, l domain.Lease, lease time.Duration) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE inventory_snapshots SET lease_until = NOW() + make_interval(secs => $3)
		WHERE id = $1 AND attempt_token = $2 AND status = 'PROCESSING'`, l.SnapshotID, l.Token, lease.Seconds())
	if err != nil {
		return fmt.Errorf("extend lease: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLeaseLost
	}
	return nil
}

func (r *Snapshots) Finish(ctx context.Context, l domain.Lease, status domain.Status, message string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE inventory_snapshots
		SET status = $3, error_message = NULLIF($4, ''), lease_until = NULL,
		    processed_at = CASE WHEN $3 IN ('COMPLETED', 'SUPERSEDED', 'FAILED') THEN NOW() ELSE processed_at END
		WHERE id = $1 AND attempt_token = $2 AND status = 'PROCESSING'`, l.SnapshotID, l.Token, status, message)
	if err != nil {
		return fmt.Errorf("finish snapshot: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrLeaseLost
	}
	return nil
}

func (r *Snapshots) FailUnfinished(ctx context.Context, id uuid.UUID, message string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE inventory_snapshots SET status = 'FAILED', error_message = $2, lease_until = NULL, processed_at = NOW()
		WHERE id = $1 AND status IN ('PENDING', 'QUEUED', 'PROCESSING')`, id, message)
	if err != nil {
		return fmt.Errorf("fail snapshot: %w", err)
	}
	return nil
}
