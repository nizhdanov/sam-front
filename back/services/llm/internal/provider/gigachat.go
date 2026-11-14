package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type GigaChatConfig struct {
	AuthURL string
	BaseURL string
	AuthKey string
	Scope   string
}

type GigaChat struct {
	cfg    GigaChatConfig
	client *http.Client

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

func NewGigaChat(cfg GigaChatConfig, client *http.Client) *GigaChat {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &GigaChat{cfg: cfg, client: client}
}

func (p *GigaChat) Name() string { return "gigachat" }

func (p *GigaChat) accessToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && time.Until(p.expiresAt) > time.Minute {
		return p.token, nil
	}
	if p.cfg.AuthKey == "" {
		return "", ErrNotConfigured
	}

	form := url.Values{"scope": {p.cfg.Scope}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.AuthURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Basic "+p.cfg.AuthKey)
	req.Header.Set("RqUID", uuid.NewString())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request gigachat token: %w", err)
	}
	if err := checkStatus(resp); err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode gigachat token: %w", err)
	}
	p.token = body.AccessToken
	p.expiresAt = time.UnixMilli(body.ExpiresAt)
	return p.token, nil
}

func (p *GigaChat) invalidate() {
	p.mu.Lock()
	p.token = ""
	p.mu.Unlock()
}

type gigaMessage struct {
	Role         string            `json:"role"`
	Content      string            `json:"content"`
	Name         string            `json:"name,omitempty"`
	FunctionCall *gigaFunctionCall `json:"function_call,omitempty"`
	StateID      string            `json:"functions_state_id,omitempty"`
}

type gigaFunctionCall struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type gigaFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type gigaRequest struct {
	Model        string         `json:"model"`
	Messages     []gigaMessage  `json:"messages"`
	Functions    []gigaFunction `json:"functions,omitempty"`
	FunctionCall string         `json:"function_call,omitempty"`
	MaxTokens    int            `json:"max_tokens,omitempty"`
	Temperature  float64        `json:"temperature,omitempty"`
	Stream       bool           `json:"stream,omitempty"`
}

type gigaResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message gigaMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (p *GigaChat) build(req Request, stream bool) gigaRequest {
	out := gigaRequest{Model: req.Model, MaxTokens: req.MaxTokens, Stream: stream}
	if req.Temperature > 0 {
		out.Temperature = req.Temperature
	}
	for _, m := range req.Messages {
		gm := gigaMessage{Role: string(m.Role), Content: m.Content}
		switch m.Role {
		case RoleTool:
			gm.Role, gm.Name = "function", m.Name
		case RoleAssistant:
			if len(m.ToolCalls) > 0 {
				tc := m.ToolCalls[0]
				gm.FunctionCall = &gigaFunctionCall{Name: tc.Name, Arguments: nonEmptyJSON(tc.Arguments)}
				gm.StateID = tc.ID
			}
		}
		out.Messages = append(out.Messages, gm)
	}
	for _, t := range req.Tools {
		out.Functions = append(out.Functions, gigaFunction{Name: t.Name, Description: t.Description, Parameters: t.Parameters})
	}
	if len(out.Functions) > 0 {
		out.FunctionCall = "auto"
	}
	return out
}

func (p *GigaChat) post(ctx context.Context, body gigaRequest) (*http.Response, error) {
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	for attempt := 0; attempt < 2; attempt++ {
		token, err := p.accessToken(ctx)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/chat/completions", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := p.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("call gigachat: %w", err)
		}
		if resp.StatusCode == http.StatusUnauthorized && attempt == 0 {
			_ = resp.Body.Close()
			p.invalidate()
			continue
		}
		return resp, checkStatus(resp)
	}
	return nil, fmt.Errorf("gigachat rejected the access token")
}

func (p *GigaChat) Complete(ctx context.Context, req Request) (Response, error) {
	resp, err := p.post(ctx, p.build(req, false))
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out gigaResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("decode gigachat response: %w", err)
	}
	if len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("gigachat returned no choices")
	}
	msg := out.Choices[0].Message
	result := Response{
		Content: msg.Content,
		Model:   out.Model,
		Usage:   Usage{PromptTokens: out.Usage.PromptTokens, CompletionTokens: out.Usage.CompletionTokens},
	}
	if msg.FunctionCall != nil {
		id := msg.StateID
		if id == "" {
			id = uuid.NewString()
		}
		result.ToolCalls = []ToolCall{{ID: id, Name: msg.FunctionCall.Name, Arguments: nonEmptyJSON(msg.FunctionCall.Arguments)}}
	}
	return result, nil
}

func (p *GigaChat) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	resp, err := p.post(ctx, p.build(req, true))
	if err != nil {
		return nil, err
	}
	return streamSSE(ctx, resp), nil
}

func nonEmptyJSON(raw json.RawMessage) json.RawMessage {
	if len(bytes.TrimSpace(raw)) == 0 {
		return json.RawMessage(`{}`)
	}
	if raw[0] == '"' {
		var s string
		if json.Unmarshal(raw, &s) == nil && json.Valid([]byte(s)) {
			return json.RawMessage(s)
		}
	}
	return raw
}
