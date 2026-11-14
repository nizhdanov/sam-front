package http

import (
	"net/http"

	"sam/pkg/httpx"
	"sam/pkg/opt"
	"sam/services/license/internal/domain"
)

func (h *handler) myDevice(w http.ResponseWriter, r *http.Request) {
	d, err := h.svc.MyDevice(r.Context())
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
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
	Type        *domain.DeviceType   `json:"device_type"`
	Status      *domain.DeviceStatus `json:"status"`
	Hostname    opt.Field[string]    `json:"hostname"`
	Responsible opt.Field[string]    `json:"responsible"`
	Location    opt.Field[string]    `json:"location"`
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
	d, err := h.svc.UpdateDevice(r.Context(), id, domain.DevicePatch(req))
	if err != nil {
		h.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, d)
}
