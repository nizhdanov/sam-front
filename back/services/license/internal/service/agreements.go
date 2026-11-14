package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/pkg/opt"
	"sam/services/license/internal/domain"
)

const (
	maxAgreementText = 200_000
	similarLimit     = 3
)

type AnalysisView struct {
	domain.AgreementAnalysis
	Result     json.RawMessage           `json:"result"`
	Paragraphs []string                  `json:"paragraphs"`
	Similar    []domain.SimilarAgreement `json:"similar_agreements"`
}

type analysisResult struct {
	LicenseType   *domain.LicenseType `json:"license_type"`
	CostModel     *domain.CostModel   `json:"cost_model"`
	CommercialUse struct {
		Value string `json:"value"`
	} `json:"commercial_use"`
}

func (s *Service) ListAgreements(ctx context.Context, page httpx.PageRequest) ([]domain.Agreement, int, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, 0, err
	}
	items, total, err := s.agreements.List(ctx, page)
	for i := range items {
		items[i].AgreementText = nil
	}
	return items, total, err
}

func (s *Service) GetAgreement(ctx context.Context, id uuid.UUID) (domain.Agreement, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Agreement{}, err
	}
	return s.agreements.Get(ctx, id)
}

func validateAgreement(a domain.Agreement) error {
	switch {
	case a.Name == "":
		return apperr.Invalid("name is required")
	case !a.LicenseType.Valid():
		return apperr.Invalid("unknown license_type")
	case !a.CostModel.Valid():
		return apperr.Invalid("unknown cost_model")
	case a.AgreementText != nil && len(*a.AgreementText) > maxAgreementText:
		return apperr.Invalid("agreement_text is too long")
	}
	return nil
}

func (s *Service) CreateAgreement(ctx context.Context, a domain.Agreement) (domain.Agreement, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Agreement{}, err
	}
	a.ID = uuid.New()
	a.Name = strings.TrimSpace(a.Name)
	if a.LicenseType == "" {
		a.LicenseType = domain.LicenseCommercial
	}
	if a.CostModel == "" {
		a.CostModel = domain.CostPaid
	}
	if err := validateAgreement(a); err != nil {
		return domain.Agreement{}, err
	}
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.agreements.Create(ctx, a); err != nil {
			return err
		}
		return s.record(ctx, "agreement", a.ID.String(), domain.AuditCreate, "Добавлено соглашение "+a.Name,
			map[string]any{"license_type": a.LicenseType, "cost_model": a.CostModel})
	})
	if err != nil {
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
	case p.CostModel != nil && !p.CostModel.Valid():
		return domain.Agreement{}, apperr.Invalid("unknown cost_model")
	case p.AgreementText.Value != nil && len(*p.AgreementText.Value) > maxAgreementText:
		return domain.Agreement{}, apperr.Invalid("agreement_text is too long")
	}
	current, err := s.agreements.Get(ctx, id)
	if err != nil {
		return domain.Agreement{}, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.updateAgreement(ctx, id, p); err != nil {
			return err
		}
		return s.record(ctx, "agreement", id.String(), domain.AuditUpdate, "Изменено соглашение "+current.Name, agreementChanges(p))
	})
	if err != nil {
		return domain.Agreement{}, err
	}
	return s.agreements.Get(ctx, id)
}

func (s *Service) updateAgreement(ctx context.Context, id uuid.UUID, p domain.AgreementPatch) error {
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
}

func (s *Service) UploadAgreementFile(ctx context.Context, id uuid.UUID, fileName, text string) (domain.Agreement, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return domain.Agreement{}, apperr.Invalid("file contains no text")
	}
	return s.UpdateAgreement(ctx, id, domain.AgreementPatch{
		AgreementText: opt.Field[string]{Set: true, Value: &text},
		FileName:      opt.Field[string]{Set: true, Value: &fileName},
	})
}

func (s *Service) DeleteAgreement(ctx context.Context, id uuid.UUID) error {
	if _, err := requireAdmin(ctx); err != nil {
		return err
	}
	current, err := s.agreements.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.agreements.Delete(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, "agreement", id.String(), domain.AuditDelete, "Удалено соглашение "+current.Name, nil)
	})
}

func agreementChanges(p domain.AgreementPatch) map[string]any {
	out := map[string]any{}
	if p.Name != nil {
		out["name"] = *p.Name
	}
	if p.LicenseType != nil {
		out["license_type"] = *p.LicenseType
	}
	if p.CostModel != nil {
		out["cost_model"] = *p.CostModel
	}
	if p.AgreementText.Set {
		out["agreement_text"] = "изменён"
	}
	if p.FileName.Set {
		out["file_name"] = p.FileName.Value
	}
	if p.SourceURL.Set {
		out["source_url"] = p.SourceURL.Value
	}
	if p.CommercialAllowed.Set {
		out["commercial_allowed"] = p.CommercialAllowed.Value
	}
	if p.ReviewStatus != nil {
		out["review_status"] = *p.ReviewStatus
	}
	return out
}

func Paragraphs(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	sep := "\n\n"
	if !strings.Contains(text, sep) {
		sep = "\n"
	}
	var out []string
	for _, p := range strings.Split(text, sep) {
		if p = strings.Join(strings.Fields(p), " "); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func numbered(paragraphs []string) string {
	var b strings.Builder
	for i, p := range paragraphs {
		fmt.Fprintf(&b, "[%d] %s\n", i+1, p)
	}
	return b.String()
}

func (s *Service) AnalyzeAgreement(ctx context.Context, id uuid.UUID, force bool) (AnalysisView, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return AnalysisView{}, err
	}
	a, err := s.agreements.Get(ctx, id)
	if err != nil {
		return AnalysisView{}, err
	}
	if a.AgreementText == nil || strings.TrimSpace(*a.AgreementText) == "" {
		return AnalysisView{}, apperr.Invalid("agreement has no text to analyze")
	}
	sum := sha256.Sum256([]byte(*a.AgreementText))
	hash := hex.EncodeToString(sum[:])

	if !force {
		cached, err := s.agreements.LatestAnalysis(ctx, id, hash)
		if err == nil {
			return s.analysisView(ctx, a, cached)
		}
		if apperr.KindOf(err) != apperr.KindNotFound {
			return AnalysisView{}, err
		}
	}

	analysis, model, promptVersion, err := s.analyzer.Analyze(ctx, id, numbered(Paragraphs(*a.AgreementText)))
	if err != nil {
		return AnalysisView{}, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.agreements.SaveAnalysis(ctx, domain.AgreementAnalysis{
			ID: uuid.New(), AgreementID: id, ContentHash: hash, Model: model, PromptVersion: promptVersion, Analysis: analysis,
		}); err != nil {
			return err
		}
		return s.record(ctx, "agreement", id.String(), "ANALYZE", "AI-анализ соглашения "+a.Name, map[string]any{"model": model})
	})
	if err != nil {
		return AnalysisView{}, err
	}
	latest, err := s.agreements.LatestAnalysis(ctx, id, hash)
	if err != nil {
		return AnalysisView{}, err
	}
	return s.analysisView(ctx, a, latest)
}

func (s *Service) LatestAnalysis(ctx context.Context, id uuid.UUID) (AnalysisView, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return AnalysisView{}, err
	}
	a, err := s.agreements.Get(ctx, id)
	if err != nil {
		return AnalysisView{}, err
	}
	latest, err := s.agreements.LatestAnalysis(ctx, id, "")
	if err != nil {
		return AnalysisView{}, err
	}
	return s.analysisView(ctx, a, latest)
}

func (s *Service) analysisView(ctx context.Context, a domain.Agreement, analysis domain.AgreementAnalysis) (AnalysisView, error) {
	view := AnalysisView{AgreementAnalysis: analysis, Result: analysis.Analysis, Paragraphs: []string{}}
	if a.AgreementText != nil {
		view.Paragraphs = Paragraphs(*a.AgreementText)
	}
	similar, err := s.agreements.Similar(ctx, a.ID, similarLimit)
	if err != nil {
		return AnalysisView{}, err
	}
	view.Similar = similar
	return view, nil
}

func (s *Service) SimilarAgreements(ctx context.Context, id uuid.UUID) ([]domain.SimilarAgreement, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	if _, err := s.agreements.Get(ctx, id); err != nil {
		return nil, err
	}
	return s.agreements.Similar(ctx, id, similarLimit)
}

func (s *Service) ApproveAnalysis(ctx context.Context, id uuid.UUID) (AnalysisView, error) {
	c, err := requireAdmin(ctx)
	if err != nil {
		return AnalysisView{}, err
	}
	a, err := s.agreements.Get(ctx, id)
	if err != nil {
		return AnalysisView{}, err
	}
	latest, err := s.agreements.LatestAnalysis(ctx, id, "")
	if err != nil {
		return AnalysisView{}, err
	}
	var result analysisResult
	if err := json.Unmarshal(latest.Analysis, &result); err != nil {
		return AnalysisView{}, apperr.Invalid("analysis result is malformed")
	}
	status := "ANALYSIS_APPROVED"
	patch := domain.AgreementPatch{ReviewStatus: &status}
	if result.LicenseType != nil && result.LicenseType.Valid() {
		patch.LicenseType = result.LicenseType
	}
	if result.CostModel != nil && result.CostModel.Valid() {
		patch.CostModel = result.CostModel
	}
	switch result.CommercialUse.Value {
	case "YES", "NO":
		allowed := result.CommercialUse.Value == "YES"
		patch.CommercialAllowed = opt.Field[bool]{Set: true, Value: &allowed}
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.agreements.ApproveAnalysis(ctx, latest.ID, c.Actor()); err != nil {
			return err
		}
		if err := s.updateAgreement(ctx, id, patch); err != nil {
			return err
		}
		return s.record(ctx, "agreement", id.String(), domain.AuditApprove, "Анализ сохранён в соглашение "+a.Name, agreementChanges(patch))
	})
	if err != nil {
		return AnalysisView{}, err
	}
	return s.LatestAnalysis(ctx, id)
}
