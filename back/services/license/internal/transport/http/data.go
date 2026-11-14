package http

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/pkg/xlsx"
	"sam/services/license/internal/domain"
)

const maxExportRows = 10_000

var reservedParams = map[string]bool{"group_by": true, "limit": true, "format": true}

func (h *handler) queryData(w http.ResponseWriter, r *http.Request) {
	format := r.URL.Query().Get("format")
	export := format != "" && format != "json"
	maxRows := domain.MaxQueryRows
	if export {
		maxRows = domain.MaxExportRows
	}
	limit, err := httpx.QueryInt(r, "limit", maxRows, 1, maxRows)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	q := domain.DataQuery{
		Dataset: domain.Dataset(chi.URLParam(r, "dataset")),
		GroupBy: r.URL.Query().Get("group_by"),
		Limit:   limit,
		Export:  export,
		Filters: map[string]string{},
	}
	for key, values := range r.URL.Query() {
		if reservedParams[key] || len(values) == 0 {
			continue
		}
		q.Filters[key] = values[0]
	}
	table, err := h.svc.QueryData(r.Context(), q)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	if !export {
		httpx.JSON(w, http.StatusOK, table)
		return
	}
	h.writeTable(w, r, format, string(table.Dataset), table.Columns, table.Rows)
}

type exportRequest struct {
	Title   string   `json:"title"`
	Format  string   `json:"format"`
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
}

func (h *handler) exportTable(w http.ResponseWriter, r *http.Request) {
	var req exportRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	switch {
	case len(req.Columns) == 0:
		h.fail(w, r, apperr.Invalid("columns are required"))
		return
	case len(req.Rows) > maxExportRows:
		h.fail(w, r, apperr.TooLarge(fmt.Sprintf("at most %d rows can be exported", maxExportRows)))
		return
	}
	if req.Title == "" {
		req.Title = "export"
	}
	h.writeTable(w, r, req.Format, req.Title, req.Columns, req.Rows)
}

func (h *handler) writeTable(w http.ResponseWriter, r *http.Request, format, title string, columns []string, rows [][]any) {
	cells := make([][]string, len(rows))
	for i, row := range rows {
		cells[i] = make([]string, len(row))
		for j, v := range row {
			cells[i][j] = cellText(v)
		}
	}
	name := fmt.Sprintf("%s-%s", title, time.Now().Format("20060102-150405"))
	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, name))
		_, _ = w.Write([]byte{0xEF, 0xBB, 0xBF})
		cw := csv.NewWriter(w)
		cw.Comma = ';'
		_ = cw.Write(columns)
		_ = cw.WriteAll(cells)
	case "xlsx":
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, name))
		if err := xlsx.Write(w, sheetName(title), columns, cells); err != nil {
			h.log.ErrorContext(r.Context(), "write xlsx", "err", err)
		}
	default:
		h.fail(w, r, apperr.Invalid("format must be json, csv or xlsx"))
	}
}

func cellText(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if t {
			return "да"
		}
		return "нет"
	}
	return fmt.Sprint(v)
}

func sheetName(title string) string {
	title = strings.NewReplacer("/", " ", "\\", " ", "?", "", "*", "", "[", "", "]", "", ":", "").Replace(title)
	if r := []rune(title); len(r) > 31 {
		title = string(r[:31])
	}
	return title
}
