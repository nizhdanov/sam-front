package service

import (
	"context"
	"strings"
	"time"
	"unicode/utf8"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/services/llm/internal/prompts"
	"sam/services/llm/internal/provider"
	"sam/services/llm/internal/tools"
)

type ChatMessage struct {
	Role    string
	Content string
}

type ChatOutcome struct {
	Model string
	Usage provider.Usage
}

type Emit func(delta string) error

const chunkSize = 48

func (s *Service) Chat(ctx context.Context, history []ChatMessage, message string, emit Emit) (ChatOutcome, error) {
	claims, err := auth.Require(ctx, auth.UserRoles...)
	if err != nil {
		return ChatOutcome{}, err
	}
	if strings.TrimSpace(message) == "" {
		return ChatOutcome{}, apperr.Invalid("message is empty")
	}
	release, err := s.acquire(ctx)
	if err != nil {
		return ChatOutcome{}, err
	}
	defer release()

	messages := []provider.Message{{Role: provider.RoleSystem, Content: prompts.Chat(claims.Role)}}
	messages = append(messages, s.trimHistory(history)...)
	messages = append(messages, provider.Message{Role: provider.RoleUser, Content: message})
	available := tools.ForRole(claims.Role)

	var outcome ChatOutcome
	for range s.cfg.MaxToolCalls {
		resp, err := s.complete(ctx, provider.Request{
			Task:      "chat",
			Model:     s.cfg.Models.Chat,
			Messages:  messages,
			Tools:     available,
			MaxTokens: s.cfg.MaxOutputTokens,
		})
		if err != nil {
			return outcome, err
		}
		outcome.Model = resp.Model
		outcome.Usage = addUsage(outcome.Usage, resp.Usage)

		if len(resp.ToolCalls) == 0 {
			return outcome, emitChunks(resp.Content, emit)
		}
		messages = append(messages, provider.Message{Role: provider.RoleAssistant, Content: resp.Content, ToolCalls: resp.ToolCalls})
		for _, call := range resp.ToolCalls {
			result := s.tools.Run(ctx, claims.Role, call)
			s.log.DebugContext(ctx, "tool executed", "tool", call.Name, "result_bytes", len(result))
			messages = append(messages, provider.Message{Role: provider.RoleTool, Name: call.Name, ToolCallID: call.ID, Content: result})
		}
	}

	messages = append(messages, provider.Message{
		Role:    provider.RoleSystem,
		Content: "Лимит обращений к инструментам исчерпан. Ответь пользователю на основе уже полученных данных.",
	})
	return s.streamFinal(ctx, messages, outcome, emit)
}

func (s *Service) streamFinal(ctx context.Context, messages []provider.Message, outcome ChatOutcome, emit Emit) (ChatOutcome, error) {
	start := time.Now()
	events, err := s.provider.Stream(ctx, provider.Request{
		Task:      "chat",
		Model:     s.cfg.Models.Chat,
		Messages:  messages,
		MaxTokens: s.cfg.MaxOutputTokens,
	})
	if err != nil {
		s.record(ctx, "chat", s.cfg.Models.Chat, start, provider.Usage{}, err)
		return outcome, providerError(err)
	}
	var usage provider.Usage
	for ev := range events {
		if ev.Err != nil {
			s.record(ctx, "chat", s.cfg.Models.Chat, start, usage, ev.Err)
			return outcome, providerError(ev.Err)
		}
		usage = addUsage(usage, ev.Usage)
		if ev.Delta != "" {
			if err := emit(ev.Delta); err != nil {
				return outcome, err
			}
		}
	}
	s.record(ctx, "chat", s.cfg.Models.Chat, start, usage, nil)
	outcome.Usage = addUsage(outcome.Usage, usage)
	return outcome, nil
}

func (s *Service) trimHistory(history []ChatMessage) []provider.Message {
	budget := s.cfg.MaxContextChars
	var out []provider.Message
	for i := len(history) - 1; i >= 0; i-- {
		h := history[i]
		role := provider.RoleUser
		if h.Role == "assistant" {
			role = provider.RoleAssistant
		}
		budget -= utf8.RuneCountInString(h.Content)
		if budget < 0 {
			break
		}
		out = append([]provider.Message{{Role: role, Content: h.Content}}, out...)
	}
	return out
}

func emitChunks(content string, emit Emit) error {
	runes := []rune(content)
	for start := 0; start < len(runes); start += chunkSize {
		if err := emit(string(runes[start:min(start+chunkSize, len(runes))])); err != nil {
			return err
		}
	}
	return nil
}

func addUsage(a, b provider.Usage) provider.Usage {
	return provider.Usage{PromptTokens: a.PromptTokens + b.PromptTokens, CompletionTokens: a.CompletionTokens + b.CompletionTokens}
}
