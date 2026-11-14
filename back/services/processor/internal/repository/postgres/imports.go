package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/services/processor/internal/domain"
)

type Imports struct{ pool *pgxpool.Pool }

func NewImports(pool *pgxpool.Pool) *Imports { return &Imports{pool: pool} }

const importSelect = `
	SELECT id, filename, COALESCE(storage_path, ''), source, status, file_size, total_rows, total_devices, created_by,
	       collected_at, error_message, created_at, updated_at, completed_at
	FROM imports`

type scanner interface{ Scan(dest ...any) error }

func scanImport(row scanner) (domain.Import, error) {
	var i domain.Import
	err := row.Scan(&i.ID, &i.Filename, &i.StoragePath, &i.Source, &i.Status, &i.FileSize, &i.TotalRows, &i.TotalDevices,
		&i.CreatedBy, &i.CollectedAt, &i.ErrorMessage, &i.CreatedAt, &i.UpdatedAt, &i.CompletedAt)
	return i, err
}

func (r *Imports) Create(ctx context.Context, i domain.Import) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO imports (id, filename, storage_path, source, status, file_size, created_by, collected_at)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8)`,
		i.ID, i.Filename, i.StoragePath, i.Source, i.Status, i.FileSize, i.CreatedBy, i.CollectedAt)
	if err != nil {
		return fmt.Errorf("create import: %w", err)
	}
	return nil
}

func (r *Imports) Get(ctx context.Context, id uuid.UUID) (domain.Import, error) {
	i, err := scanImport(pg.DB(ctx, r.pool).QueryRow(ctx, importSelect+` WHERE id = $1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return i, apperr.NotFound("import not found")
	}
	if err != nil {
		return i, fmt.Errorf("get import: %w", err)
	}
	return i, nil
}

func (r *Imports) List(ctx context.Context, page httpx.PageRequest) ([]domain.Import, int, error) {
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM imports`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count imports: %w", err)
	}
	rows, err := db.Query(ctx, importSelect+` ORDER BY created_at DESC LIMIT $1 OFFSET $2`, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list imports: %w", err)
	}
	defer rows.Close()
	var out []domain.Import
	for rows.Next() {
		i, err := scanImport(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, i)
	}
	return out, total, rows.Err()
}

func (r *Imports) ByStatus(ctx context.Context, statuses ...domain.ImportStatus) ([]domain.Import, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, importSelect+` WHERE status = ANY($1) ORDER BY created_at`, statuses)
	if err != nil {
		return nil, fmt.Errorf("imports by status: %w", err)
	}
	defer rows.Close()
	var out []domain.Import
	for rows.Next() {
		i, err := scanImport(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (r *Imports) SetStatus(ctx context.Context, id uuid.UUID, status domain.ImportStatus, message *string) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		UPDATE imports SET status = $2, error_message = $3, updated_at = NOW(),
		       completed_at = CASE WHEN $2 IN ('COMPLETED', 'COMPLETED_WITH_ERRORS', 'FAILED') THEN NOW() ELSE NULL END
		WHERE id = $1`, id, status, message)
	if err != nil {
		return fmt.Errorf("set import status: %w", err)
	}
	return nil
}

func (r *Imports) SetParsed(ctx context.Context, id uuid.UUID, rows, devices int) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		UPDATE imports SET total_rows = $2, total_devices = $3, status = 'PROCESSING', error_message = NULL, updated_at = NOW()
		WHERE id = $1`, id, rows, devices)
	if err != nil {
		return fmt.Errorf("set import parsed: %w", err)
	}
	return nil
}

func (r *Imports) Expired(ctx context.Context, before time.Time) ([]uuid.UUID, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		SELECT id FROM imports
		WHERE source = 'CSV' AND storage_path IS NOT NULL AND completed_at < $1
		  AND status IN ('COMPLETED', 'COMPLETED_WITH_ERRORS', 'FAILED')`, before)
	if err != nil {
		return nil, fmt.Errorf("expired imports: %w", err)
	}
	return pgx.CollectRows(rows, pgx.RowTo[uuid.UUID])
}

func (r *Imports) ClearStorage(ctx context.Context, id uuid.UUID) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `UPDATE imports SET storage_path = NULL, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("clear import storage: %w", err)
	}
	return nil
}
