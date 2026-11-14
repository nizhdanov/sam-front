package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	pg "sam/pkg/postgres"
)

type Settings struct{ pool *pgxpool.Pool }

func NewSettings(pool *pgxpool.Pool) *Settings { return &Settings{pool: pool} }

func (r *Settings) Get(ctx context.Context, key string, dst any) (bool, error) {
	var raw []byte
	err := pg.DB(ctx, r.pool).QueryRow(ctx, `SELECT value FROM settings WHERE key = $1`, key).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("get setting %s: %w", key, err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		return false, fmt.Errorf("decode setting %s: %w", key, err)
	}
	return true, nil
}

func (r *Settings) Set(ctx context.Context, key string, value any, by string) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode setting %s: %w", key, err)
	}
	_, err = pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO settings (key, value, updated_by) VALUES ($1, $2, $3)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = NOW()`,
		key, raw, by)
	if err != nil {
		return fmt.Errorf("set setting %s: %w", key, err)
	}
	return nil
}
