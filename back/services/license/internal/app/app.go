package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"

	"sam/pkg/auth"
	"sam/pkg/health"
	"sam/pkg/postgres"
	"sam/services/license/internal/config"
	"sam/services/license/internal/domain"
	"sam/services/license/internal/platform/llm"
	pgrepo "sam/services/license/internal/repository/postgres"
	"sam/services/license/internal/service"
	grpctransport "sam/services/license/internal/transport/grpc"
	httptransport "sam/services/license/internal/transport/http"
	"sam/services/license/migrations"
)

type App struct {
	cfg     config.Config
	log     *slog.Logger
	http    *http.Server
	grpc    *grpc.Server
	grpcHS  *grpchealth.Server
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

	pubKey, err := os.ReadFile(cfg.Auth.PublicKeyFile)
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

	llmConn, err := llm.Connect(cfg.LLM, auth.NewServiceSigner(cfg.Auth.ServiceSecret, "license", auth.RoleLicenseService))
	if err != nil {
		return nil, err
	}
	a.closers = append(a.closers, func() { _ = llmConn.Close() })

	a.svc, err = service.New(service.Deps{
		Tx:            postgres.NewTxRunner(pool),
		Locks:         pgrepo.NewLocks(pool),
		Products:      pgrepo.NewProducts(pool),
		Versions:      pgrepo.NewVersions(pool),
		Aliases:       pgrepo.NewAliases(pool),
		Matching:      pgrepo.NewMatching(pool),
		Devices:       pgrepo.NewDevices(pool),
		Employees:     pgrepo.NewEmployees(pool),
		Installations: pgrepo.NewInstallations(pool),
		Entitlements:  pgrepo.NewEntitlements(pool),
		Agreements:    pgrepo.NewAgreements(pool),
		Findings:      pgrepo.NewFindings(pool),
		Analytics:     pgrepo.NewAnalytics(pool),
		Analyzer:      llm.New(llmConn),
		Log:           log,
	}, domain.MatchingConfig(cfg.Matching))
	if err != nil {
		return nil, err
	}

	checker := health.New(2 * time.Second)
	checker.Add("postgres", pool.Ping)

	a.http = &http.Server{
		Addr: cfg.HTTP.Addr,
		Handler: httptransport.NewRouter(httptransport.Deps{
			Log:            log,
			Verifier:       verifier,
			Health:         checker,
			Service:        a.svc,
			RequestTimeout: cfg.HTTP.RequestTimeout,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}
	a.grpc, a.grpcHS = grpctransport.NewServer(log, verifier, a.svc)
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

	g.Go(func() error {
		lis, err := (&net.ListenConfig{}).Listen(gctx, "tcp", a.cfg.GRPC.Addr)
		if err != nil {
			return fmt.Errorf("grpc listen: %w", err)
		}
		a.log.Info("grpc server listening", "addr", a.cfg.GRPC.Addr)
		if err := a.grpc.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc server: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		return a.svc.RunPeriodicChecks(gctx, a.cfg.Compliance.CheckInterval)
	})

	g.Go(func() error {
		<-gctx.Done()
		a.log.Info("shutting down")
		shCtx, cancel := context.WithTimeout(context.Background(), a.cfg.HTTP.ShutdownTimeout)
		defer cancel()

		a.grpcHS.Shutdown()
		var errs []error
		if err := a.http.Shutdown(shCtx); err != nil {
			errs = append(errs, fmt.Errorf("http shutdown: %w", err))
		}
		stopped := make(chan struct{})
		go func() { a.grpc.GracefulStop(); close(stopped) }()
		select {
		case <-stopped:
		case <-shCtx.Done():
			a.grpc.Stop()
			errs = append(errs, errors.New("grpc graceful stop timed out"))
		}
		return errors.Join(errs...)
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
