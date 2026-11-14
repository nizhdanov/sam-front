package http

import (
	"net/http"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/services/license/internal/domain"
	"sam/services/license/internal/service"
)

func (h *handler) listRules(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f := domain.RuleFilter{Search: r.URL.Query().Get("search")}
	if f.ProductID, err = queryID(r, "product_id"); err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.Rules(r.Context(), f, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

type ruleRequest struct {
	RawName      string     `json:"raw_name"`
	RawPublisher string     `json:"raw_publisher"`
	RawVersion   string     `json:"raw_version"`
	ProductID    uuid.UUID  `json:"product_id"`
	VersionID    *uuid.UUID `json:"version_id"`
}

func (h *handler) createRule(w http.ResponseWriter, r *http.Request) {
	var req ruleRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	rule, err := h.svc.CreateRule(r.Context(), service.RuleInput(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, rule)
}

func (h *handler) updateRule(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req approveRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	if req.ProductID == uuid.Nil {
		h.fail(w, r, apperr.Invalid("product_id is required"))
		return
	}
	rule, err := h.svc.UpdateRule(r.Context(), id, req.ProductID, req.VersionID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rule)
}

func (h *handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if err := h.svc.DeleteRule(r.Context(), id); err != nil {
		h.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) matchingSettings(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.MatchingSettings(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

func (h *handler) updateMatchingSettings(w http.ResponseWriter, r *http.Request) {
	var req service.MatchingSettings
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	s, err := h.svc.UpdateMatchingSettings(r.Context(), req)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

func (h *handler) auditLog(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	f := domain.AuditFilter{Actor: q.Get("actor"), Entity: q.Get("entity")}
	if raw := q.Get("from"); raw != "" {
		t, err := parseTimestamp(raw)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		f.From = &t
	}
	if raw := q.Get("to"); raw != "" {
		t, err := parseTimestamp(raw)
		if err != nil {
			h.fail(w, r, err)
			return
		}
		f.To = &t
	}
	items, total, err := h.svc.AuditLog(r.Context(), f, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) rematch(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.StartRematch(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, res)
}

func (h *handler) generateSuggestions(w http.ResponseWriter, r *http.Request) {
	res, err := h.svc.StartCatalogSuggestions(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, res)
}

func (h *handler) listSuggestions(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.CatalogSuggestions(r.Context(), domain.ReviewStatus(r.URL.Query().Get("status")), page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) approveSuggestions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs []uuid.UUID `json:"ids"`
		All bool        `json:"all"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	res, err := h.svc.StartApproveSuggestions(r.Context(), req.IDs, req.All)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, res)
}

func (h *handler) rejectSuggestion(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	s, err := h.svc.RejectSuggestion(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}
