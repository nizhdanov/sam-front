package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Aliases struct{ pool *pgxpool.Pool }

func NewAliases(pool *pgxpool.Pool) *Aliases { return &Aliases{pool: pool} }

const aliasSelect = `
	SELECT id, source_key, raw_name, raw_publisher, raw_version, product_id, version_id, match_method, reviewed_by, created_at
	FROM software_aliases`

func scanAlias(row scanner) (domain.Alias, error) {
	var a domain.Alias
	err := row.Scan(&a.ID, &a.SourceKey, &a.RawName, &a.RawPublisher, &a.RawVersion, &a.ProductID, &a.VersionID,
		&a.MatchMethod, &a.ReviewedBy, &a.CreatedAt)
	return a, err
}

func (r *Aliases) ByProduct(ctx context.Context, productID uuid.UUID) ([]domain.Alias, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, aliasSelect+` WHERE product_id = $1 ORDER BY raw_name, raw_version`, productID)
	if err != nil {
		return nil, fmt.Errorf("list aliases: %w", err)
	}
	return collect(rows, scanAlias)
}

func (r *Aliases) BySourceKeys(ctx context.Context, keys []string) (map[string]domain.Alias, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, aliasSelect+` WHERE source_key = ANY($1)`, keys)
	if err != nil {
		return nil, fmt.Errorf("find aliases: %w", err)
	}
	list, err := collect(rows, scanAlias)
	if err != nil {
		return nil, err
	}
	out := make(map[string]domain.Alias, len(list))
	for _, a := range list {
		out[a.SourceKey] = a
	}
	return out, nil
}

func (r *Aliases) BySourceKey(ctx context.Context, key string) (domain.Alias, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, aliasSelect+` WHERE source_key = $1`, key)
	return one(rows, err, scanAlias, "alias")
}

func (r *Aliases) Upsert(ctx context.Context, a domain.Assignment) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO software_aliases (id, source_key, raw_name, raw_publisher, raw_version, product_id, version_id, match_method, reviewed_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (source_key) DO UPDATE SET
			product_id = EXCLUDED.product_id, version_id = EXCLUDED.version_id,
			match_method = EXCLUDED.match_method, reviewed_by = EXCLUDED.reviewed_by, updated_at = NOW()`,
		uuid.New(), a.SourceKey, a.RawName, a.RawPublisher, a.RawVersion, a.ProductID, a.VersionID, a.Method, a.ReviewedBy)
	if pg.IsForeignKeyViolation(err) {
		return apperr.NotFound("software product not found")
	}
	if err != nil {
		return fmt.Errorf("upsert alias: %w", err)
	}
	return nil
}

func (r *Aliases) Delete(ctx context.Context, productID, id uuid.UUID) (string, error) {
	var key string
	err := pg.DB(ctx, r.pool).QueryRow(ctx, `DELETE FROM software_aliases WHERE id = $1 AND product_id = $2 RETURNING source_key`,
		id, productID).Scan(&key)
	if err != nil {
		return "", notFound(err, "alias")
	}
	return key, nil
}
