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
	SELECT p.id, p.canonical_name, p.publisher, p.edition, p.agreement_id, a.name, a.license_type, p.policy_status,
	       p.policy_comment, p.description,
	       (SELECT COUNT(*) FROM installations i WHERE i.product_id = p.id AND i.is_active),
	       p.archived_at, p.created_at, p.updated_at
	FROM software_products p
	LEFT JOIN license_agreements a ON a.id = p.agreement_id`

func scanProduct(row scanner) (domain.Product, error) {
	var p domain.Product
	err := row.Scan(&p.ID, &p.CanonicalName, &p.Publisher, &p.Edition, &p.AgreementID, &p.AgreementName, &p.LicenseType,
		&p.PolicyStatus, &p.PolicyComment, &p.Description, &p.Installations, &p.ArchivedAt, &p.CreatedAt, &p.UpdatedAt)
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
	if f.PolicyStatus != "" {
		w.add("p.policy_status = ?", f.PolicyStatus)
	}
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM software_products p`+w.String(), w.args...).Scan(&total); err != nil {
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
		                               agreement_id, policy_status, policy_comment, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		p.ID, p.CanonicalName, swkey.Normalize(p.CanonicalName), p.Publisher, swkey.NormalizePublisher(value(p.Publisher)),
		p.Edition, p.AgreementID, p.PolicyStatus, p.PolicyComment, p.Description)
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
	if p.AgreementID.Set {
		s.add("agreement_id", p.AgreementID.Value)
	}
	if p.PolicyStatus != nil {
		s.add("policy_status", *p.PolicyStatus)
	}
	if p.PolicyComment != nil {
		s.add("policy_comment", nonEmpty(*p.PolicyComment))
	}
	if p.Description != nil {
		s.add("description", nonEmpty(*p.Description))
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
	if err != nil {
		return fmt.Errorf("update product: %w", err)
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
