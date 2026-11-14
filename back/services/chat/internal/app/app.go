package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"sam/pkg/auth"
	"sam/pkg/buildconfig"
	"sam/pkg/health"
	"sam/pkg/postgres"
	"sam/services/chat/internal/config"
	"sam/services/chat/internal/platform/llm"
	pgrepo "sam/services/chat/internal/repository/postgres"
	"sam/services/chat/internal/service"
	httptransport "sam/services/chat/internal/transport/http"
	"sam/services/chat/migrations"
)

type App struct {
	cfg     config.Config
	log     *slog.Logger
	http    *http.Server
	closers []func()
}

func New(ctx context.Context, cfg config.Config, log *slog.Logger) (_ *App, err error) {
	a := &App{cfg: cfg, log: log}
	defer func() {
		if err != nil {
			a.Close()
		}
	}()

	pubKey, err := buildconfig.ReadFile(cfg.Auth.PublicKeyFile)
	if err != nil {
		return nil, fmt.Errorf("read jwt public key: %w", err)
	}
	verifier, err := auth.NewVerifier(pubKey, cfg.Auth.ServiceSecret, cfg.Auth.Leeway)
	if err != nil {
		return nil, err
	}

	pool, err := postgres.Connect(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}
	a.closers = append(a.closers, pool.Close)
	if err := postgres.Migrate(ctx, pool, migrations.Postgres(), log); err != nil {
		return nil, err
	}

	llmConn, err := llm.Connect(cfg.LLM, nil)
	if err != nil {
		return nil, err
	}
	a.closers = append(a.closers, func() { _ = llmConn.Close() })

	svc := service.New(pgrepo.NewConversations(pool), pgrepo.NewMessages(pool), llm.New(llmConn), service.Config{
		MaxHistoryMessages: cfg.Chat.MaxHistoryMessages,
		MaxMessageLength:   cfg.Chat.MaxMessageLength,
	}, log)

	checker := health.New(2 * time.Second)
	checker.Add("postgres", pool.Ping)
	a.http = &http.Server{
		Addr: cfg.HTTP.Addr,
		Handler: httptransport.NewRouter(httptransport.Deps{
			Log:            log,
			Verifier:       verifier,
			Health:         checker,
			Service:        svc,
			RequestTimeout: cfg.HTTP.RequestTimeout,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	return a, nil
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
	shCtx, cancel := context.WithTimeout(context.Background(), a.cfg.HTTP.ShutdownTimeout)
	defer cancel()
	return a.http.Shutdown(shCtx)
}

func (a *App) Close() {
	for i := len(a.closers) - 1; i >= 0; i-- {
		a.closers[i]()
	}
	a.closers = nil
}
