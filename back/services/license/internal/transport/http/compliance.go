package http

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/services/license/internal/domain"
)

func (h *handler) listFindings(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	f := domain.FindingFilter{Severity: domain.Severity(q.Get("severity")), Status: domain.FindingStatus(q.Get("status"))}
	if raw := q.Get("type"); raw != "" {
		for t := range strings.SplitSeq(raw, ",") {
			ft := domain.FindingType(strings.TrimSpace(t))
			if !ft.Valid() {
				h.fail(w, r, apperr.Invalid("unknown finding type "+string(ft)))
				return
			}
			f.Types = append(f.Types, ft)
		}
	}
	if f.DeviceID, err = queryID(r, "device_id"); err != nil {
		h.fail(w, r, err)
		return
	}
	if f.ProductID, err = queryID(r, "product_id"); err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.ListFindings(r.Context(), f, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) getFinding(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f, err := h.svc.GetFinding(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, f)
}

func (h *handler) acknowledgeFinding(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	f, err := h.svc.AcknowledgeFinding(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, f)
}

func (h *handler) complianceSummary(w http.ResponseWriter, r *http.Request) {
	s, err := h.svc.ComplianceSummary(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, s)
}

func (h *handler) recalculate(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.TriggerRecalculation(r.Context()); err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "recalculated"})
}

func (h *handler) dashboard(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.Dashboard(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *handler) softwareAnalytics(w http.ResponseWriter, r *http.Request) {
	limit, err := httpx.QueryInt(r, "limit", 20, 1, httpx.MaxLimit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, err := h.svc.SoftwareAnalytics(r.Context(), limit)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) departmentAnalytics(w http.ResponseWriter, r *http.Request) {
	items, err := h.svc.DepartmentAnalytics(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *handler) pendingMatches(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.MatchingTasks(r.Context(), domain.ReviewStatus(r.URL.Query().Get("status")), page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) getMatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	t, err := h.svc.MatchingTask(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

type approveRequest struct {
	ProductID uuid.UUID  `json:"product_id"`
	VersionID *uuid.UUID `json:"version_id"`
}

func (h *handler) approveMatch(w http.ResponseWriter, r *http.Request) {
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
	t, err := h.svc.ApproveMatching(r.Context(), id, req.ProductID, req.VersionID)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}

func (h *handler) rejectMatch(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	if r.ContentLength != 0 {
		if err := httpx.Decode(w, r, &req); err != nil {
			h.fail(w, r, err)
			return
		}
	}
	t, err := h.svc.RejectMatching(r.Context(), id, req.Reason)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, t)
}
