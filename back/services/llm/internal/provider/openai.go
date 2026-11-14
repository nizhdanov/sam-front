package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type OpenAIConfig struct {
	BaseURL string
	APIKey  string
}

type OpenAI struct {
	cfg    OpenAIConfig
	client *http.Client
}

func NewOpenAI(cfg OpenAIConfig, client *http.Client) *OpenAI {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &OpenAI{cfg: cfg, client: client}
}

func (p *OpenAI) Name() string { return "openai_compatible" }

type openAIMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAITool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string          `json:"name"`
		Description string          `json:"description"`
		Parameters  json.RawMessage `json:"parameters"`
	} `json:"function"`
}

type openAIRequest struct {
	Model          string            `json:"model"`
	Messages       []openAIMessage   `json:"messages"`
	Tools          []openAITool      `json:"tools,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	Temperature    float64           `json:"temperature"`
	Stream         bool              `json:"stream,omitempty"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type openAIResponse struct {
	Model   string `json:"model"`
	Choices []struct {
		Message openAIMessage `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (p *OpenAI) build(req Request, stream bool) openAIRequest {
	out := openAIRequest{Model: req.Model, MaxTokens: req.MaxTokens, Temperature: req.Temperature, Stream: stream}
	if req.JSON {
		out.ResponseFormat = map[string]string{"type": "json_object"}
	}
	for _, m := range req.Messages {
		om := openAIMessage{Role: string(m.Role), Content: m.Content, ToolCallID: m.ToolCallID}
		for _, tc := range m.ToolCalls {
			call := openAIToolCall{ID: tc.ID, Type: "function"}
			call.Function.Name = tc.Name
			call.Function.Arguments = string(tc.Arguments)
			om.ToolCalls = append(om.ToolCalls, call)
		}
		out.Messages = append(out.Messages, om)
	}
	for _, t := range req.Tools {
		tool := openAITool{Type: "function"}
		tool.Function.Name, tool.Function.Description, tool.Function.Parameters = t.Name, t.Description, t.Parameters
		out.Tools = append(out.Tools, tool)
	}
	return out
}

func (p *OpenAI) post(ctx context.Context, body openAIRequest) (*http.Response, error) {
	if p.cfg.BaseURL == "" {
		return nil, ErrNotConfigured
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.cfg.BaseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if p.cfg.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.cfg.APIKey)
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call model provider: %w", err)
	}
	return resp, checkStatus(resp)
}

func (p *OpenAI) Complete(ctx context.Context, req Request) (Response, error) {
	resp, err := p.post(ctx, p.build(req, false))
	if err != nil {
		return Response{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var out openAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return Response{}, fmt.Errorf("decode model response: %w", err)
	}
	if len(out.Choices) == 0 {
		return Response{}, fmt.Errorf("model returned no choices")
	}
	msg := out.Choices[0].Message
	result := Response{
		Content: msg.Content,
		Model:   out.Model,
		Usage:   Usage{PromptTokens: out.Usage.PromptTokens, CompletionTokens: out.Usage.CompletionTokens},
	}
	for _, tc := range msg.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{ID: tc.ID, Name: tc.Function.Name, Arguments: json.RawMessage(tc.Function.Arguments)})
	}
	return result, nil
}

func (p *OpenAI) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	resp, err := p.post(ctx, p.build(req, true))
	if err != nil {
		return nil, err
	}
	return streamSSE(ctx, resp), nil
}
