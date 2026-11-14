package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"

	"sam/pkg/buildconfig"

	"sam/pkg/postgres"
	"sam/services/chat/internal/platform/llm"
)

type Config struct {
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	HTTP     HTTP            `envPrefix:"HTTP_"`
	Auth     Auth            `envPrefix:"JWT_"`
	Postgres postgres.Config `envPrefix:"POSTGRES_"`
	LLM      llm.Config      `envPrefix:"LLM_"`
	Chat     Chat            `envPrefix:"CHAT_"`
}

type HTTP struct {
	Addr            string        `env:"ADDR" envDefault:":8084"`
	RequestTimeout  time.Duration `env:"REQUEST_TIMEOUT" envDefault:"15s"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"20s"`
}

type Auth struct {
	PublicKeyFile string        `env:"PUBLIC_KEY_FILE,required"`
	ServiceSecret string        `env:"SERVICE_SECRET,required"`
	Leeway        time.Duration `env:"LEEWAY" envDefault:"30s"`
}

type Chat struct {
	MaxHistoryMessages int `env:"MAX_HISTORY_MESSAGES" envDefault:"12"`
	MaxMessageLength   int `env:"MAX_MESSAGE_LENGTH" envDefault:"4000"`
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
