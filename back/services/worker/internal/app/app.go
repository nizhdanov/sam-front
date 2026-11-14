package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"

	"sam/pkg/auth"
	"sam/pkg/health"
	"sam/pkg/natsx"
	"sam/pkg/postgres"
	"sam/services/worker/internal/config"
	"sam/services/worker/internal/matching"
	"sam/services/worker/internal/platform/license"
	"sam/services/worker/internal/platform/llm"
	pgrepo "sam/services/worker/internal/repository/postgres"
	"sam/services/worker/internal/service"
	"sam/services/worker/internal/transport/events"
)

type App struct {
	cfg      config.Config
	log      *slog.Logger
	http     *http.Server
	consumer *events.Consumer
	closers  []func()
}

func New(ctx context.Context, cfg config.Config, log *slog.Logger) (_ *App, err error) {
	a := &App{cfg: cfg, log: log}
	defer func() {
		if err != nil {
			a.Close()
		}
	}()

	pool, err := postgres.Connect(ctx, cfg.Postgres)
	if err != nil {
		return nil, err
	}
	a.closers = append(a.closers, pool.Close)

	signer := auth.NewServiceSigner(cfg.ServiceSecret, "worker", auth.RoleInventoryWorker)
	licenseConn, err := license.Connect(cfg.License, signer)
	if err != nil {
		return nil, err
	}
	a.closers = append(a.closers, func() { _ = licenseConn.Close() })
	catalog := license.New(licenseConn)

	var model matching.Model
	if cfg.LLM.Enabled {
		llmConn, err := llm.Connect(cfg.LLM, signer)
		if err != nil {
			return nil, err
		}
		a.closers = append(a.closers, func() { _ = llmConn.Close() })
		model = llm.New(llmConn)
	}

	nc, js, err := natsx.Connect(cfg.NATS, "worker", log)
	if err != nil {
		return nil, err
	}
	a.closers = append(a.closers, func() { _ = nc.Drain() })
	if _, err := natsx.EnsureInventoryStream(ctx, js, cfg.NATS); err != nil {
		return nil, fmt.Errorf("ensure inventory stream: %w", err)
	}

	matcher := matching.New(catalog, model, matching.Config{CacheSize: cfg.Cache.MaxEntries, CacheTTL: cfg.Cache.TTL}, log)
	svc := service.New(pgrepo.NewSnapshots(pool), matcher, catalog, service.Config{
		Lease:          cfg.Worker.Lease,
		ProcessTimeout: cfg.Worker.ProcessTimeout,
	}, log)
	a.consumer, err = events.NewConsumer(ctx, js, cfg.Consumer, svc, log)
	if err != nil {
		return nil, fmt.Errorf("create consumer: %w", err)
	}

	checker := health.New(2 * time.Second)
	checker.Add("postgres", pool.Ping)
	checker.Add("nats", natsx.Ready(nc))
	r := chi.NewRouter()
	r.Get("/healthz", checker.LiveHandler())
	r.Get("/readyz", checker.ReadyHandler())
	a.http = &http.Server{Addr: cfg.HTTPAddr, Handler: r, ReadHeaderTimeout: 5 * time.Second}
	return a, nil
}

func (a *App) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		a.log.Info("consuming inventory snapshots")
		return a.consumer.Run(gctx)
	})
	g.Go(func() error {
		<-gctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return a.http.Shutdown(shCtx)
	})
	err := g.Wait()
	a.log.Info("stopped", "err", err)
	return err
}

func (a *App) Close() {
	for i := len(a.closers) - 1; i >= 0; i-- {
		a.closers[i]()
	}
	a.closers = nil
}
