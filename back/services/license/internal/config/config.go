package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"

	"sam/pkg/buildconfig"

	"sam/pkg/postgres"
	"sam/services/license/internal/platform/llm"
)

type Config struct {
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	HTTP       HTTP            `envPrefix:"HTTP_"`
	GRPC       GRPC            `envPrefix:"GRPC_"`
	Auth       Auth            `envPrefix:"JWT_"`
	Postgres   postgres.Config `envPrefix:"POSTGRES_"`
	LLM        llm.Config      `envPrefix:"LLM_"`
	Matching   Matching        `envPrefix:"MATCH_"`
	Compliance Compliance      `envPrefix:"COMPLIANCE_"`
}

type HTTP struct {
	Addr            string        `env:"ADDR" envDefault:":8083"`
	ReadTimeout     time.Duration `env:"READ_TIMEOUT" envDefault:"15s"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT" envDefault:"150s"`
	IdleTimeout     time.Duration `env:"IDLE_TIMEOUT" envDefault:"120s"`
	RequestTimeout  time.Duration `env:"REQUEST_TIMEOUT" envDefault:"120s"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"20s"`
}

type GRPC struct {
	Addr string `env:"ADDR" envDefault:":9093"`
}

type Auth struct {
	PublicKeyFile string        `env:"PUBLIC_KEY_FILE,required"`
	ServiceSecret string        `env:"SERVICE_SECRET,required"`
	Leeway        time.Duration `env:"LEEWAY" envDefault:"30s"`
}

type Matching struct {
	HighThreshold       float64 `env:"HIGH_THRESHOLD" envDefault:"0.95"`
	CandidateThreshold  float64 `env:"CANDIDATE_THRESHOLD" envDefault:"0.65"`
	MinGap              float64 `env:"MIN_GAP" envDefault:"0.1"`
	MaxCandidates       int     `env:"MAX_CANDIDATES" envDefault:"5"`
	LLMAcceptConfidence float64 `env:"LLM_ACCEPT_CONFIDENCE" envDefault:"0.85"`
}

type Compliance struct {
	CheckInterval      time.Duration `env:"CHECK_INTERVAL" envDefault:"1h"`
	ExpiringWithinDays int           `env:"EXPIRING_WITHIN_DAYS" envDefault:"30"`
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
