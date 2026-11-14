package domain

import (
	"time"

	"github.com/google/uuid"
)

type MessageRole string

const (
	MessageUser      MessageRole = "user"
	MessageAssistant MessageRole = "assistant"
)

type MessageStatus string

const (
	MessageCompleted  MessageStatus = "COMPLETED"
	MessageIncomplete MessageStatus = "INCOMPLETE"
	MessageFailed     MessageStatus = "FAILED"
)

type Conversation struct {
	ID        uuid.UUID `json:"id"`
	UserID    string    `json:"-"`
	UserRole  string    `json:"-"`
	Title     *string   `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID             uuid.UUID     `json:"id"`
	ConversationID uuid.UUID     `json:"conversation_id"`
	Role           MessageRole   `json:"role"`
	Content        string        `json:"content"`
	Status         MessageStatus `json:"status"`
	CreatedAt      time.Time     `json:"created_at"`
}
