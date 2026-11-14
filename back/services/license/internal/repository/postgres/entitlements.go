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

type Entitlements struct{ pool *pgxpool.Pool }

func NewEntitlements(pool *pgxpool.Pool) *Entitlements { return &Entitlements{pool: pool} }

const entitlementSelect = `
	SELECT e.id, e.product_id, p.canonical_name, e.agreement_id, e.license_metric, e.quantity, e.unit_price::float8,
	       e.currency, e.valid_from, e.expires_at, e.purchase_reference, e.is_active, e.created_at, e.updated_at
	FROM license_entitlements e
	JOIN software_products p ON p.id = e.product_id`

func scanEntitlement(row scanner) (domain.Entitlement, error) {
	var e domain.Entitlement
	err := row.Scan(&e.ID, &e.ProductID, &e.ProductName, &e.AgreementID, &e.Metric, &e.Quantity, &e.UnitPrice,
		&e.Currency, &e.ValidFrom, &e.ExpiresAt, &e.PurchaseReference, &e.IsActive, &e.CreatedAt, &e.UpdatedAt)
	return e, err
}

func (r *Entitlements) List(ctx context.Context, productID *uuid.UUID, activeOnly bool, page httpx.PageRequest) ([]domain.Entitlement, int, error) {
	var w where
	if productID != nil {
		w.add("e.product_id = ?", *productID)
	}
	if activeOnly {
		w.raw("e.is_active")
	}
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM license_entitlements e`+w.String(), w.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count licenses: %w", err)
	}
	query := entitlementSelect + w.String() + ` ORDER BY p.canonical_name, e.created_at LIMIT ` + w.arg(page.Limit) + ` OFFSET ` + w.arg(page.Offset())
	rows, err := db.Query(ctx, query, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list licenses: %w", err)
	}
	items, err := collect(rows, scanEntitlement)
	return items, total, err
}

func (r *Entitlements) Get(ctx context.Context, id uuid.UUID) (domain.Entitlement, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, entitlementSelect+` WHERE e.id = $1`, id)
	return one(rows, err, scanEntitlement, "license")
}

func (r *Entitlements) Active(ctx context.Context, s domain.Scope) (map[uuid.UUID][]domain.Entitlement, error) {
	query := entitlementSelect + ` WHERE e.is_active`
	var args []any
	if !s.All {
		query += ` AND e.product_id = ANY($1)`
		args = append(args, uuids(s.ProductIDs))
	}
	rows, err := pg.DB(ctx, r.pool).Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("active licenses: %w", err)
	}
	list, err := collect(rows, scanEntitlement)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID][]domain.Entitlement)
	for _, e := range list {
		out[e.ProductID] = append(out[e.ProductID], e)
	}
	return out, nil
}

func (r *Entitlements) Create(ctx context.Context, e domain.Entitlement) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO license_entitlements (id, product_id, agreement_id, license_metric, quantity, unit_price, currency,
		                                  valid_from, expires_at, purchase_reference)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		e.ID, e.ProductID, e.AgreementID, e.Metric, e.Quantity, e.UnitPrice, e.Currency, e.ValidFrom, e.ExpiresAt, e.PurchaseReference)
	if pg.IsForeignKeyViolation(err) {
		return apperr.Invalid("product or agreement does not exist")
	}
	if err != nil {
		return fmt.Errorf("create license: %w", err)
	}
	return nil
}

func (r *Entitlements) Update(ctx context.Context, id uuid.UUID, p domain.EntitlementPatch) error {
	var s set
	if p.ProductID != nil {
		s.add("product_id", *p.ProductID)
	}
	if p.AgreementID.Set {
		s.add("agreement_id", p.AgreementID.Value)
	}
	if p.Metric != nil {
		s.add("license_metric", *p.Metric)
	}
	if p.Quantity != nil {
		s.add("quantity", *p.Quantity)
	}
	if p.UnitPrice.Set {
		s.add("unit_price", p.UnitPrice.Value)
	}
	if p.Currency.Set {
		s.add("currency", p.Currency.Value)
	}
	if p.ValidFrom.Set {
		s.add("valid_from", p.ValidFrom.Value)
	}
	if p.ExpiresAt.Set {
		s.add("expires_at", p.ExpiresAt.Value)
	}
	if p.PurchaseReference.Set {
		s.add("purchase_reference", p.PurchaseReference.Value)
	}
	if s.empty() {
		return nil
	}
	query, args := s.update("license_entitlements", id)
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, query, args...)
	if pg.IsForeignKeyViolation(err) {
		return apperr.Invalid("product or agreement does not exist")
	}
	if err != nil {
		return fmt.Errorf("update license: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("license not found")
	}
	return nil
}

func (r *Entitlements) Deactivate(ctx context.Context, id uuid.UUID) error {
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, `UPDATE license_entitlements SET is_active = FALSE, updated_at = NOW() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deactivate license: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("license not found")
	}
	return nil
}
