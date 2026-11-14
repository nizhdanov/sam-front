package http

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"sam/pkg/httpx"
	"sam/pkg/opt"
	"sam/services/license/internal/domain"
)

func (h *handler) listLicenses(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	productID, err := queryID(r, "product_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	active, err := httpx.QueryBool(r, "active")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.ListEntitlements(r.Context(), productID, active != nil && *active, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) getLicense(w http.ResponseWriter, r *http.Request) {
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

type licenseRequest struct {
	ProductID         uuid.UUID     `json:"product_id"`
	AgreementID       *uuid.UUID    `json:"agreement_id"`
	Metric            domain.Metric `json:"license_metric"`
	Quantity          int           `json:"quantity"`
	UnitPrice         *float64      `json:"unit_price"`
	Currency          *string       `json:"currency"`
	ValidFrom         *timestamp    `json:"valid_from"`
	ExpiresAt         *timestamp    `json:"expires_at"`
	PurchaseReference *string       `json:"purchase_reference"`
}

func (h *handler) createLicense(w http.ResponseWriter, r *http.Request) {
	var req licenseRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.CreateEntitlement(r.Context(), domain.Entitlement{
		ProductID: req.ProductID, AgreementID: req.AgreementID, Metric: req.Metric, Quantity: req.Quantity,
		UnitPrice: req.UnitPrice, Currency: req.Currency, ValidFrom: req.ValidFrom.ptr(), ExpiresAt: req.ExpiresAt.ptr(),
		PurchaseReference: req.PurchaseReference,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, e)
}

type licensePatchRequest struct {
	ProductID         *uuid.UUID           `json:"product_id"`
	AgreementID       opt.Field[uuid.UUID] `json:"agreement_id"`
	Metric            *domain.Metric       `json:"license_metric"`
	Quantity          *int                 `json:"quantity"`
	UnitPrice         opt.Field[float64]   `json:"unit_price"`
	Currency          opt.Field[string]    `json:"currency"`
	ValidFrom         opt.Field[timestamp] `json:"valid_from"`
	ExpiresAt         opt.Field[timestamp] `json:"expires_at"`
	PurchaseReference opt.Field[string]    `json:"purchase_reference"`
}

func timeField(f opt.Field[timestamp]) opt.Field[time.Time] {
	return opt.Field[time.Time]{Set: f.Set, Value: f.Value.ptr()}
}

func (h *handler) updateLicense(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req licensePatchRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.UpdateEntitlement(r.Context(), id, domain.EntitlementPatch{
		ProductID: req.ProductID, AgreementID: req.AgreementID, Metric: req.Metric, Quantity: req.Quantity,
		UnitPrice: req.UnitPrice, Currency: req.Currency, ValidFrom: timeField(req.ValidFrom), ExpiresAt: timeField(req.ExpiresAt),
		PurchaseReference: req.PurchaseReference,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

func (h *handler) deactivateLicense(w http.ResponseWriter, r *http.Request) {
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

func (h *handler) balances(w http.ResponseWriter, r *http.Request) {
	productID, err := queryID(r, "product_id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.svc.Balances(r.Context(), productID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}
