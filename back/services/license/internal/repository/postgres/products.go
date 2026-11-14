package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/pkg/swkey"
	"sam/services/license/internal/domain"
)

type Products struct{ pool *pgxpool.Pool }

func NewProducts(pool *pgxpool.Pool) *Products { return &Products{pool: pool} }

const productSelect = `
	SELECT p.id, p.canonical_name, p.publisher, p.edition, p.purpose, p.section, p.class, p.in_bpo, p.current_version,
	       p.agreement_id, a.name, a.license_type, a.cost_model, p.status, p.status_comment,
	       (SELECT COALESCE(SUM(e.quantity), 0) FROM license_entitlements e
	        WHERE e.product_id = p.id AND e.is_active
	          AND (e.valid_from IS NULL OR e.valid_from <= NOW()) AND (e.expires_at IS NULL OR e.expires_at > NOW())),
	       (SELECT COUNT(*) FROM installations i WHERE i.product_id = p.id AND i.is_active),
	       (SELECT COUNT(DISTINCT i.device_id) FROM installations i WHERE i.product_id = p.id AND i.is_active),
	       p.archived_at, p.created_at, p.updated_at
	FROM software_products p
	LEFT JOIN license_agreements a ON a.id = p.agreement_id`

func scanProduct(row scanner) (domain.Product, error) {
	var p domain.Product
	err := row.Scan(&p.ID, &p.CanonicalName, &p.Publisher, &p.Edition, &p.Purpose, &p.Section, &p.Class, &p.InBPO,
		&p.CurrentVersion, &p.AgreementID, &p.AgreementName, &p.LicenseType, &p.CostModel, &p.Status, &p.StatusComment,
		&p.Purchased, &p.Installations, &p.Devices, &p.ArchivedAt, &p.CreatedAt, &p.UpdatedAt)
	return p, err
}

func (r *Products) List(ctx context.Context, f domain.ProductFilter, page httpx.PageRequest) ([]domain.Product, int, error) {
	var w where
	if !f.IncludeArchived {
		w.raw("p.archived_at IS NULL")
	}
	if f.Search != "" {
		w.add("(p.canonical_name ILIKE ? OR p.publisher ILIKE ?)", "%"+f.Search+"%")
	}
	if f.Status != "" {
		w.add("p.status = ?", f.Status)
	}
	if f.Section != "" {
		w.add("p.section = ?", f.Section)
	}
	if f.Class != "" {
		w.add("p.class = ?", f.Class)
	}
	if f.LicenseType != "" {
		w.add("a.license_type = ?", f.LicenseType)
	}
	if f.CostModel != "" {
		w.add("a.cost_model = ?", f.CostModel)
	}
	if f.InBPO != nil {
		w.add("p.in_bpo = ?", *f.InBPO)
	}
	if f.PublicOnly {
		w.raw("(p.in_bpo OR a.cost_model = 'FREE' OR p.status = 'FORBIDDEN')")
	}
	db := pg.DB(ctx, r.pool)
	var total int
	count := `SELECT COUNT(*) FROM software_products p LEFT JOIN license_agreements a ON a.id = p.agreement_id`
	if err := db.QueryRow(ctx, count+w.String(), w.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count products: %w", err)
	}
	query := productSelect + w.String() + ` ORDER BY p.canonical_name, p.edition NULLS FIRST LIMIT ` + w.arg(page.Limit) + ` OFFSET ` + w.arg(page.Offset())
	rows, err := db.Query(ctx, query, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list products: %w", err)
	}
	items, err := collect(rows, scanProduct)
	return items, total, err
}

func (r *Products) Get(ctx context.Context, id uuid.UUID) (domain.Product, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, productSelect+` WHERE p.id = $1`, id)
	return one(rows, err, scanProduct, "software product")
}

func (r *Products) GetMany(ctx context.Context, ids []uuid.UUID) ([]domain.Product, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, productSelect+` WHERE p.id = ANY($1)`, uuids(ids))
	if err != nil {
		return nil, fmt.Errorf("get products: %w", err)
	}
	return collect(rows, scanProduct)
}

func (r *Products) Create(ctx context.Context, p domain.Product) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO software_products (id, canonical_name, normalized_name, publisher, normalized_publisher, edition,
		                               purpose, section, class, in_bpo, current_version, agreement_id, status, status_comment)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		p.ID, p.CanonicalName, swkey.Normalize(p.CanonicalName), p.Publisher, swkey.NormalizePublisher(value(p.Publisher)),
		p.Edition, p.Purpose, p.Section, p.Class, p.InBPO, p.CurrentVersion, p.AgreementID, p.Status, p.StatusComment)
	if pg.IsForeignKeyViolation(err) {
		return apperr.Invalid("agreement does not exist")
	}
	if pg.IsUniqueViolation(err) {
		return apperr.Conflict("product with the same name, publisher and edition already exists")
	}
	if err != nil {
		return fmt.Errorf("create product: %w", err)
	}
	return nil
}

func (r *Products) Update(ctx context.Context, id uuid.UUID, p domain.ProductPatch) error {
	var s set
	if p.CanonicalName != nil {
		name := strings.TrimSpace(*p.CanonicalName)
		s.add("canonical_name", name)
		s.add("normalized_name", swkey.Normalize(name))
	}
	if p.Publisher != nil {
		s.add("publisher", nonEmpty(*p.Publisher))
		s.add("normalized_publisher", swkey.NormalizePublisher(*p.Publisher))
	}
	if p.Edition != nil {
		s.add("edition", nonEmpty(*p.Edition))
	}
	if p.Purpose != nil {
		s.add("purpose", nonEmpty(*p.Purpose))
	}
	if p.Section != nil {
		s.add("section", nonEmpty(*p.Section))
	}
	if p.Class != nil {
		s.add("class", nonEmpty(*p.Class))
	}
	if p.InBPO != nil {
		s.add("in_bpo", *p.InBPO)
	}
	if p.CurrentVersion.Set {
		s.add("current_version", p.CurrentVersion.Value)
	}
	if p.AgreementID.Set {
		s.add("agreement_id", p.AgreementID.Value)
	}
	if p.Status != nil {
		s.add("status", *p.Status)
	}
	if p.StatusComment != nil {
		s.add("status_comment", nonEmpty(*p.StatusComment))
	}
	if p.Archived != nil {
		if *p.Archived {
			s.raw("archived_at = COALESCE(archived_at, NOW())")
		} else {
			s.raw("archived_at = NULL")
		}
	}
	if s.empty() {
		return nil
	}
	query, args := s.update("software_products", id)
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, query, args...)
	if pg.IsForeignKeyViolation(err) {
		return apperr.Invalid("agreement does not exist")
	}
	if pg.IsUniqueViolation(err) {
		return apperr.Conflict("product with the same name, publisher and edition already exists")
	}
	if err != nil {
		return fmt.Errorf("update product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("software product not found")
	}
	return nil
}

func (r *Products) Usage(ctx context.Context, id uuid.UUID) (installations, contracts int, err error) {
	err = pg.DB(ctx, r.pool).QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM installations WHERE product_id = $1),
		       (SELECT COUNT(*) FROM license_entitlements WHERE product_id = $1)`, id).Scan(&installations, &contracts)
	if err != nil {
		return 0, 0, fmt.Errorf("product usage: %w", err)
	}
	return installations, contracts, nil
}

func (r *Products) Delete(ctx context.Context, id uuid.UUID) error {
	db := pg.DB(ctx, r.pool)
	for _, q := range []string{
		`DELETE FROM software_aliases WHERE product_id = $1`,
		`DELETE FROM software_versions WHERE product_id = $1`,
		`UPDATE matching_candidates SET suggested_product = NULL WHERE suggested_product = $1`,
		`DELETE FROM compliance_findings WHERE product_id = $1`,
	} {
		if _, err := db.Exec(ctx, q, id); err != nil {
			return fmt.Errorf("delete product references: %w", err)
		}
	}
	tag, err := db.Exec(ctx, `DELETE FROM software_products WHERE id = $1`, id)
	if pg.IsForeignKeyViolation(err) {
		return apperr.Conflict("product is still referenced")
	}
	if err != nil {
		return fmt.Errorf("delete product: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("software product not found")
	}
	return nil
}

func (r *Products) Candidates(ctx context.Context, normalizedName string, limit int) ([]domain.Candidate, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		SELECT id, canonical_name, COALESCE(publisher, ''), COALESCE(edition, ''),
		       GREATEST(similarity(normalized_name, $1), word_similarity(normalized_name, $1)) AS score
		FROM software_products
		WHERE archived_at IS NULL AND (normalized_name = $1 OR normalized_name % $1 OR normalized_name <% $1)
		ORDER BY (normalized_name = $1) DESC, score DESC, canonical_name
		LIMIT $2`, normalizedName, limit)
	if err != nil {
		return nil, fmt.Errorf("find candidates: %w", err)
	}
	return collect(rows, func(row scanner) (domain.Candidate, error) {
		var c domain.Candidate
		err := row.Scan(&c.ProductID, &c.Name, &c.Publisher, &c.Edition, &c.Similarity)
		return c, err
	})
}

func (r *Products) ProductIDsByAgreement(ctx context.Context, agreementID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `SELECT id FROM software_products WHERE agreement_id = $1`, agreementID)
	if err != nil {
		return nil, fmt.Errorf("products by agreement: %w", err)
	}
	return collect(rows, scanID)
}

func scanID(row scanner) (uuid.UUID, error) {
	var id uuid.UUID
	err := row.Scan(&id)
	return id, err
}

func value(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func nonEmpty(s string) *string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return &s
}
