package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"

	"sam/pkg/natsx"
	"sam/pkg/postgres"
)

type Config struct {
	LogLevel  string `env:"LOG_LEVEL" envDefault:"info"`
	LogFormat string `env:"LOG_FORMAT" envDefault:"json"`

	HTTP     HTTP            `envPrefix:"HTTP_"`
	Auth     Auth            `envPrefix:"JWT_"`
	Postgres postgres.Config `envPrefix:"POSTGRES_"`
	NATS     natsx.Config    `envPrefix:"NATS_"`
	Import   Import          `envPrefix:"IMPORT_"`
}

type HTTP struct {
	Addr            string        `env:"ADDR" envDefault:":8081"`
	RequestTimeout  time.Duration `env:"REQUEST_TIMEOUT" envDefault:"30s"`
	UploadTimeout   time.Duration `env:"UPLOAD_TIMEOUT" envDefault:"10m"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"20s"`
}

type Auth struct {
	PublicKeyFile string        `env:"PUBLIC_KEY_FILE,required"`
	ServiceSecret string        `env:"SERVICE_SECRET,required"`
	Leeway        time.Duration `env:"LEEWAY" envDefault:"30s"`
}

type Import struct {
	StorageDir           string        `env:"STORAGE_DIR" envDefault:"/data/imports"`
	MaxFileSizeMB        int64         `env:"MAX_FILE_SIZE_MB" envDefault:"50"`
	MaxRows              int           `env:"MAX_ROWS" envDefault:"500000"`
	MaxFieldBytes        int           `env:"MAX_FIELD_BYTES" envDefault:"512"`
	MaxConcurrentUploads int           `env:"MAX_CONCURRENT_UPLOADS" envDefault:"2"`
	ParseConcurrency     int           `env:"PARSE_CONCURRENCY" envDefault:"1"`
	RetentionDays        int           `env:"RETENTION_DAYS" envDefault:"7"`
	CleanupInterval      time.Duration `env:"CLEANUP_INTERVAL" envDefault:"1h"`
}

func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return cfg, nil
}
