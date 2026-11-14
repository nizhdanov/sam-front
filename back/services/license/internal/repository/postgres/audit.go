package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Audit struct{ pool *pgxpool.Pool }

func NewAudit(pool *pgxpool.Pool) *Audit { return &Audit{pool: pool} }

func (r *Audit) Record(ctx context.Context, e domain.AuditEntry) error {
	changes, err := json.Marshal(e.Changes)
	if err != nil {
		return fmt.Errorf("encode audit changes: %w", err)
	}
	if e.Changes == nil {
		changes = []byte("{}")
	}
	_, err = pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO audit_log (actor, entity, entity_id, action, summary, changes) VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6)`,
		e.Actor, e.Entity, e.EntityID, e.Action, e.Summary, changes)
	if err != nil {
		return fmt.Errorf("record audit: %w", err)
	}
	return nil
}

func (r *Audit) List(ctx context.Context, f domain.AuditFilter, page httpx.PageRequest) ([]domain.AuditEntry, int, error) {
	var w where
	if f.Actor != "" {
		w.add("actor = ?", f.Actor)
	}
	if f.Entity != "" {
		w.add("entity = ?", f.Entity)
	}
	if f.From != nil {
		w.add("created_at >= ?", *f.From)
	}
	if f.To != nil {
		w.add("created_at < ?", *f.To)
	}
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_log`+w.String(), w.args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count audit: %w", err)
	}
	query := `SELECT id, created_at, actor, entity, COALESCE(entity_id, ''), action, summary, changes FROM audit_log` +
		w.String() + ` ORDER BY created_at DESC, id DESC LIMIT ` + w.arg(page.Limit) + ` OFFSET ` + w.arg(page.Offset())
	rows, err := db.Query(ctx, query, w.args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list audit: %w", err)
	}
	items, err := collect(rows, func(row scanner) (domain.AuditEntry, error) {
		var e domain.AuditEntry
		var changes []byte
		if err := row.Scan(&e.ID, &e.CreatedAt, &e.Actor, &e.Entity, &e.EntityID, &e.Action, &e.Summary, &changes); err != nil {
			return e, err
		}
		if len(changes) > 2 {
			if err := json.Unmarshal(changes, &e.Changes); err != nil {
				return e, err
			}
		}
		return e, nil
	})
	return items, total, err
}
