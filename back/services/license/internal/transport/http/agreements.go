package http

import (
	"errors"
	"io"
	"net/http"

	"sam/pkg/apperr"
	"sam/pkg/doctext"
	"sam/pkg/httpx"
	"sam/pkg/opt"
	"sam/services/license/internal/domain"
)

const maxAgreementFile = 10 << 20

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
	CostModel         domain.CostModel   `json:"cost_model"`
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
		Name: req.Name, LicenseType: req.LicenseType, CostModel: req.CostModel, AgreementText: req.AgreementText,
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
	CostModel         *domain.CostModel   `json:"cost_model"`
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
	a, err := h.svc.UpdateAgreement(r.Context(), id, domain.AgreementPatch{
		Name: req.Name, LicenseType: req.LicenseType, CostModel: req.CostModel, AgreementText: req.AgreementText,
		SourceURL: req.SourceURL, CommercialAllowed: req.CommercialAllowed, ReviewStatus: req.ReviewStatus,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
}

func (h *handler) deleteAgreement(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DeleteAgreement(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) uploadAgreementFile(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxAgreementFile+1<<20)
	file, header, err := r.FormFile("file")
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			h.fail(w, r, apperr.TooLarge("file exceeds 10 MB"))
			return
		}
		h.fail(w, r, apperr.Invalid("multipart field \"file\" is required"))
		return
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, maxAgreementFile+1))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if len(data) > maxAgreementFile {
		h.fail(w, r, apperr.TooLarge("file exceeds 10 MB"))
		return
	}
	text, err := doctext.Extract(header.Filename, data)
	if err != nil {
		h.fail(w, r, apperr.Invalid(err.Error()))
		return
	}
	a, err := h.svc.UploadAgreementFile(r.Context(), id, header.Filename, text)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, a)
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
	httpx.JSON(w, http.StatusOK, a)
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
	httpx.JSON(w, http.StatusOK, a)
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
	httpx.JSON(w, http.StatusOK, a)
}

func (h *handler) similarAgreements(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.svc.SimilarAgreements(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}
