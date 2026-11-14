package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/services/license/internal/domain"
)

const maxAgreementText = 200_000

func (s *Service) ListAgreements(ctx context.Context, page httpx.PageRequest) ([]domain.Agreement, int, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return nil, 0, err
	}
	items, total, err := s.agreements.List(ctx, page)
	for i := range items {
		items[i].AgreementText = nil
	}
	return items, total, err
}

func (s *Service) GetAgreement(ctx context.Context, id uuid.UUID) (domain.Agreement, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return domain.Agreement{}, err
	}
	return s.agreements.Get(ctx, id)
}

func (s *Service) CreateAgreement(ctx context.Context, a domain.Agreement) (domain.Agreement, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Agreement{}, err
	}
	a.ID = uuid.New()
	a.Name = strings.TrimSpace(a.Name)
	if a.LicenseType == "" {
		a.LicenseType = domain.LicenseUnknown
	}
	switch {
	case a.Name == "":
		return domain.Agreement{}, apperr.Invalid("name is required")
	case !a.LicenseType.Valid():
		return domain.Agreement{}, apperr.Invalid("unknown license_type")
	case a.AgreementText != nil && len(*a.AgreementText) > maxAgreementText:
		return domain.Agreement{}, apperr.Invalid("agreement_text is too long")
	}
	if err := s.agreements.Create(ctx, a); err != nil {
		return domain.Agreement{}, err
	}
	return s.agreements.Get(ctx, a.ID)
}

func (s *Service) UpdateAgreement(ctx context.Context, id uuid.UUID, p domain.AgreementPatch) (domain.Agreement, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Agreement{}, err
	}
	switch {
	case p.Name != nil && strings.TrimSpace(*p.Name) == "":
		return domain.Agreement{}, apperr.Invalid("name must not be empty")
	case p.LicenseType != nil && !p.LicenseType.Valid():
		return domain.Agreement{}, apperr.Invalid("unknown license_type")
	case p.AgreementText.Value != nil && len(*p.AgreementText.Value) > maxAgreementText:
		return domain.Agreement{}, apperr.Invalid("agreement_text is too long")
	}
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		products, err := s.products.ProductIDsByAgreement(ctx, id)
		if err != nil {
			return err
		}
		if err := s.locks.Products(ctx, products); err != nil {
			return err
		}
		if err := s.agreements.Update(ctx, id, p); err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: products})
	})
	if err != nil {
		return domain.Agreement{}, err
	}
	return s.agreements.Get(ctx, id)
}

func (s *Service) AnalyzeAgreement(ctx context.Context, id uuid.UUID, force bool) (domain.AgreementAnalysis, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.AgreementAnalysis{}, err
	}
	a, err := s.agreements.Get(ctx, id)
	if err != nil {
		return domain.AgreementAnalysis{}, err
	}
	if a.AgreementText == nil || strings.TrimSpace(*a.AgreementText) == "" {
		return domain.AgreementAnalysis{}, apperr.Invalid("agreement has no text to analyze")
	}
	sum := sha256.Sum256([]byte(*a.AgreementText))
	hash := hex.EncodeToString(sum[:])

	if !force {
		cached, err := s.agreements.LatestAnalysis(ctx, id, hash)
		if err == nil {
			return cached, nil
		}
		if apperr.KindOf(err) != apperr.KindNotFound {
			return domain.AgreementAnalysis{}, err
		}
	}

	analysis, model, promptVersion, err := s.analyzer.Analyze(ctx, id, *a.AgreementText)
	if err != nil {
		return domain.AgreementAnalysis{}, err
	}
	if err := s.agreements.SaveAnalysis(ctx, domain.AgreementAnalysis{
		ID: uuid.New(), AgreementID: id, ContentHash: hash, Model: model, PromptVersion: promptVersion, Analysis: analysis,
	}); err != nil {
		return domain.AgreementAnalysis{}, err
	}
	return s.agreements.LatestAnalysis(ctx, id, hash)
}

func (s *Service) LatestAnalysis(ctx context.Context, id uuid.UUID) (domain.AgreementAnalysis, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return domain.AgreementAnalysis{}, err
	}
	return s.agreements.LatestAnalysis(ctx, id, "")
}

func (s *Service) ApproveAnalysis(ctx context.Context, id uuid.UUID) (domain.AgreementAnalysis, error) {
	c, err := requireAdmin(ctx)
	if err != nil {
		return domain.AgreementAnalysis{}, err
	}
	latest, err := s.agreements.LatestAnalysis(ctx, id, "")
	if err != nil {
		return domain.AgreementAnalysis{}, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.agreements.ApproveAnalysis(ctx, latest.ID, c.Actor()); err != nil {
			return err
		}
		status := "ANALYSIS_APPROVED"
		return s.agreements.Update(ctx, id, domain.AgreementPatch{ReviewStatus: &status})
	})
	if err != nil {
		return domain.AgreementAnalysis{}, err
	}
	return s.agreements.LatestAnalysis(ctx, id, "")
}
