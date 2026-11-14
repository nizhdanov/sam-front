package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sam/pkg/healthcheck"
	"sam/pkg/logger"
	"sam/services/api-gateway/internal/app"
	"sam/services/api-gateway/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		return healthcheck.Probe(":8080")
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log := logger.New(cfg.LogLevel, cfg.LogFormat).With("service", "api-gateway")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(cfg, log)
	if err != nil {
		return err
	}
	return a.Run(ctx)
}
