package http

import (
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/pkg/health"
	"sam/pkg/httpx"
	"sam/services/processor/internal/domain"
	"sam/services/processor/internal/service"
)

type Deps struct {
	Log            *slog.Logger
	Verifier       *auth.Verifier
	Health         *health.Checker
	Service        *service.Service
	RequestTimeout time.Duration
	UploadTimeout  time.Duration
}

type handler struct {
	svc *service.Service
	log *slog.Logger
}

func NewRouter(d Deps) http.Handler {
	h := &handler{svc: d.Service, log: d.Log}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(httpx.RequestLogger(d.Log))
	r.Use(middleware.Recoverer)
	r.NotFound(httpx.NotFound)
	r.MethodNotAllowed(httpx.MethodNotAllowed)

	r.Get("/healthz", d.Health.LiveHandler())
	r.Get("/readyz", d.Health.ReadyHandler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.Middleware(d.Verifier))

		r.With(middleware.Timeout(d.UploadTimeout)).Post("/imports", h.upload)
		r.With(middleware.Timeout(d.UploadTimeout)).Post("/agent/inventory", h.agentInventory)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(d.RequestTimeout))
			r.Get("/imports", h.list)
			r.Get("/imports/{id}", h.get)
			r.Get("/imports/{id}/errors", h.errors)
			r.Post("/imports/{id}/retry", h.retry)
		})
	})
	return r
}

type importResponse struct {
	ID               uuid.UUID           `json:"id"`
	Filename         string              `json:"filename"`
	Source           domain.Source       `json:"source"`
	Status           domain.ImportStatus `json:"status"`
	FileSize         int64               `json:"file_size"`
	TotalRows        int                 `json:"total_rows"`
	LoadedRows       int                 `json:"loaded_rows"`
	DuplicateRows    int                 `json:"duplicate_rows"`
	ExcludedRows     int                 `json:"excluded_rows"`
	ErrorRows        int                 `json:"error_rows"`
	TotalDevices     int                 `json:"total_devices"`
	ProcessedDevices int                 `json:"processed_devices"`
	FailedDevices    int                 `json:"failed_devices"`
	PendingDevices   int                 `json:"pending_devices"`
	ProgressPercent  float64             `json:"progress_percent"`
	CreatedBy        string              `json:"created_by"`
	CollectedAt      time.Time           `json:"collected_at"`
	ErrorMessage     *string             `json:"error_message"`
	CreatedAt        time.Time           `json:"created_at"`
	CompletedAt      *time.Time          `json:"completed_at"`
}

func toResponse(v service.ImportView) importResponse {
	p := v.Progress
	return importResponse{
		ID:               v.ID,
		Filename:         v.Filename,
		Source:           v.Source,
		Status:           v.Status,
		FileSize:         v.FileSize,
		TotalRows:        v.TotalRows,
		LoadedRows:       v.LoadedRows,
		DuplicateRows:    v.DuplicateRows,
		ExcludedRows:     v.ExcludedRows,
		ErrorRows:        v.ErrorRows,
		TotalDevices:     v.TotalDevices,
		ProcessedDevices: p.Completed + p.Superseded,
		FailedDevices:    p.Failed,
		PendingDevices:   p.Total - p.Finished(),
		ProgressPercent:  float64(int(p.Percent()*100)) / 100,
		CreatedBy:        v.CreatedBy,
		CollectedAt:      v.CollectedAt,
		ErrorMessage:     v.ErrorMessage,
		CreatedAt:        v.CreatedAt,
		CompletedAt:      v.CompletedAt,
	}
}

type acceptedResponse struct {
	ImportID uuid.UUID           `json:"import_id"`
	Status   domain.ImportStatus `json:"status"`
	Filename string              `json:"filename"`
}

func (h *handler) upload(w http.ResponseWriter, r *http.Request) {
	collectedAt, err := parseTime(r.URL.Query().Get("collected_at"))
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	mr, err := r.MultipartReader()
	if err != nil {
		httpx.Error(w, r, h.log, apperr.Invalid("request must be multipart/form-data"))
		return
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			httpx.Error(w, r, h.log, apperr.Invalid("file field is required"))
			return
		}
		if err != nil {
			httpx.Error(w, r, h.log, uploadError(err))
			return
		}
		switch part.FormName() {
		case "collected_at":
			raw, err := io.ReadAll(io.LimitReader(part, 64))
			if err == nil {
				collectedAt, err = parseTime(strings.TrimSpace(string(raw)))
			}
			if err != nil {
				httpx.Error(w, r, h.log, err)
				return
			}
		case "file":
			h.saveUpload(w, r, part, collectedAt)
			return
		}
	}
}

func (h *handler) saveUpload(w http.ResponseWriter, r *http.Request, part *multipart.Part, collectedAt time.Time) {
	imp, err := h.svc.Upload(r.Context(), part.FileName(), part, collectedAt)
	if err != nil {
		httpx.Error(w, r, h.log, uploadError(err))
		return
	}
	httpx.JSON(w, http.StatusAccepted, acceptedResponse{ImportID: imp.ID, Status: imp.Status, Filename: imp.Filename})
}

func uploadError(err error) error {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		return apperr.TooLarge("file is too large")
	}
	return err
}

type agentRequest struct {
	SerialNumber string `json:"serial_number"`
	CollectedAt  string `json:"collected_at"`
	SnapshotType string `json:"snapshot_type"`
	Software     []struct {
		DisplayName string `json:"display_name"`
		Publisher   string `json:"publisher"`
		Version     string `json:"version"`
	} `json:"software"`
}

func (h *handler) agentInventory(w http.ResponseWriter, r *http.Request) {
	var req agentRequest
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	collectedAt, err := parseTime(req.CollectedAt)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	report := service.AgentReport{
		SerialNumber: req.SerialNumber,
		CollectedAt:  collectedAt,
		Partial:      strings.EqualFold(req.SnapshotType, string(domain.SnapshotPartial)),
		Software:     make([]domain.RawSoftware, 0, len(req.Software)),
	}
	for _, sw := range req.Software {
		report.Software = append(report.Software, domain.RawSoftware(sw))
	}
	imp, err := h.svc.IngestAgent(r.Context(), report)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, acceptedResponse{ImportID: imp.ID, Status: imp.Status, Filename: imp.Filename})
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	views, total, err := h.svc.List(r.Context(), page)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	items := make([]importResponse, 0, len(views))
	for _, v := range views {
		items = append(items, toResponse(v))
	}
	httpx.JSON(w, http.StatusOK, httpx.NewPage(items, page, total))
}

func (h *handler) get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	v, err := h.svc.Get(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toResponse(v))
}

func (h *handler) errors(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	page, err := httpx.ParsePage(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	items, total, err := h.svc.Errors(r.Context(), id, page)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusOK, httpx.NewPage(items, page, total))
}

func (h *handler) retry(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	v, err := h.svc.Retry(r.Context(), id)
	if err != nil {
		httpx.Error(w, r, h.log, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, toResponse(v))
}

func pathID(r *http.Request) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		return uuid.Nil, apperr.Invalid("import id must be a UUID")
	}
	return id, nil
}

func parseTime(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, time.DateOnly} {
		if t, err := time.Parse(layout, raw); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, apperr.Invalid("collected_at must be RFC3339 or YYYY-MM-DD")
}
