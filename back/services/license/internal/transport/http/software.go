package http

import (
	"net/http"

	"github.com/google/uuid"

	"sam/pkg/auth"
	"sam/pkg/httpx"
	"sam/pkg/opt"
	"sam/services/license/internal/domain"
	"sam/services/license/internal/service"
)

func (h *handler) listProducts(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	archived, err := httpx.QueryBool(r, "include_archived")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	inBPO, err := httpx.QueryBool(r, "in_bpo")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f := domain.ProductFilter{
		Search:          q.Get("search"),
		Status:          domain.ProductStatus(q.Get("status")),
		Section:         q.Get("section"),
		Class:           q.Get("class"),
		LicenseType:     domain.LicenseType(q.Get("license_type")),
		CostModel:       domain.CostModel(q.Get("cost_model")),
		InBPO:           inBPO,
		IncludeArchived: archived != nil && *archived,
	}
	items, total, err := h.svc.ListProducts(r.Context(), f, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if auth.ClaimsFrom(r.Context()).IsAdmin() {
		respondPage(w, items, page, total)
		return
	}
	public := make([]domain.PublicProduct, 0, len(items))
	for _, p := range items {
		public = append(public, p.Public())
	}
	respondPage(w, public, page, total)
}

func (h *handler) getProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.GetProduct(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if auth.ClaimsFrom(r.Context()).IsAdmin() {
		httpx.JSON(w, http.StatusOK, p)
		return
	}
	httpx.JSON(w, http.StatusOK, p.Public())
}

type productRequest struct {
	CanonicalName  string               `json:"canonical_name"`
	Publisher      *string              `json:"publisher"`
	Edition        *string              `json:"edition"`
	Purpose        *string              `json:"purpose"`
	Section        *string              `json:"section"`
	Class          *string              `json:"class"`
	InBPO          bool                 `json:"in_bpo"`
	CurrentVersion *string              `json:"current_version"`
	AgreementID    *uuid.UUID           `json:"agreement_id"`
	Status         domain.ProductStatus `json:"status"`
	StatusComment  *string              `json:"status_comment"`
}

func (h *handler) createProduct(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.CreateProduct(r.Context(), service.ProductInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

type productPatchRequest struct {
	CanonicalName  *string               `json:"canonical_name"`
	Publisher      *string               `json:"publisher"`
	Edition        *string               `json:"edition"`
	Purpose        *string               `json:"purpose"`
	Section        *string               `json:"section"`
	Class          *string               `json:"class"`
	InBPO          *bool                 `json:"in_bpo"`
	CurrentVersion opt.Field[string]     `json:"current_version"`
	AgreementID    opt.Field[uuid.UUID]  `json:"agreement_id"`
	Status         *domain.ProductStatus `json:"status"`
	StatusComment  *string               `json:"status_comment"`
	Archived       *bool                 `json:"archived"`
}

func (h *handler) updateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req productPatchRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.UpdateProduct(r.Context(), id, domain.ProductPatch(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
}

func (h *handler) deleteProduct(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DeleteProduct(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) listVersions(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.svc.Versions(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type versionRequest struct {
	Version         string           `json:"version"`
	LifecycleStatus domain.Lifecycle `json:"lifecycle_status"`
	ReleaseDate     *timestamp       `json:"release_date"`
	EndOfSupport    *timestamp       `json:"end_of_support"`
}

func (h *handler) createVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req versionRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.CreateVersion(r.Context(), id, service.VersionInput{
		Version: req.Version, LifecycleStatus: req.LifecycleStatus, ReleaseDate: req.ReleaseDate.ptr(), EndOfSupport: req.EndOfSupport.ptr(),
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, v)
}

func (h *handler) updateVersion(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	versionID, err := pathID(r, "versionID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req struct {
		LifecycleStatus domain.Lifecycle `json:"lifecycle_status"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	v, err := h.svc.SetVersionLifecycle(r.Context(), id, versionID, req.LifecycleStatus)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, v)
}
