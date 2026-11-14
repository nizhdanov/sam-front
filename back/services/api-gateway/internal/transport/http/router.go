package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"sam/pkg/auth"
	"sam/pkg/health"
	"sam/pkg/httpx"
	"sam/services/api-gateway/internal/identity"
)

type Upstreams struct {
	Processor string
	License   string
	Chat      string
}

type Limits struct {
	MaxBodyBytes   int64
	MaxUploadBytes int64
	RequestTimeout time.Duration
	UploadTimeout  time.Duration
	LoginPerMinute int
	ChatPerMinute  int
}

type Deps struct {
	Log         *slog.Logger
	Identity    identity.Provider
	Signer      *auth.Signer
	Verifier    *auth.Verifier
	Health      *health.Checker
	Upstreams   Upstreams
	Limits      Limits
	CORSOrigins []string
}

type handler struct {
	log      *slog.Logger
	identity identity.Provider
	signer   *auth.Signer
}

func NewRouter(d Deps) (http.Handler, error) {
	processor, err := newProxy(d.Upstreams.Processor, d.Log)
	if err != nil {
		return nil, err
	}
	license, err := newProxy(d.Upstreams.License, d.Log)
	if err != nil {
		return nil, err
	}
	chat, err := newProxy(d.Upstreams.Chat, d.Log)
	if err != nil {
		return nil, err
	}

	h := &handler{log: d.Log, identity: d.Identity, signer: d.Signer}
	lim := d.Limits
	regular := func(next http.Handler) http.Handler {
		return withTimeout(lim.RequestTimeout, limitBody(lim.MaxBodyBytes, next))
	}
	upload := func(next http.Handler) http.Handler {
		return withTimeout(lim.UploadTimeout, limitBody(lim.MaxUploadBytes, next))
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(httpx.RequestLogger(d.Log))
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   d.CORSOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Authorization", "Content-Type", middleware.RequestIDHeader},
		ExposedHeaders:   []string{middleware.RequestIDHeader},
		AllowCredentials: true,
		MaxAge:           600,
	}))
	r.NotFound(httpx.NotFound)
	r.MethodNotAllowed(httpx.MethodNotAllowed)

	r.Get("/healthz", d.Health.LiveHandler())
	r.Get("/readyz", d.Health.ReadyHandler())

	r.Route("/api/v1", func(r chi.Router) {
		r.With(newLimiter(lim.LoginPerMinute, 5).middleware).Post("/auth/login", h.login)

		r.Group(func(r chi.Router) {
			r.Use(auth.Middleware(d.Verifier))
			r.Get("/auth/me", h.me)

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRoles(auth.RoleAdmin))
				r.Handle("/imports", upload(processor))
				r.Handle("/imports/*", regular(processor))
			})
			r.With(auth.RequireRoles(auth.RoleAdmin, auth.RoleInventoryAgent)).Handle("/agent/inventory", upload(processor))

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRoles(auth.Privileged...))
				for _, prefix := range []string{"/employees", "/licenses", "/matching", "/agreements"} {
					r.Handle(prefix, regular(license))
					r.Handle(prefix+"/*", regular(license))
				}
			})

			r.Group(func(r chi.Router) {
				r.Use(auth.RequireRoles(auth.UserRoles...))
				for _, prefix := range []string{"/devices", "/me", "/software", "/compliance", "/analytics"} {
					r.Handle(prefix, regular(license))
					r.Handle(prefix+"/*", regular(license))
				}
				r.With(newLimiter(lim.ChatPerMinute, 10).middleware).Handle("/chat/*", limitBody(lim.MaxBodyBytes, chat))
			})
		})
	})
	return r, nil
}
