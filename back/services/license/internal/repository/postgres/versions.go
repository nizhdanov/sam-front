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

type Versions struct{ pool *pgxpool.Pool }

func NewVersions(pool *pgxpool.Pool) *Versions { return &Versions{pool: pool} }

const versionSelect = `SELECT id, product_id, version, lifecycle_status, release_date, end_of_support, created_at FROM software_versions`

func scanVersion(row scanner) (domain.Version, error) {
	var v domain.Version
	err := row.Scan(&v.ID, &v.ProductID, &v.Version, &v.LifecycleStatus, &v.ReleaseDate, &v.EndOfSupport, &v.CreatedAt)
	return v, err
}

func (r *Versions) List(ctx context.Context, productID uuid.UUID) ([]domain.Version, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, versionSelect+` WHERE product_id = $1 ORDER BY created_at`, productID)
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	return collect(rows, scanVersion)
}

func (r *Versions) ByProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]domain.Version, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, versionSelect+` WHERE product_id = ANY($1)`, uuids(productIDs))
	if err != nil {
		return nil, fmt.Errorf("list versions: %w", err)
	}
	list, err := collect(rows, scanVersion)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]domain.Version)
	for _, v := range list {
		out[v.ProductID] = append(out[v.ProductID], v)
	}
	return out, nil
}

func (r *Versions) Get(ctx context.Context, id uuid.UUID) (domain.Version, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, versionSelect+` WHERE id = $1`, id)
	return one(rows, err, scanVersion, "version")
}

func (r *Versions) Create(ctx context.Context, v domain.Version) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO software_versions (id, product_id, version, lifecycle_status, release_date, end_of_support)
		VALUES ($1, $2, $3, $4, $5, $6)`, v.ID, v.ProductID, v.Version, v.LifecycleStatus, v.ReleaseDate, v.EndOfSupport)
	switch {
	case pg.IsUniqueViolation(err):
		return apperr.Conflict("version already exists")
	case pg.IsForeignKeyViolation(err):
		return apperr.NotFound("software product not found")
	case err != nil:
		return fmt.Errorf("create version: %w", err)
	}
	return nil
}

func (r *Versions) SetLifecycle(ctx context.Context, productID, id uuid.UUID, lifecycle domain.Lifecycle) error {
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, `UPDATE software_versions SET lifecycle_status = $1 WHERE id = $2 AND product_id = $3`,
		lifecycle, id, productID)
	if err != nil {
		return fmt.Errorf("update version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("version not found")
	}
	return nil
}
