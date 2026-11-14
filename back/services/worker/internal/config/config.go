package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"

	"sam/pkg/buildconfig"

	"sam/pkg/natsx"
	"sam/pkg/postgres"
	"sam/services/worker/internal/platform/license"
	"sam/services/worker/internal/platform/llm"
	"sam/services/worker/internal/transport/events"
)

type Config struct {
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	HTTPAddr      string          `env:"HTTP_ADDR" envDefault:":8082"`
	ServiceSecret string          `env:"JWT_SERVICE_SECRET,required"`
	Postgres      postgres.Config `envPrefix:"POSTGRES_"`
	NATS          natsx.Config    `envPrefix:"NATS_"`
	License       license.Config  `envPrefix:"LICENSE_"`
	LLM           llm.Config      `envPrefix:"LLM_"`
	Consumer      events.Config   `envPrefix:"WORKER_"`
	Worker        Worker          `envPrefix:"WORKER_"`
	Cache         Cache           `envPrefix:"MATCH_CACHE_"`
}

type Worker struct {
	ProcessTimeout time.Duration `env:"PROCESS_TIMEOUT" envDefault:"300s"`
	Lease          time.Duration `env:"LEASE" envDefault:"120s"`
}

type Cache struct {
	MaxEntries int           `env:"MAX_ENTRIES" envDefault:"10000"`
	TTL        time.Duration `env:"TTL" envDefault:"600s"`
}

func Load() (Config, error) {
	values, err := buildconfig.Environment()
	if err != nil {
		return Config{}, err
	}
	cfg, err := env.ParseAsWithOptions[Config](env.Options{Environment: values})
	if err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}
