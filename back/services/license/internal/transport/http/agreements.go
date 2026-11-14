package http

import (
	"encoding/json"
	"net/http"

	"sam/pkg/httpx"
	"sam/pkg/opt"
	"sam/services/license/internal/domain"
)

func (h *handler) listAgreements(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.ListAgreements(r.Context(), page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) getAgreement(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.GetAgreement(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

type agreementRequest struct {
	Name              string             `json:"name"`
	LicenseType       domain.LicenseType `json:"license_type"`
	AgreementText     *string            `json:"agreement_text"`
	SourceURL         *string            `json:"source_url"`
	CommercialAllowed *bool              `json:"commercial_allowed"`
}

func (h *handler) createAgreement(w http.ResponseWriter, r *http.Request) {
	var req agreementRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.CreateAgreement(r.Context(), domain.Agreement{
		Name: req.Name, LicenseType: req.LicenseType, AgreementText: req.AgreementText,
		SourceURL: req.SourceURL, CommercialAllowed: req.CommercialAllowed,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, a)
}

type agreementPatchRequest struct {
	Name              *string             `json:"name"`
	LicenseType       *domain.LicenseType `json:"license_type"`
	AgreementText     opt.Field[string]   `json:"agreement_text"`
	SourceURL         opt.Field[string]   `json:"source_url"`
	CommercialAllowed opt.Field[bool]     `json:"commercial_allowed"`
	ReviewStatus      *string             `json:"review_status"`
}

func (h *handler) updateAgreement(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req agreementPatchRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.UpdateAgreement(r.Context(), id, domain.AgreementPatch(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

type analysisResponse struct {
	domain.AgreementAnalysis
	Result json.RawMessage `json:"analysis"`
}

func toAnalysisResponse(a domain.AgreementAnalysis) analysisResponse {
	return analysisResponse{AgreementAnalysis: a, Result: a.Analysis}
}

func (h *handler) analyzeAgreement(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	force, err := httpx.QueryBool(r, "force")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.AnalyzeAgreement(r.Context(), id, force != nil && *force)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toAnalysisResponse(a))
}

func (h *handler) getAnalysis(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.LatestAnalysis(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toAnalysisResponse(a))
}

func (h *handler) approveAnalysis(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.ApproveAnalysis(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toAnalysisResponse(a))
}
