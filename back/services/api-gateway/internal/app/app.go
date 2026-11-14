package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"sam/pkg/auth"
	"sam/pkg/health"
	"sam/services/api-gateway/internal/config"
	"sam/services/api-gateway/internal/identity"
	httptransport "sam/services/api-gateway/internal/transport/http"
)

type App struct {
	cfg  config.Config
	log  *slog.Logger
	http *http.Server
}

func New(cfg config.Config, log *slog.Logger) (*App, error) {
	privKey, err := os.ReadFile(cfg.Auth.PrivateKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read jwt private key: %w", err)
	}
	signer, err := auth.NewSigner(privKey, cfg.Auth.TTL)
	if err != nil {
		return nil, err
	}
	pubKey, err := os.ReadFile(cfg.Auth.PublicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read jwt public key: %w", err)
	}
	verifier, err := auth.NewVerifier(pubKey, cfg.Auth.ServiceSecret, cfg.Auth.Leeway)
	if err != nil {
		return nil, err
	}
	users, err := identity.LoadMock(cfg.UsersFile)
	if err != nil {
		return nil, err
	}

	router, err := httptransport.NewRouter(httptransport.Deps{
		Log:      log,
		Identity: users,
		Signer:   signer,
		Verifier: verifier,
		Health:   health.New(2 * time.Second),
		Upstreams: httptransport.Upstreams{
			Processor: cfg.Upstreams.Processor,
			License:   cfg.Upstreams.License,
			Chat:      cfg.Upstreams.Chat,
		},
		Limits: httptransport.Limits{
			MaxBodyBytes:   cfg.Limits.MaxBodyKB << 10,
			MaxUploadBytes: cfg.Limits.MaxUploadMB << 20,
			RequestTimeout: cfg.Limits.RequestTimeout,
			UploadTimeout:  cfg.Limits.UploadTimeout,
			LoginPerMinute: cfg.Limits.LoginPerMinute,
			ChatPerMinute:  cfg.Limits.ChatPerMinute,
		},
		CORSOrigins: cfg.CORSOrigins,
	})
	if err != nil {
		return nil, err
	}

	return &App{cfg: cfg, log: log, http: &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		a.log.Info("http server listening", "addr", a.cfg.HTTP.Addr)
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server: %w", err)
		}
		close(errCh)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}
	a.log.Info("shutting down")
	shCtx, cancel := context.WithTimeout(context.Background(), a.cfg.HTTP.ShutdownTimeout)
	defer cancel()
	return a.http.Shutdown(shCtx)
}
