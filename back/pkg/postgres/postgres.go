package postgres

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

type Config struct {
	DSN             string        `env:"DSN,required"`
	MaxConns        int32         `env:"MAX_CONNS" envDefault:"5"`
	MinConns        int32         `env:"MIN_CONNS" envDefault:"0"`
	MaxConnLifetime time.Duration `env:"MAX_CONN_LIFETIME" envDefault:"30m"`
	MaxConnIdleTime time.Duration `env:"MAX_CONN_IDLE_TIME" envDefault:"5m"`
	ConnectTimeout  time.Duration `env:"CONNECT_TIMEOUT" envDefault:"30s"`
}

func Connect(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	pcfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	pcfg.MaxConns = cfg.MaxConns
	pcfg.MinConns = cfg.MinConns
	pcfg.MaxConnLifetime = cfg.MaxConnLifetime
	pcfg.MaxConnIdleTime = cfg.MaxConnIdleTime

	pool, err := pgxpool.NewWithConfig(ctx, pcfg)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, cfg.ConnectTimeout)
	defer cancel()
	for {
		if err = pool.Ping(ctx); err == nil {
			return pool, nil
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, fmt.Errorf("ping postgres: %w", err)
		case <-time.After(time.Second):
		}
	}
}

func Migrate(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS, log *slog.Logger) error {
	db := stdlib.OpenDBFromPool(pool)
	defer func() { _ = db.Close() }()

	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("goose locker: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, fsys, goose.WithSessionLocker(locker))
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return fmt.Errorf("postgres migrations: %w", err)
	}
	for _, r := range results {
		log.Info("postgres migration applied", "version", r.Source.Version, "file", r.Source.Path)
	}
	return nil
}

func IsUniqueViolation(err error) bool { return hasCode(err, "23505") }

func IsForeignKeyViolation(err error) bool { return hasCode(err, "23503") }

func IsNoRows(err error) bool { return errors.Is(err, pgx.ErrNoRows) }

func hasCode(err error, code string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == code
}
