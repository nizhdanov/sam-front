package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	pg "sam/pkg/postgres"
	"sam/services/chat/internal/domain"
)

type Conversations struct{ pool *pgxpool.Pool }

func NewConversations(pool *pgxpool.Pool) *Conversations { return &Conversations{pool: pool} }

const conversationSelect = `SELECT id, user_id, user_role, title, created_at, updated_at FROM conversations`

func scanConversation(row pgx.CollectableRow) (domain.Conversation, error) {
	var c domain.Conversation
	err := row.Scan(&c.ID, &c.UserID, &c.UserRole, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	return c, err
}

func (r *Conversations) Create(ctx context.Context, c domain.Conversation) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `INSERT INTO conversations (id, user_id, user_role, title) VALUES ($1, $2, $3, $4)`,
		c.ID, c.UserID, c.UserRole, c.Title)
	if err != nil {
		return fmt.Errorf("create conversation: %w", err)
	}
	return nil
}

func (r *Conversations) Get(ctx context.Context, id uuid.UUID) (domain.Conversation, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, conversationSelect+` WHERE id = $1`, id)
	if err != nil {
		return domain.Conversation{}, fmt.Errorf("get conversation: %w", err)
	}
	c, err := pgx.CollectExactlyOneRow(rows, scanConversation)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, apperr.NotFound("conversation not found")
	}
	return c, err
}

func (r *Conversations) List(ctx context.Context, userID string, page httpx.PageRequest) ([]domain.Conversation, int, error) {
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM conversations WHERE user_id = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count conversations: %w", err)
	}
	rows, err := db.Query(ctx, conversationSelect+` WHERE user_id = $1 ORDER BY updated_at DESC LIMIT $2 OFFSET $3`,
		userID, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list conversations: %w", err)
	}
	items, err := pgx.CollectRows(rows, scanConversation)
	return items, total, err
}

func (r *Conversations) Touch(ctx context.Context, id uuid.UUID, title *string) error {
	_, err := pg.DB(ctx, r.pool).Exec(ctx, `UPDATE conversations SET updated_at = NOW(), title = COALESCE(title, $2) WHERE id = $1`, id, title)
	if err != nil {
		return fmt.Errorf("touch conversation: %w", err)
	}
	return nil
}

func (r *Conversations) Delete(ctx context.Context, id uuid.UUID) error {
	if _, err := pg.DB(ctx, r.pool).Exec(ctx, `DELETE FROM conversations WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}

type Messages struct{ pool *pgxpool.Pool }

func NewMessages(pool *pgxpool.Pool) *Messages { return &Messages{pool: pool} }

func scanMessage(row pgx.CollectableRow) (domain.Message, error) {
	var m domain.Message
	err := row.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.Status, &m.CreatedAt)
	return m, err
}

func (r *Messages) Create(ctx context.Context, m domain.Message) (domain.Message, error) {
	err := pg.DB(ctx, r.pool).QueryRow(ctx, `
		INSERT INTO messages (id, conversation_id, role, content, status) VALUES ($1, $2, $3, $4, $5) RETURNING created_at`,
		m.ID, m.ConversationID, m.Role, m.Content, m.Status).Scan(&m.CreatedAt)
	if err != nil {
		return m, fmt.Errorf("create message: %w", err)
	}
	return m, nil
}

func (r *Messages) List(ctx context.Context, conversationID uuid.UUID, page httpx.PageRequest) ([]domain.Message, int, error) {
	db := pg.DB(ctx, r.pool)
	var total int
	if err := db.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE conversation_id = $1`, conversationID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count messages: %w", err)
	}
	rows, err := db.Query(ctx, `
		SELECT id, conversation_id, role, content, status, created_at FROM messages
		WHERE conversation_id = $1 ORDER BY created_at, id LIMIT $2 OFFSET $3`, conversationID, page.Limit, page.Offset())
	if err != nil {
		return nil, 0, fmt.Errorf("list messages: %w", err)
	}
	items, err := pgx.CollectRows(rows, scanMessage)
	return items, total, err
}

func (r *Messages) Recent(ctx context.Context, conversationID uuid.UUID, limit int) ([]domain.Message, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, `
		SELECT id, conversation_id, role, content, status, created_at FROM (
			SELECT * FROM messages WHERE conversation_id = $1 AND status = 'COMPLETED'
			ORDER BY created_at DESC, id DESC LIMIT $2
		) recent ORDER BY created_at, id`, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("recent messages: %w", err)
	}
	return pgx.CollectRows(rows, scanMessage)
}
