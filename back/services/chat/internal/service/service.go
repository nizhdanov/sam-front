package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/pkg/httpx"
	"sam/services/chat/internal/domain"
)

type Conversations interface {
	Create(ctx context.Context, c domain.Conversation) error
	Get(ctx context.Context, id uuid.UUID) (domain.Conversation, error)
	List(ctx context.Context, userID string, page httpx.PageRequest) ([]domain.Conversation, int, error)
	Touch(ctx context.Context, id uuid.UUID, title *string) error
	Delete(ctx context.Context, id uuid.UUID) error
}

type Messages interface {
	Create(ctx context.Context, m domain.Message) (domain.Message, error)
	List(ctx context.Context, conversationID uuid.UUID, page httpx.PageRequest) ([]domain.Message, int, error)
	Recent(ctx context.Context, conversationID uuid.UUID, limit int) ([]domain.Message, error)
}

type Assistant interface {
	Chat(ctx context.Context, conversationID string, history []domain.Message, message string, onDelta func(string) error) error
}

type Config struct {
	MaxHistoryMessages int
	MaxMessageLength   int
}

const turnWait = 3 * time.Minute

type Service struct {
	conversations Conversations
	messages      Messages
	assistant     Assistant
	cfg           Config
	log           *slog.Logger

	mu    sync.Mutex
	turns map[uuid.UUID]chan struct{}
}

func New(conversations Conversations, messages Messages, assistant Assistant, cfg Config, log *slog.Logger) *Service {
	return &Service{
		conversations: conversations,
		messages:      messages,
		assistant:     assistant,
		cfg:           cfg,
		log:           log,
		turns:         make(map[uuid.UUID]chan struct{}),
	}
}

func (s *Service) CreateConversation(ctx context.Context, title string) (domain.Conversation, error) {
	claims, err := auth.Require(ctx, auth.UserRoles...)
	if err != nil {
		return domain.Conversation{}, err
	}
	c := domain.Conversation{ID: uuid.New(), UserID: claims.Subject, UserRole: string(claims.Role)}
	if title = strings.TrimSpace(title); title != "" {
		c.Title = &title
	}
	if err := s.conversations.Create(ctx, c); err != nil {
		return domain.Conversation{}, err
	}
	return s.conversations.Get(ctx, c.ID)
}

func (s *Service) ListConversations(ctx context.Context, page httpx.PageRequest) ([]domain.Conversation, int, error) {
	claims, err := auth.Require(ctx, auth.UserRoles...)
	if err != nil {
		return nil, 0, err
	}
	return s.conversations.List(ctx, claims.Subject, page)
}

func (s *Service) Conversation(ctx context.Context, id uuid.UUID) (domain.Conversation, error) {
	claims, err := auth.Require(ctx, auth.UserRoles...)
	if err != nil {
		return domain.Conversation{}, err
	}
	c, err := s.conversations.Get(ctx, id)
	if err != nil {
		return domain.Conversation{}, err
	}
	if c.UserID != claims.Subject {
		return domain.Conversation{}, apperr.NotFound("conversation not found")
	}
	if c.UserRole != string(claims.Role) {
		return domain.Conversation{}, apperr.Forbidden("conversation was created under a different access role")
	}
	return c, nil
}

func (s *Service) DeleteConversation(ctx context.Context, id uuid.UUID) error {
	if _, err := s.Conversation(ctx, id); err != nil {
		return err
	}
	return s.conversations.Delete(ctx, id)
}

func (s *Service) Messages(ctx context.Context, id uuid.UUID, page httpx.PageRequest) ([]domain.Message, int, error) {
	if _, err := s.Conversation(ctx, id); err != nil {
		return nil, 0, err
	}
	return s.messages.List(ctx, id, page)
}

type Stream interface {
	Start(messageID uuid.UUID) error
	Delta(text string) error
}

func (s *Service) Send(ctx context.Context, id uuid.UUID, content string, stream Stream) (domain.Message, error) {
	content = strings.TrimSpace(content)
	switch {
	case content == "":
		return domain.Message{}, apperr.Invalid("message is empty")
	case utf8.RuneCountInString(content) > s.cfg.MaxMessageLength:
		return domain.Message{}, apperr.Invalid("message is too long")
	}
	c, err := s.Conversation(ctx, id)
	if err != nil {
		return domain.Message{}, err
	}
	release, err := s.waitTurn(ctx, c.ID)
	if err != nil {
		return domain.Message{}, err
	}
	defer release()

	history, err := s.messages.Recent(ctx, id, s.cfg.MaxHistoryMessages)
	if err != nil {
		return domain.Message{}, err
	}
	if _, err := s.messages.Create(ctx, domain.Message{
		ID: uuid.New(), ConversationID: id, Role: domain.MessageUser, Content: content, Status: domain.MessageCompleted,
	}); err != nil {
		return domain.Message{}, err
	}

	reply := domain.Message{ID: uuid.New(), ConversationID: id, Role: domain.MessageAssistant, Status: domain.MessageCompleted}
	if err := stream.Start(reply.ID); err != nil {
		return domain.Message{}, err
	}
	var answer strings.Builder
	chatErr := s.assistant.Chat(ctx, id.String(), history, content, func(delta string) error {
		answer.WriteString(delta)
		return stream.Delta(delta)
	})

	reply.Content = answer.String()
	if chatErr != nil {
		reply.Status = domain.MessageFailed
		if reply.Content != "" {
			reply.Status = domain.MessageIncomplete
		}
		if errors.Is(chatErr, context.Canceled) {
			s.log.InfoContext(ctx, "chat stream cancelled by client", "conversation_id", id)
		}
	}
	saveCtx := context.WithoutCancel(ctx)
	if reply.Content != "" {
		if _, err := s.messages.Create(saveCtx, reply); err != nil {
			s.log.ErrorContext(ctx, "save assistant message", "conversation_id", id, "err", err)
		}
	}
	if err := s.conversations.Touch(saveCtx, id, titleFrom(content)); err != nil {
		s.log.ErrorContext(ctx, "touch conversation", "conversation_id", id, "err", err)
	}
	return reply, chatErr
}

func (s *Service) waitTurn(ctx context.Context, conversationID uuid.UUID) (func(), error) {
	s.mu.Lock()
	turn, ok := s.turns[conversationID]
	if !ok {
		turn = make(chan struct{}, 1)
		s.turns[conversationID] = turn
	}
	s.mu.Unlock()

	timer := time.NewTimer(turnWait)
	defer timer.Stop()
	select {
	case turn <- struct{}{}:
		return func() { <-turn }, nil
	case <-timer.C:
		return nil, apperr.RateLimited("previous message in this conversation is still being answered")
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func titleFrom(content string) *string {
	runes := []rune(strings.Join(strings.Fields(content), " "))
	if len(runes) > 60 {
		runes = append(runes[:57], []rune("...")...)
	}
	title := string(runes)
	return &title
}
