package http

import (
	"net/http"

	"github.com/google/uuid"

	"sam/pkg/apperr"
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
	f := domain.ProductFilter{Search: q.Get("search"), PolicyStatus: domain.PolicyStatus(q.Get("policy_status")), IncludeArchived: archived != nil && *archived}
	items, total, err := h.svc.ListProducts(r.Context(), f, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if auth.ClaimsFrom(r.Context()).IsPrivileged() {
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
	if auth.ClaimsFrom(r.Context()).IsPrivileged() {
		httpx.JSON(w, http.StatusOK, p)
		return
	}
	httpx.JSON(w, http.StatusOK, p.Public())
}

type productRequest struct {
	CanonicalName string     `json:"canonical_name"`
	Publisher     *string    `json:"publisher"`
	Edition       *string    `json:"edition"`
	AgreementID   *uuid.UUID `json:"agreement_id"`
	PolicyStatus  string     `json:"policy_status"`
	Description   *string    `json:"description"`
}

func (h *handler) createProduct(w http.ResponseWriter, r *http.Request) {
	var req productRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	p, err := h.svc.CreateProduct(r.Context(), service.ProductInput{
		CanonicalName: req.CanonicalName, Publisher: req.Publisher, Edition: req.Edition,
		AgreementID: req.AgreementID, PolicyStatus: domain.PolicyStatus(req.PolicyStatus), Description: req.Description,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, p)
}

type productPatchRequest struct {
	CanonicalName *string              `json:"canonical_name"`
	Publisher     *string              `json:"publisher"`
	Edition       *string              `json:"edition"`
	AgreementID   opt.Field[uuid.UUID] `json:"agreement_id"`
	PolicyStatus  *domain.PolicyStatus `json:"policy_status"`
	Description   *string              `json:"description"`
	Archived      *bool                `json:"archived"`
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

func (h *handler) updatePolicy(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req struct {
		PolicyStatus domain.PolicyStatus `json:"policy_status"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.PolicyStatus == "" {
		h.fail(w, r, apperr.Invalid("policy_status is required"))
		return
	}
	p, err := h.svc.UpdateProduct(r.Context(), id, domain.ProductPatch{PolicyStatus: &req.PolicyStatus})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, p)
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

func (h *handler) listAliases(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.svc.Aliases(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

type aliasRequest struct {
	RawName      string     `json:"raw_name"`
	RawPublisher string     `json:"raw_publisher"`
	RawVersion   string     `json:"raw_version"`
	VersionID    *uuid.UUID `json:"version_id"`
}

func (h *handler) createAlias(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req aliasRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	a, err := h.svc.CreateAlias(r.Context(), id, service.AliasInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, a)
}

func (h *handler) deleteAlias(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	aliasID, err := pathID(r, "aliasID")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DeleteAlias(r.Context(), id, aliasID); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) productInstallations(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.ProductInstallations(r.Context(), id, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) productBalance(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.svc.Balances(r.Context(), &id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}
