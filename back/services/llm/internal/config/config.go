package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"

	"sam/pkg/buildconfig"

	"sam/services/llm/internal/platform/license"
)

type Config struct {
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	HTTPAddr string         `env:"HTTP_ADDR" envDefault:":8085"`
	GRPCAddr string         `env:"GRPC_ADDR" envDefault:":9095"`
	Auth     Auth           `envPrefix:"JWT_"`
	License  license.Config `envPrefix:"LICENSE_"`
	LLM      LLM            `envPrefix:"LLM_"`
	GigaChat GigaChat       `envPrefix:"GIGACHAT_"`
}

type Auth struct {
	PublicKeyFile string        `env:"PUBLIC_KEY_FILE,required"`
	ServiceSecret string        `env:"SERVICE_SECRET,required"`
	Leeway        time.Duration `env:"LEEWAY" envDefault:"30s"`
}

type LLM struct {
	Provider         string        `env:"PROVIDER" envDefault:"mock"`
	BaseURL          string        `env:"BASE_URL"`
	APIKey           string        `env:"API_KEY"`
	ChatModel        string        `env:"CHAT_MODEL" envDefault:"GigaChat"`
	MatchingModel    string        `env:"MATCHING_MODEL" envDefault:"GigaChat"`
	EULAModel        string        `env:"EULA_MODEL" envDefault:"GigaChat"`
	Timeout          time.Duration `env:"TIMEOUT_SECONDS" envDefault:"45s"`
	MaxConcurrent    int           `env:"MAX_CONCURRENT_REQUESTS" envDefault:"4"`
	MaxToolCalls     int           `env:"MAX_TOOL_CALLS" envDefault:"5"`
	MaxOutputTokens  int           `env:"MAX_OUTPUT_TOKENS" envDefault:"1500"`
	MaxContextTokens int           `env:"MAX_CONTEXT_TOKENS" envDefault:"8000"`
	DailyTokenLimit  int           `env:"DAILY_TOKEN_LIMIT" envDefault:"500000"`
}

type GigaChat struct {
	AuthURL            string `env:"AUTH_URL" envDefault:"https://ngw.devices.sberbank.ru:9443/api/v2/oauth"`
	BaseURL            string `env:"BASE_URL" envDefault:"https://gigachat.devices.sberbank.ru/api/v1"`
	AuthKey            string `env:"AUTH_KEY"`
	ClientID           string `env:"CLIENT_ID"`
	ClientSecret       string `env:"CLIENT_SECRET"`
	Scope              string `env:"SCOPE" envDefault:"GIGACHAT_API_PERS"`
	CACertPath         string `env:"CA_CERT_PATH"`
	InsecureSkipVerify bool   `env:"INSECURE_SKIP_VERIFY" envDefault:"false"`
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
