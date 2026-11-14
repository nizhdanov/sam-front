package postgres

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	pg "sam/pkg/postgres"
)

type Locks struct{ pool *pgxpool.Pool }

func NewLocks(pool *pgxpool.Pool) *Locks { return &Locks{pool: pool} }

func (l *Locks) Products(ctx context.Context, ids []uuid.UUID) error {
	keys := make([]string, 0, len(ids))
	for _, id := range ids {
		keys = append(keys, "product:"+id.String())
	}
	return l.lock(ctx, keys)
}

func (l *Locks) All(ctx context.Context) error {
	return l.lock(ctx, []string{"compliance:all"})
}

func (l *Locks) lock(ctx context.Context, keys []string) error {
	slices.Sort(keys)
	keys = slices.Compact(keys)
	db := pg.DB(ctx, l.pool)
	for _, key := range keys {
		if _, err := db.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, key); err != nil {
			return fmt.Errorf("advisory lock %s: %w", key, err)
		}
	}
	return nil
}

func notFound(err error, what string) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return apperr.NotFound(what + " not found")
	}
	return fmt.Errorf("%s: %w", what, err)
}

type scanner interface{ Scan(dest ...any) error }

func collect[T any](rows pgx.Rows, scan func(scanner) (T, error)) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func one[T any](rows pgx.Rows, err error, scan func(scanner) (T, error), what string) (T, error) {
	var zero T
	if err != nil {
		return zero, fmt.Errorf("%s: %w", what, err)
	}
	items, err := collect(rows, scan)
	if err != nil {
		return zero, fmt.Errorf("%s: %w", what, err)
	}
	if len(items) == 0 {
		return zero, apperr.NotFound(what + " not found")
	}
	return items[0], nil
}

type where struct {
	clauses []string
	args    []any
}

func (w *where) add(clause string, value any) {
	w.args = append(w.args, value)
	w.clauses = append(w.clauses, strings.ReplaceAll(clause, "?", "$"+strconv.Itoa(len(w.args))))
}

func (w *where) raw(clause string) { w.clauses = append(w.clauses, clause) }

func (w *where) arg(value any) string {
	w.args = append(w.args, value)
	return "$" + strconv.Itoa(len(w.args))
}

func (w *where) String() string {
	if len(w.clauses) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(w.clauses, " AND ")
}

type set struct {
	columns []string
	args    []any
}

func (s *set) add(column string, value any) {
	s.args = append(s.args, value)
	s.columns = append(s.columns, column+" = $"+strconv.Itoa(len(s.args)))
}

func (s *set) raw(expr string) { s.columns = append(s.columns, expr) }

func (s *set) empty() bool { return len(s.columns) == 0 }

func (s *set) update(table string, id uuid.UUID) (string, []any) {
	args := append(s.args, id)
	return fmt.Sprintf(`UPDATE %s SET %s, updated_at = NOW() WHERE id = $%d`, table, strings.Join(s.columns, ", "), len(args)), args
}

func uuids(ids []uuid.UUID) []uuid.UUID {
	if ids == nil {
		return []uuid.UUID{}
	}
	return ids
}
