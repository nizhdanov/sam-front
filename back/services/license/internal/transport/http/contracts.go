package http

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/pkg/opt"
	"sam/services/license/internal/domain"
)

func (h *handler) listContracts(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var f domain.ContractFilter
	if f.ProductID, err = queryID(r, "product_id"); err != nil {
		h.fail(w, r, err)
		return
	}
	active, err := httpx.QueryBool(r, "active")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f.ActiveOnly = active != nil && *active
	if f.ExpiringWithin, err = httpx.QueryInt(r, "expiring_within_days", 0, 0, 3650); err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.ListEntitlements(r.Context(), f, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) getContract(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.GetEntitlement(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

type contractRequest struct {
	ProductID      uuid.UUID     `json:"product_id"`
	AgreementID    *uuid.UUID    `json:"agreement_id"`
	Metric         domain.Metric `json:"license_metric"`
	Quantity       int           `json:"quantity"`
	ValidFrom      *timestamp    `json:"valid_from"`
	ExpiresAt      *timestamp    `json:"expires_at"`
	ContractNumber *string       `json:"purchase_reference"`
}

func (h *handler) createContract(w http.ResponseWriter, r *http.Request) {
	var req contractRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.Metric == "" {
		req.Metric = domain.MetricPerDevice
	}
	e, err := h.svc.CreateEntitlement(r.Context(), domain.Entitlement{
		ProductID: req.ProductID, AgreementID: req.AgreementID, Metric: req.Metric, Quantity: req.Quantity,
		ValidFrom: req.ValidFrom.ptr(), ExpiresAt: req.ExpiresAt.ptr(), PurchaseReference: req.ContractNumber,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

type contractPatchRequest struct {
	ProductID      *uuid.UUID           `json:"product_id"`
	AgreementID    opt.Field[uuid.UUID] `json:"agreement_id"`
	Metric         *domain.Metric       `json:"license_metric"`
	Quantity       *int                 `json:"quantity"`
	ValidFrom      opt.Field[timestamp] `json:"valid_from"`
	ExpiresAt      opt.Field[timestamp] `json:"expires_at"`
	ContractNumber opt.Field[string]    `json:"purchase_reference"`
}

func timeField(f opt.Field[timestamp]) opt.Field[time.Time] {
	return opt.Field[time.Time]{Set: f.Set, Value: f.Value.ptr()}
}

func (h *handler) updateContract(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req contractPatchRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.UpdateEntitlement(r.Context(), id, domain.EntitlementPatch{
		ProductID: req.ProductID, AgreementID: req.AgreementID, Metric: req.Metric, Quantity: req.Quantity,
		ValidFrom: timeField(req.ValidFrom), ExpiresAt: timeField(req.ExpiresAt), PurchaseReference: req.ContractNumber,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

func (h *handler) deactivateContract(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.DeactivateEntitlement(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

func (h *handler) balance(w http.ResponseWriter, r *http.Request) {
	switch view := r.URL.Query().Get("view"); view {
	case "", "commercial":
		productID, err := queryID(r, "product_id")
		if err != nil {
			h.fail(w, r, err)
			return
		}
		items, err := h.svc.CommercialBalances(r.Context(), productID)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"view": "commercial", "items": items})
	case "free":
		items, err := h.svc.FreeSoftware(r.Context())
		if err != nil {
			h.fail(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"view": "free", "items": items})
	default:
		h.fail(w, r, apperr.Invalid("view must be commercial or free"))
	}
}
