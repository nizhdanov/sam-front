package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/pkg/health"
	"sam/pkg/httpx"
	"sam/services/license/internal/service"
)

type Deps struct {
	Log            *slog.Logger
	Verifier       *auth.Verifier
	Health         *health.Checker
	Service        *service.Service
	RequestTimeout time.Duration
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
		r.Use(middleware.Timeout(d.RequestTimeout))
		r.Use(auth.Middleware(d.Verifier))

		r.Get("/me/device", h.myDevice)
		r.Get("/dashboard", h.dashboard)
		r.Get("/balance", h.balance)
		r.Get("/audit", h.auditLog)
		r.Get("/data/{dataset}", h.queryData)
		r.Post("/export", h.exportTable)

		r.Route("/devices", func(r chi.Router) {
			r.Get("/{id}", h.getDevice)
			r.Patch("/{id}", h.updateDevice)
		})

		r.Route("/software", func(r chi.Router) {
			r.Get("/", h.listProducts)
			r.Post("/", h.createProduct)
			r.Get("/{id}", h.getProduct)
			r.Patch("/{id}", h.updateProduct)
			r.Delete("/{id}", h.deleteProduct)
			r.Get("/{id}/versions", h.listVersions)
			r.Post("/{id}/versions", h.createVersion)
			r.Patch("/{id}/versions/{versionID}", h.updateVersion)
		})

		r.Route("/contracts", func(r chi.Router) {
			r.Get("/", h.listContracts)
			r.Post("/", h.createContract)
			r.Get("/{id}", h.getContract)
			r.Patch("/{id}", h.updateContract)
			r.Post("/{id}/deactivate", h.deactivateContract)
		})

		r.Route("/agreements", func(r chi.Router) {
			r.Get("/", h.listAgreements)
			r.Post("/", h.createAgreement)
			r.Get("/{id}", h.getAgreement)
			r.Patch("/{id}", h.updateAgreement)
			r.Delete("/{id}", h.deleteAgreement)
			r.Post("/{id}/file", h.uploadAgreementFile)
			r.Post("/{id}/analyze", h.analyzeAgreement)
			r.Get("/{id}/analysis", h.getAnalysis)
			r.Post("/{id}/analysis/approve", h.approveAnalysis)
			r.Get("/{id}/similar", h.similarAgreements)
		})

		r.Route("/compliance", func(r chi.Router) {
			r.Get("/findings", h.listFindings)
			r.Post("/findings/{id}/acknowledge", h.acknowledgeFinding)
			r.Get("/summary", h.complianceSummary)
			r.Post("/recalculate", h.recalculate)
		})

		r.Route("/matching", func(r chi.Router) {
			r.Get("/pending", h.pendingMatches)
			r.Post("/rematch", h.rematch)
			r.Post("/{id}/approve", h.approveMatch)
			r.Post("/{id}/reject", h.rejectMatch)
			r.Get("/rules", h.listRules)
			r.Post("/rules", h.createRule)
			r.Patch("/rules/{id}", h.updateRule)
			r.Delete("/rules/{id}", h.deleteRule)
			r.Get("/settings", h.matchingSettings)
			r.Put("/settings", h.updateMatchingSettings)
		})
	})
	return r
}

type handler struct {
	svc *service.Service
	log *slog.Logger
}

func (h *handler) fail(w http.ResponseWriter, r *http.Request, err error) {
	httpx.Error(w, r, h.log, err)
}

func pathID(r *http.Request, name string) (uuid.UUID, error) {
	id, err := uuid.Parse(chi.URLParam(r, name))
	if err != nil {
		return uuid.Nil, apperr.Invalid(name + " must be a UUID")
	}
	return id, nil
}

func queryID(r *http.Request, name string) (*uuid.UUID, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, apperr.Invalid(name + " must be a UUID")
	}
	return &id, nil
}

func respondPage[T any](w http.ResponseWriter, items []T, page httpx.PageRequest, total int) {
	httpx.JSON(w, http.StatusOK, httpx.NewPage(items, page, total))
}

type timestamp time.Time

func (t *timestamp) UnmarshalJSON(b []byte) error {
	s := string(b)
	if len(s) < 2 || s[0] != '"' {
		return apperr.Invalid("timestamp must be a string")
	}
	v, err := parseTimestamp(s[1 : len(s)-1])
	if err != nil {
		return err
	}
	*t = timestamp(v)
	return nil
}

func parseTimestamp(s string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, time.DateOnly} {
		if v, err := time.Parse(layout, s); err == nil {
			return v, nil
		}
	}
	return time.Time{}, apperr.Invalid("timestamp must be RFC3339 or YYYY-MM-DD")
}

func (t *timestamp) ptr() *time.Time {
	if t == nil {
		return nil
	}
	v := time.Time(*t)
	return &v
}
