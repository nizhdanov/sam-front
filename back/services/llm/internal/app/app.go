package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"

	"sam/pkg/auth"
	"sam/pkg/buildconfig"
	"sam/pkg/health"
	"sam/services/llm/internal/config"
	"sam/services/llm/internal/platform/license"
	"sam/services/llm/internal/service"
	"sam/services/llm/internal/tools"
	grpctransport "sam/services/llm/internal/transport/grpc"
)

type App struct {
	cfg     config.Config
	log     *slog.Logger
	http    *http.Server
	grpc    *grpc.Server
	grpcHS  *grpchealth.Server
	closers []func()
}

func New(cfg config.Config, log *slog.Logger) (_ *App, err error) {
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

	model, err := newProvider(cfg)
	if err != nil {
		return nil, err
	}
	if model.Name() == "mock" {
		log.Warn("LLM provider is mock: AI answers are demonstration only")
	}

	licenseConn, err := license.Connect(cfg.License)
	if err != nil {
		return nil, err
	}
	a.closers = append(a.closers, func() { _ = licenseConn.Close() })

	svc := service.New(model, tools.NewExecutor(license.New(licenseConn)), service.Config{
		Models:          service.Models{Chat: cfg.LLM.ChatModel, Matching: cfg.LLM.MatchingModel, EULA: cfg.LLM.EULAModel},
		MaxConcurrent:   cfg.LLM.MaxConcurrent,
		MaxToolCalls:    cfg.LLM.MaxToolCalls,
		MaxOutputTokens: cfg.LLM.MaxOutputTokens,
		MaxContextChars: cfg.LLM.MaxContextTokens * 3,
		DailyTokenLimit: cfg.LLM.DailyTokenLimit,
	}, log)

	checker := health.New(2 * time.Second)
	r := chi.NewRouter()
	r.Get("/healthz", checker.LiveHandler())
	r.Get("/readyz", checker.ReadyHandler())
	a.http = &http.Server{Addr: cfg.HTTPAddr, Handler: r, ReadHeaderTimeout: 5 * time.Second}
	a.grpc, a.grpcHS = grpctransport.NewServer(log, verifier, svc)
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
		lis, err := (&net.ListenConfig{}).Listen(gctx, "tcp", a.cfg.GRPCAddr)
		if err != nil {
			return fmt.Errorf("grpc listen: %w", err)
		}
		a.log.Info("grpc server listening", "addr", a.cfg.GRPCAddr, "provider", a.cfg.LLM.Provider)
		if err := a.grpc.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			return fmt.Errorf("grpc server: %w", err)
		}
		return nil
	})
	g.Go(func() error {
		<-gctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		a.grpcHS.Shutdown()
		stopped := make(chan struct{})
		go func() { a.grpc.GracefulStop(); close(stopped) }()
		select {
		case <-stopped:
		case <-shCtx.Done():
			a.grpc.Stop()
		}
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
