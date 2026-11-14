package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"sam/pkg/auth"
	"sam/pkg/buildconfig"
	"sam/pkg/health"
	"sam/pkg/natsx"
	"sam/pkg/postgres"
	"sam/services/processor/internal/config"
	"sam/services/processor/internal/platform/publisher"
	pgrepo "sam/services/processor/internal/repository/postgres"
	"sam/services/processor/internal/service"
	"sam/services/processor/internal/storage"
	httptransport "sam/services/processor/internal/transport/http"
	"sam/services/processor/migrations"
)

type App struct {
	cfg     config.Config
	log     *slog.Logger
	http    *http.Server
	svc     *service.Service
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

	nc, js, err := natsx.Connect(cfg.NATS, "processor", log)
	if err != nil {
		return nil, err
	}
	a.closers = append(a.closers, func() { _ = nc.Drain() })
	if _, err := natsx.EnsureInventoryStream(ctx, js, cfg.NATS); err != nil {
		log.Warn("inventory stream is not ready yet, publishing will retry", "err", err)
	}

	files, err := storage.New(cfg.Import.StorageDir)
	if err != nil {
		return nil, err
	}

	a.svc, err = service.New(service.Deps{
		Tx:        postgres.NewTxRunner(pool),
		Imports:   pgrepo.NewImports(pool),
		Snapshots: pgrepo.NewSnapshots(pool),
		Errors:    pgrepo.NewImportErrors(pool),
		Files:     files,
		Publisher: publisher.New(js),
		Log:       log,
	}, service.Config{
		MaxFileBytes:       cfg.Import.MaxFileSizeMB << 20,
		MaxRows:            cfg.Import.MaxRows,
		MaxFieldBytes:      cfg.Import.MaxFieldBytes,
		MaxConcurrentLoads: cfg.Import.MaxConcurrentUploads,
		ParseConcurrency:   cfg.Import.ParseConcurrency,
		Retention:          time.Duration(cfg.Import.RetentionDays) * 24 * time.Hour,
	})
	if err != nil {
		return nil, err
	}

	checker := health.New(2 * time.Second)
	checker.Add("postgres", pool.Ping)
	checker.Add("nats", natsx.Ready(nc))

	a.http = &http.Server{
		Addr: cfg.HTTP.Addr,
		Handler: httptransport.NewRouter(httptransport.Deps{
			Log:            log,
			Verifier:       verifier,
			Health:         checker,
			Service:        a.svc,
			RequestTimeout: cfg.HTTP.RequestTimeout,
			UploadTimeout:  cfg.HTTP.UploadTimeout,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	return a, nil
}

func (a *App) Run(ctx context.Context) error {
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		a.log.Info("http server listening", "addr", a.cfg.HTTP.Addr)
		if err := a.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http server: %w", err)
		}
		return nil
	})
	g.Go(func() error { return a.svc.RunWorkers(gctx) })
	g.Go(func() error { return a.svc.RunRetention(gctx, a.cfg.Import.CleanupInterval) })
	g.Go(func() error {
		<-gctx.Done()
		a.log.Info("shutting down")
		shCtx, cancel := context.WithTimeout(context.Background(), a.cfg.HTTP.ShutdownTimeout)
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
