package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type Matching struct{ pool *pgxpool.Pool }

func NewMatching(pool *pgxpool.Pool) *Matching { return &Matching{pool: pool} }

const matchingSelect = `
	SELECT m.id, m.source_key, m.raw_name, m.raw_publisher, m.raw_version, m.candidate_ids, m.suggested_product,
	       m.match_method, m.confidence_score::float8, m.reason, m.status,
	       (SELECT COUNT(DISTINCT device_id) FROM installations i WHERE i.source_key = m.source_key AND i.is_active),
	       m.reviewed_by, m.reviewed_at, m.created_at
	FROM matching_candidates m`

func scanTask(row scanner) (domain.MatchingTask, error) {
	var t domain.MatchingTask
	var ids []byte
	err := row.Scan(&t.ID, &t.SourceKey, &t.RawName, &t.RawPublisher, &t.RawVersion, &ids, &t.SuggestedProduct,
		&t.MatchMethod, &t.Confidence, &t.Reason, &t.Status, &t.AffectedDevices, &t.ReviewedBy, &t.ReviewedAt, &t.CreatedAt)
	if err != nil {
		return t, err
	}
	return t, json.Unmarshal(ids, &t.CandidateIDs)
}

func (r *Matching) StatusByKeys(ctx context.Context, keys []string) (map[string]domain.ReviewStatus, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `SELECT source_key, status FROM matching_candidates WHERE source_key = ANY($1)`, keys)
	if err != nil {
		return nil, fmt.Errorf("matching statuses: %w", err)
	}
	type row struct {
		key    string
		status domain.ReviewStatus
	}
	list, err := collect(rows, func(s scanner) (row, error) {
		var r row
		err := s.Scan(&r.key, &r.status)
		return r, err
	})
	if err != nil {
		return nil, err
	}
	out := make(map[string]domain.ReviewStatus, len(list))
	for _, r := range list {
		out[r.key] = r.status
	}
	return out, nil
}

func (r *Matching) UpsertPending(ctx context.Context, s domain.Suggestion) error {
	ids := make([]uuid.UUID, 0, len(s.Candidates))
	for _, c := range s.Candidates {
		ids = append(ids, c.ProductID)
	}
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return err
	}
	var method *domain.MatchMethod
	if s.Method != "" {
		method = &s.Method
	}
	var confidence *float64
	if s.Confidence > 0 {
		confidence = &s.Confidence
	}
	_, err = pg.DB(ctx, r.pool).Exec(ctx, `
		INSERT INTO matching_candidates (id, source_key, raw_name, raw_publisher, raw_version, candidate_ids,
		                                 suggested_product, match_method, confidence_score, reason)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (source_key) DO UPDATE SET
			candidate_ids = EXCLUDED.candidate_ids,
			suggested_product = EXCLUDED.suggested_product,
			match_method = EXCLUDED.match_method,
			confidence_score = EXCLUDED.confidence_score,
			reason = EXCLUDED.reason,
			updated_at = NOW()
		WHERE matching_candidates.status = 'PENDING'`,
		uuid.New(), s.Software.Key(), s.Software.DisplayName, nonEmpty(s.Software.Publisher), nonEmpty(s.Software.Version),
		idsJSON, s.ProductID, method, confidence, nonEmpty(s.Reason))
	if err != nil {
		return fmt.Errorf("save matching candidate: %w", err)
	}
	return nil
}

func (r *Matching) List(ctx context.Context, status domain.ReviewStatus, page httpx.PageRequest) ([]domain.MatchingTask, int, error) {
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM matching_candidates WHERE status = $1`, status).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count matching tasks: %w", err)
	}
	rows, err := db.Query(ctx, matchingSelect+` WHERE m.status = $1 ORDER BY m.created_at LIMIT $2 OFFSET $3`,
		status, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list matching tasks: %w", err)
	}
	items, err := collect(rows, scanTask)
	return items, total, err
}

func (r *Matching) Get(ctx context.Context, id uuid.UUID, lock bool) (domain.MatchingTask, error) {
	query := matchingSelect + ` WHERE m.id = $1`
	if lock {
		query += ` FOR UPDATE OF m`
	}
	rows, err := pg.DB(ctx, r.pool).Query(ctx, query, id)
	return one(rows, err, scanTask, "matching task")
}

func (r *Matching) Approve(ctx context.Context, key string, productID uuid.UUID, reviewer *string) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		UPDATE matching_candidates SET status = 'APPROVED', suggested_product = $2, reviewed_by = $3,
		       reviewed_at = CASE WHEN $3::text IS NULL THEN NULL ELSE NOW() END, updated_at = NOW()
		WHERE source_key = $1 AND status = 'PENDING'`, key, productID, reviewer)
	if err != nil {
		return fmt.Errorf("approve matching task: %w", err)
	}
	return nil
}

func (r *Matching) Reject(ctx context.Context, id uuid.UUID, reviewer, reason string) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `
		UPDATE matching_candidates SET status = 'REJECTED', reviewed_by = $2, reviewed_at = NOW(), updated_at = NOW(),
		       reason = COALESCE(NULLIF($3, ''), reason)
		WHERE id = $1`, id, reviewer, reason)
	if err != nil {
		return fmt.Errorf("reject matching task: %w", err)
	}
	return nil
}
