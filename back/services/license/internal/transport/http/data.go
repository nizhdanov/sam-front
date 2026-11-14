package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"sam/pkg/httpx"
	"sam/services/license/internal/domain"
)

func (h *handler) queryData(w http.ResponseWriter, r *http.Request) {
	limit, err := httpx.QueryInt(r, "limit", domain.MaxQueryRows, 1, domain.MaxQueryRows)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	q := domain.DataQuery{
		Dataset: domain.Dataset(chi.URLParam(r, "dataset")),
		GroupBy: r.URL.Query().Get("group_by"),
		Limit:   limit,
		Filters: map[string]string{},
	}
	for key, values := range r.URL.Query() {
		if key == "group_by" || key == "limit" || len(values) == 0 {
			continue
		}
		q.Filters[key] = values[0]
	}
	table, err := h.svc.QueryData(r.Context(), q)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, table)
}
