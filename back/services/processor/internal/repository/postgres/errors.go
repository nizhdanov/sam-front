package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/services/processor/internal/domain"
)

type ImportErrors struct{ pool *pgxpool.Pool }

func NewImportErrors(pool *pgxpool.Pool) *ImportErrors { return &ImportErrors{pool: pool} }

func (r *ImportErrors) Insert(ctx context.Context, importID uuid.UUID, errs []domain.ImportError) error {
	if len(errs) == 0 {
		return nil
	}
	rows := make([][]any, 0, len(errs))
	for _, e := range errs {
		rows = append(rows, []any{uuid.New(), importID, e.SnapshotID, e.RowNumber, e.Code, e.Message})
	}
	_, err := pg.DB(ctx, r.pool).CopyFrom(ctx, pgx.Identifier{"import_errors"},
		[]string{"id", "import_id", "snapshot_id", "row_number", "error_code", "error_message"}, pgx.CopyFromRows(rows))
	if err != nil {
		return fmt.Errorf("insert import errors: %w", err)
	}
	return nil
}

func (r *ImportErrors) List(ctx context.Context, importID uuid.UUID, page httpx.PageRequest) ([]domain.ImportError, int, error) {
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM import_errors WHERE import_id = $1`, importID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count import errors: %w", err)
	}
	rows, err := db.Query(ctx, `
		SELECT id, snapshot_id, row_number, error_code, error_message, created_at
		FROM import_errors WHERE import_id = $1 ORDER BY row_number NULLS LAST, created_at LIMIT $2 OFFSET $3`,
		importID, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list import errors: %w", err)
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.ImportError, error) {
		var e domain.ImportError
		err := row.Scan(&e.ID, &e.SnapshotID, &e.RowNumber, &e.Code, &e.Message, &e.CreatedAt)
		return e, err
	})
	return items, total, err
}
