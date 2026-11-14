package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Suggestions struct{ pool *pgxpool.Pool }

func NewSuggestions(pool *pgxpool.Pool) *Suggestions { return &Suggestions{pool: pool} }

const suggestionSelect = `
	SELECT id, group_key, canonical_name, publisher, is_software, items, model, status, product_id, reviewed_by,
	       reviewed_at, created_at
	FROM catalog_suggestions`

func scanSuggestion(row scanner) (domain.CatalogSuggestion, error) {
	var s domain.CatalogSuggestion
	var items []byte
	if err := row.Scan(&s.ID, &s.GroupKey, &s.CanonicalName, &s.Publisher, &s.IsSoftware, &items, &s.Model, &s.Status,
		&s.ProductID, &s.ReviewedBy, &s.ReviewedAt, &s.CreatedAt); err != nil {
		return s, err
	}
	return s, json.Unmarshal(items, &s.Items)
}

func (r *Suggestions) Merge(ctx context.Context, s domain.CatalogSuggestion) error {
	items, err := json.Marshal(s.Items)
	if err != nil {
		return err
	}
	_, err = pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO catalog_suggestions (id, group_key, canonical_name, publisher, is_software, items, model)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (group_key) WHERE status = 'PENDING' DO UPDATE SET
			items = (SELECT jsonb_agg(DISTINCT e) FROM jsonb_array_elements(catalog_suggestions.items || EXCLUDED.items) e),
			is_software = catalog_suggestions.is_software AND EXCLUDED.is_software,
			publisher = COALESCE(catalog_suggestions.publisher, EXCLUDED.publisher),
			model = EXCLUDED.model,
			updated_at = NOW()`,
		uuid.New(), s.GroupKey, s.CanonicalName, s.Publisher, s.IsSoftware, items, s.Model)
	if err != nil {
		return fmt.Errorf("merge catalog suggestion: %w", err)
	}
	return nil
}

func (r *Suggestions) List(ctx context.Context, status domain.ReviewStatus, page httpx.PageRequest) ([]domain.CatalogSuggestion, int, error) {
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM catalog_suggestions WHERE status = $1`, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count catalog suggestions: %w", err)
	}
	rows, err := db.Query(ctx, suggestionSelect+`
		WHERE status = $1 ORDER BY jsonb_array_length(items) DESC, canonical_name LIMIT $2 OFFSET $3`,
		status, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list catalog suggestions: %w", err)
	}
	items, err := collect(rows, scanSuggestion)
	return items, total, err
}

func (r *Suggestions) PendingIDs(ctx context.Context) ([]uuid.UUID, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `SELECT id FROM catalog_suggestions WHERE status = 'PENDING' ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("pending catalog suggestions: %w", err)
	}
	return collect(rows, scanID)
}

func (r *Suggestions) Get(ctx context.Context, id uuid.UUID) (domain.CatalogSuggestion, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, suggestionSelect+` WHERE id = $1`, id)
	return one(rows, err, scanSuggestion, "catalog suggestion")
}

func (r *Suggestions) Resolve(ctx context.Context, id uuid.UUID, status domain.ReviewStatus, productID *uuid.UUID, reviewer string) error {
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, `
		UPDATE catalog_suggestions SET status = $2, product_id = $3, reviewed_by = $4, reviewed_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status = 'PENDING'`, id, status, productID, reviewer)
	if err != nil {
		return fmt.Errorf("resolve catalog suggestion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.Conflict("catalog suggestion is already reviewed")
	}
	return nil
}
