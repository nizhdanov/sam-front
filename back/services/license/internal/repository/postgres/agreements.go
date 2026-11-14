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

type Agreements struct{ pool *pgxpool.Pool }

func NewAgreements(pool *pgxpool.Pool) *Agreements { return &Agreements{pool: pool} }

const agreementSelect = `
	SELECT a.id, a.name, a.license_type, a.cost_model, a.agreement_text, a.file_name, a.source_url, a.commercial_allowed,
	       a.review_status, (SELECT COUNT(*) FROM software_products p WHERE p.agreement_id = a.id AND p.archived_at IS NULL),
	       a.created_at, a.updated_at
	FROM license_agreements a`

func scanAgreement(row scanner) (domain.Agreement, error) {
	var a domain.Agreement
	err := row.Scan(&a.ID, &a.Name, &a.LicenseType, &a.CostModel, &a.AgreementText, &a.FileName, &a.SourceURL,
		&a.CommercialAllowed, &a.ReviewStatus, &a.Products, &a.CreatedAt, &a.UpdatedAt)
	return a, err
}

func (r *Agreements) List(ctx context.Context, page httpx.PageRequest) ([]domain.Agreement, int, error) {
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM license_agreements`).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count agreements: %w", err)
	}
	rows, err := db.Query(ctx, agreementSelect+` ORDER BY a.name LIMIT $1 OFFSET $2`, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list agreements: %w", err)
	}
	items, err := collect(rows, scanAgreement)
	return items, total, err
}

func (r *Agreements) Get(ctx context.Context, id uuid.UUID) (domain.Agreement, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, agreementSelect+` WHERE a.id = $1`, id)
	return one(rows, err, scanAgreement, "agreement")
}

func (r *Agreements) Create(ctx context.Context, a domain.Agreement) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO license_agreements (id, name, license_type, cost_model, agreement_text, file_name, source_url, commercial_allowed)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		a.ID, a.Name, a.LicenseType, a.CostModel, a.AgreementText, a.FileName, a.SourceURL, a.CommercialAllowed)
	if err != nil {
		return fmt.Errorf("create agreement: %w", err)
	}
	return nil
}

func (r *Agreements) Update(ctx context.Context, id uuid.UUID, p domain.AgreementPatch) error {
	var s set
	if p.Name != nil {
		s.add("name", *p.Name)
	}
	if p.LicenseType != nil {
		s.add("license_type", *p.LicenseType)
	}
	if p.CostModel != nil {
		s.add("cost_model", *p.CostModel)
	}
	if p.AgreementText.Set {
		s.add("agreement_text", p.AgreementText.Value)
	}
	if p.FileName.Set {
		s.add("file_name", p.FileName.Value)
	}
	if p.SourceURL.Set {
		s.add("source_url", p.SourceURL.Value)
	}
	if p.CommercialAllowed.Set {
		s.add("commercial_allowed", p.CommercialAllowed.Value)
	}
	if p.ReviewStatus != nil {
		s.add("review_status", *p.ReviewStatus)
	}
	if s.empty() {
		return nil
	}
	query, args := s.update("license_agreements", id)
	tag, err := pg.DB(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update agreement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("agreement not found")
	}
	return nil
}

func (r *Agreements) Delete(ctx context.Context, id uuid.UUID) error {
	db := pg.DB(ctx, r.pool)
	var products int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM software_products WHERE agreement_id = $1`, id).Scan(&products); err != nil {
		return fmt.Errorf("count agreement products: %w", err)
	}
	if products > 0 {
		return apperr.Conflict(fmt.Sprintf("agreement is used by %d product(s)", products))
	}
	if _, err := db.Exec(ctx, `UPDATE license_entitlements SET agreement_id = NULL WHERE agreement_id = $1`, id); err != nil {
		return fmt.Errorf("detach agreement contracts: %w", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM agreement_analyses WHERE agreement_id = $1`, id); err != nil {
		return fmt.Errorf("delete agreement analyses: %w", err)
	}
	tag, err := db.Exec(ctx, `DELETE FROM license_agreements WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete agreement: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return apperr.NotFound("agreement not found")
	}
	return nil
}

func (r *Agreements) Similar(ctx context.Context, id uuid.UUID, limit int) ([]domain.SimilarAgreement, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		WITH src AS (SELECT LEFT(agreement_text, 20000) AS body FROM license_agreements WHERE id = $1)
		SELECT a.id, a.name, a.license_type, a.cost_model, similarity(LEFT(a.agreement_text, 20000), src.body)::float8 AS score
		FROM license_agreements a, src
		WHERE a.id <> $1 AND a.agreement_text IS NOT NULL AND src.body IS NOT NULL
		ORDER BY score DESC, a.name
		LIMIT $2`, id, limit)
	if err != nil {
		return nil, fmt.Errorf("similar agreements: %w", err)
	}
	return collect(rows, func(row scanner) (domain.SimilarAgreement, error) {
		var a domain.SimilarAgreement
		err := row.Scan(&a.ID, &a.Name, &a.LicenseType, &a.CostModel, &a.Similarity)
		return a, err
	})
}

const analysisSelect = `
	SELECT id, agreement_id, content_hash, model, prompt_version, analysis_json, status, reviewed_by, reviewed_at, created_at
	FROM agreement_analyses`

func scanAnalysis(row scanner) (domain.AgreementAnalysis, error) {
	var a domain.AgreementAnalysis
	err := row.Scan(&a.ID, &a.AgreementID, &a.ContentHash, &a.Model, &a.PromptVersion, &a.Analysis, &a.Status,
		&a.ReviewedBy, &a.ReviewedAt, &a.CreatedAt)
	return a, err
}

func (r *Agreements) LatestAnalysis(ctx context.Context, agreementID uuid.UUID, contentHash string) (domain.AgreementAnalysis, error) {
	var w where
	w.add("agreement_id = ?", agreementID)
	if contentHash != "" {
		w.add("content_hash = ?", contentHash)
	}
	rows, err := pg.DB(ctx, r.pool).Query(ctx, analysisSelect+w.String()+` ORDER BY created_at DESC LIMIT 1`, w.args...)
	return one(rows, err, scanAnalysis, "agreement analysis")
}

func (r *Agreements) SaveAnalysis(ctx context.Context, a domain.AgreementAnalysis) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO agreement_analyses (id, agreement_id, content_hash, model, prompt_version, analysis_json)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (agreement_id, content_hash, prompt_version) DO UPDATE SET
			analysis_json = EXCLUDED.analysis_json, model = EXCLUDED.model, status = 'DRAFT',
			reviewed_by = NULL, reviewed_at = NULL, created_at = NOW()`,
		a.ID, a.AgreementID, a.ContentHash, a.Model, a.PromptVersion, a.Analysis)
	if err != nil {
		return fmt.Errorf("save agreement analysis: %w", err)
	}
	return nil
}

func (r *Agreements) ApproveAnalysis(ctx context.Context, id uuid.UUID, reviewer string) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		UPDATE agreement_analyses SET status = 'APPROVED', reviewed_by = $2, reviewed_at = NOW() WHERE id = $1`, id, reviewer)
	if err != nil {
		return fmt.Errorf("approve agreement analysis: %w", err)
	}
	return nil
}
