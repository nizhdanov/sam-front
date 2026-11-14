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

		r.Get("/me/devices", h.myDevices)

		r.Route("/devices", func(r chi.Router) {
			r.Get("/", h.listDevices)
			r.Get("/{id}", h.getDevice)
			r.Patch("/{id}", h.updateDevice)
			r.Get("/{id}/software", h.deviceSoftware)
			r.Get("/{id}/compliance", h.deviceCompliance)
		})

		r.Route("/employees", func(r chi.Router) {
			r.Get("/", h.listEmployees)
			r.Get("/{id}", h.getEmployee)
			r.Patch("/{id}", h.updateEmployee)
			r.Get("/{id}/devices", h.employeeDevices)
			r.Get("/{id}/software", h.employeeSoftware)
		})

		r.Route("/software", func(r chi.Router) {
			r.Get("/", h.listProducts)
			r.Post("/", h.createProduct)
			r.Get("/{id}", h.getProduct)
			r.Patch("/{id}", h.updateProduct)
			r.Patch("/{id}/policy", h.updatePolicy)
			r.Get("/{id}/versions", h.listVersions)
			r.Post("/{id}/versions", h.createVersion)
			r.Patch("/{id}/versions/{versionID}", h.updateVersion)
			r.Get("/{id}/aliases", h.listAliases)
			r.Post("/{id}/aliases", h.createAlias)
			r.Delete("/{id}/aliases/{aliasID}", h.deleteAlias)
			r.Get("/{id}/installations", h.productInstallations)
			r.Get("/{id}/license-balance", h.productBalance)
		})

		r.Route("/licenses", func(r chi.Router) {
			r.Get("/", h.listLicenses)
			r.Post("/", h.createLicense)
			r.Get("/balance", h.balances)
			r.Get("/{id}", h.getLicense)
			r.Patch("/{id}", h.updateLicense)
			r.Post("/{id}/deactivate", h.deactivateLicense)
		})

		r.Route("/agreements", func(r chi.Router) {
			r.Get("/", h.listAgreements)
			r.Post("/", h.createAgreement)
			r.Get("/{id}", h.getAgreement)
			r.Patch("/{id}", h.updateAgreement)
			r.Post("/{id}/analyze", h.analyzeAgreement)
			r.Get("/{id}/analysis", h.getAnalysis)
			r.Post("/{id}/analysis/approve", h.approveAnalysis)
		})

		r.Route("/compliance", func(r chi.Router) {
			r.Get("/findings", h.listFindings)
			r.Get("/findings/{id}", h.getFinding)
			r.Post("/findings/{id}/acknowledge", h.acknowledgeFinding)
			r.Get("/summary", h.complianceSummary)
			r.Post("/recalculate", h.recalculate)
		})

		r.Route("/analytics", func(r chi.Router) {
			r.Get("/dashboard", h.dashboard)
			r.Get("/software", h.softwareAnalytics)
			r.Get("/licenses", h.balances)
			r.Get("/compliance", h.complianceSummary)
			r.Get("/departments", h.departmentAnalytics)
		})

		r.Route("/matching", func(r chi.Router) {
			r.Get("/pending", h.pendingMatches)
			r.Get("/{id}", h.getMatch)
			r.Post("/{id}/approve", h.approveMatch)
			r.Post("/{id}/reject", h.rejectMatch)
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
	s = s[1 : len(s)-1]
	for _, layout := range []string{time.RFC3339, time.DateOnly} {
		if v, err := time.Parse(layout, s); err == nil {
			*t = timestamp(v)
			return nil
		}
	}
	return apperr.Invalid("timestamp must be RFC3339 or YYYY-MM-DD")
}

func (t *timestamp) ptr() *time.Time {
	if t == nil {
		return nil
	}
	v := time.Time(*t)
	return &v
}
