package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Aliases struct{ pool *pgxpool.Pool }

func NewAliases(pool *pgxpool.Pool) *Aliases { return &Aliases{pool: pool} }

const aliasSelect = `
	SELECT s.id, s.source_key, s.raw_name, s.raw_publisher, s.raw_version, s.product_id, p.canonical_name, s.version_id,
	       s.match_method, s.reviewed_by, s.created_at
	FROM software_aliases s
	JOIN software_products p ON p.id = s.product_id`

func scanAlias(row scanner) (domain.Alias, error) {
	var a domain.Alias
	err := row.Scan(&a.ID, &a.SourceKey, &a.RawName, &a.RawPublisher, &a.RawVersion, &a.ProductID, &a.ProductName,
		&a.VersionID, &a.MatchMethod, &a.ReviewedBy, &a.CreatedAt)
	return a, err
}

func (r *Aliases) List(ctx context.Context, f domain.RuleFilter, page httpx.PageRequest) ([]domain.Alias, int, error) {
	var w where
	if f.Search != "" {
		w.add("(s.raw_name ILIKE ? OR p.canonical_name ILIKE ?)", "%"+f.Search+"%")
	}
	if f.ProductID != nil {
		w.add("s.product_id = ?", *f.ProductID)
	}
	db := pg.DB(ctx, r.pool)
	var total int
	count := `SELECT COUNT(*) FROM software_aliases s JOIN software_products p ON p.id = s.product_id`
	if err := db.QueryRow(ctx, count+w.String(), w.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count rules: %w", err)
	}
	query := aliasSelect + w.String() + ` ORDER BY s.raw_name, s.raw_version LIMIT ` + w.arg(page.Limit) + ` OFFSET ` + w.arg(page.Offset())
	rows, err := db.Query(ctx, query, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list rules: %w", err)
	}
	items, err := collect(rows, scanAlias)
	return items, total, err
}

func (r *Aliases) Get(ctx context.Context, id uuid.UUID) (domain.Alias, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, aliasSelect+` WHERE s.id = $1`, id)
	return one(rows, err, scanAlias, "rule")
}

func (r *Aliases) BySourceKeys(ctx context.Context, keys []string) (map[string]domain.Alias, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, aliasSelect+` WHERE s.source_key = ANY($1)`, keys)
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
	rows, err := pg.DB(ctx, r.pool).Query(ctx, aliasSelect+` WHERE s.source_key = $1`, key)
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

func (r *Aliases) Delete(ctx context.Context, id uuid.UUID) (string, error) {
	var key string
	err := pg.DB(ctx, r.pool).QueryRow(ctx, `DELETE FROM software_aliases WHERE id = $1 RETURNING source_key`, id).Scan(&key)
	if err != nil {
		return "", notFound(err, "alias")
	}
	return key, nil
}
