package provider

import (
	"context"
	"encoding/json"
	"errors"
)

var ErrNotConfigured = errors.New("model provider is not configured")

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type Message struct {
	Role       Role
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
	Name       string
}

type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type Request struct {
	Task        string
	Model       string
	Messages    []Message
	Tools       []Tool
	MaxTokens   int
	Temperature float64
	JSON        bool
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
}

func (u Usage) Total() int { return u.PromptTokens + u.CompletionTokens }

type Response struct {
	Content   string
	ToolCalls []ToolCall
	Usage     Usage
	Model     string
}

type StreamEvent struct {
	Delta string
	Usage Usage
	Err   error
}

type ModelProvider interface {
	Name() string
	Complete(ctx context.Context, req Request) (Response, error)
	Stream(ctx context.Context, req Request) (<-chan StreamEvent, error)
}
