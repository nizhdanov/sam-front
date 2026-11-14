package http

import (
	"net/http"

	"github.com/google/uuid"

	"sam/pkg/httpx"
	"sam/pkg/opt"
	"sam/services/license/internal/domain"
)

func (h *handler) listDevices(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	q := r.URL.Query()
	f := domain.DeviceFilter{Search: q.Get("search"), SerialNumber: q.Get("serial_number"), Department: q.Get("department")}
	if f.EmployeeID, err = queryID(r, "employee_id"); err != nil {
		h.fail(w, r, err)
		return
	}
	if f.HasViolations, err = httpx.QueryBool(r, "has_violations"); err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.ListDevices(r.Context(), f, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) myDevices(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.MyDevices(r.Context(), page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) getDevice(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.GetDevice(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

type devicePatchRequest struct {
	Hostname        *string              `json:"hostname"`
	OperatingSystem *string              `json:"operating_system"`
	EmployeeID      opt.Field[uuid.UUID] `json:"employee_id"`
}

func (h *handler) updateDevice(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req devicePatchRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	d, err := h.svc.UpdateDevice(r.Context(), id, domain.DevicePatch{
		Hostname: req.Hostname, OperatingSystem: req.OperatingSystem, EmployeeID: req.EmployeeID,
	})
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}

func (h *handler) deviceSoftware(w http.ResponseWriter, r *http.Request) {
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
	removed, err := httpx.QueryBool(r, "include_removed")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.DeviceSoftware(r.Context(), id, removed != nil && *removed, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) deviceCompliance(w http.ResponseWriter, r *http.Request) {
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
	items, total, err := h.svc.DeviceFindings(r.Context(), id, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) listEmployees(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	items, total, err := h.svc.ListEmployees(r.Context(), r.URL.Query().Get("search"), r.URL.Query().Get("department"), page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) getEmployee(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.GetEmployee(r.Context(), id)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

type employeePatchRequest struct {
	FullName   *string `json:"full_name"`
	Department *string `json:"department"`
	Position   *string `json:"position"`
	Email      *string `json:"email"`
	IsActive   *bool   `json:"is_active"`
}

func (h *handler) updateEmployee(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		h.fail(w, r, err)
		return
	}
	var req employeePatchRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		h.fail(w, r, err)
		return
	}
	e, err := h.svc.UpdateEmployee(r.Context(), id, domain.EmployeePatch(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, e)
}

func (h *handler) employeeDevices(w http.ResponseWriter, r *http.Request) {
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
	items, total, err := h.svc.EmployeeDevices(r.Context(), id, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}

func (h *handler) employeeSoftware(w http.ResponseWriter, r *http.Request) {
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
	items, total, err := h.svc.EmployeeSoftware(r.Context(), id, page)
	if err != nil {
		h.fail(w, r, err)
		return
	}
	respondPage(w, items, page, total)
}
