package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

type Config struct {
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	HTTP      HTTP      `envPrefix:"HTTP_"`
	Auth      Auth      `envPrefix:"JWT_"`
	Upstreams Upstreams `envPrefix:"UPSTREAM_"`
	Limits    Limits    `envPrefix:"LIMIT_"`

	UsersFile   string   `env:"USERS_FILE,required"`
	CORSOrigins []string `env:"CORS_ALLOWED_ORIGINS" envDefault:"http://localhost:3000,http://localhost:5173"`
}

type HTTP struct {
	Addr            string        `env:"ADDR" envDefault:":8080"`
	ReadTimeout     time.Duration `env:"READ_TIMEOUT" envDefault:"15m"`
	IdleTimeout     time.Duration `env:"IDLE_TIMEOUT" envDefault:"120s"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"20s"`
}

type Auth struct {
	PrivateKeyFile string        `env:"PRIVATE_KEY_FILE,required"`
	PublicKeyFile  string        `env:"PUBLIC_KEY_FILE,required"`
	ServiceSecret  string        `env:"SERVICE_SECRET,required"`
	TTL            time.Duration `env:"TTL" envDefault:"8h"`
	Leeway         time.Duration `env:"LEEWAY" envDefault:"30s"`
}

type Upstreams struct {
	Processor string `env:"PROCESSOR" envDefault:"http://localhost:8081"`
	License   string `env:"LICENSE" envDefault:"http://localhost:8083"`
	Chat      string `env:"CHAT" envDefault:"http://localhost:8084"`
}

type Limits struct {
	MaxBodyKB      int64         `env:"MAX_BODY_KB" envDefault:"1024"`
	MaxUploadMB    int64         `env:"MAX_UPLOAD_MB" envDefault:"50"`
	RequestTimeout time.Duration `env:"REQUEST_TIMEOUT" envDefault:"130s"`
	UploadTimeout  time.Duration `env:"UPLOAD_TIMEOUT" envDefault:"10m"`
	LoginPerMinute int           `env:"LOGIN_PER_MINUTE" envDefault:"20"`
	ChatPerMinute  int           `env:"CHAT_PER_MINUTE" envDefault:"30"`
}

func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}
