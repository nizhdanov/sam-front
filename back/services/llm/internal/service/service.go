package service

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"sam/pkg/apperr"
	"sam/services/llm/internal/provider"
	"sam/services/llm/internal/tools"
)

type Models struct {
	Chat     string
	Matching string
	EULA     string
}

type Config struct {
	Models          Models
	MaxConcurrent   int
	MaxToolCalls    int
	MaxOutputTokens int
	MaxContextChars int
	DailyTokenLimit int
}

type Service struct {
	provider provider.ModelProvider
	tools    *tools.Executor
	cfg      Config
	log      *slog.Logger
	slots    chan struct{}
	budget   *budget
}

func New(p provider.ModelProvider, executor *tools.Executor, cfg Config, log *slog.Logger) *Service {
	return &Service{
		provider: p,
		tools:    executor,
		cfg:      cfg,
		log:      log,
		slots:    make(chan struct{}, max(1, cfg.MaxConcurrent)),
		budget:   &budget{limit: cfg.DailyTokenLimit},
	}
}

func (s *Service) acquire(ctx context.Context) (func(), error) {
	if !s.budget.available() {
		return nil, apperr.RateLimited("daily AI token budget is exhausted")
	}
	select {
	case s.slots <- struct{}{}:
		return func() { <-s.slots }, nil
	case <-ctx.Done():
		return nil, apperr.Wrap(ctx.Err(), apperr.KindTimeout, "AI service is busy")
	}
}

func (s *Service) complete(ctx context.Context, req provider.Request) (provider.Response, error) {
	start := time.Now()
	resp, err := s.provider.Complete(ctx, req)
	s.record(ctx, req.Task, req.Model, start, resp.Usage, err)
	if err != nil {
		return resp, providerError(err)
	}
	return resp, nil
}

func (s *Service) record(ctx context.Context, task, model string, start time.Time, usage provider.Usage, err error) {
	s.budget.spend(usage.Total())
	status := "ok"
	if err != nil {
		status = "error"
	}
	s.log.InfoContext(ctx, "llm request",
		"task_type", task,
		"provider", s.provider.Name(),
		"model", model,
		"latency_ms", time.Since(start).Milliseconds(),
		"prompt_tokens", usage.PromptTokens,
		"completion_tokens", usage.CompletionTokens,
		"total_tokens", usage.Total(),
		"result_status", status,
	)
	if err != nil {
		s.log.WarnContext(ctx, "llm provider error", "task_type", task, "err", err)
	}
}

func providerError(err error) error {
	switch {
	case errors.Is(err, provider.ErrNotConfigured):
		return apperr.Unavailable("AI model is not configured")
	case errors.Is(err, context.DeadlineExceeded):
		return apperr.Wrap(err, apperr.KindTimeout, "AI model did not respond in time")
	}
	var httpErr *provider.HTTPError
	if errors.As(err, &httpErr) && httpErr.Status == 429 {
		return apperr.Wrap(err, apperr.KindRateLimited, "AI model rate limit exceeded")
	}
	return apperr.Wrap(err, apperr.KindUnavailable, "AI model is unavailable")
}

func extractJSON(content string, dst any) error {
	start := strings.IndexByte(content, '{')
	end := strings.LastIndexByte(content, '}')
	if start < 0 || end <= start {
		return apperr.Unavailable("AI model returned no JSON object")
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), dst); err != nil {
		return apperr.Wrap(err, apperr.KindUnavailable, "AI model returned malformed JSON")
	}
	return nil
}

type budget struct {
	mu    sync.Mutex
	limit int
	day   string
	used  int
}

func (b *budget) roll() {
	today := time.Now().UTC().Format(time.DateOnly)
	if b.day != today {
		b.day, b.used = today, 0
	}
}

func (b *budget) available() bool {
	if b.limit <= 0 {
		return true
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roll()
	return b.used < b.limit
}

func (b *budget) spend(tokens int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.roll()
	b.used += tokens
}
