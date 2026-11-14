package service

import (
	"context"
	"encoding/json"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/services/llm/internal/prompts"
	"sam/services/llm/internal/provider"
)

const maxAgreementChars = 60_000

type Analysis struct {
	JSON          []byte
	Model         string
	PromptVersion string
}

func (s *Service) AnalyzeAgreement(ctx context.Context, text string) (Analysis, error) {
	if _, err := auth.Require(ctx, auth.RoleAdmin, auth.RoleLicenseService); err != nil {
		return Analysis{}, err
	}
	if text == "" {
		return Analysis{}, apperr.Invalid("agreement text is empty")
	}
	runes := []rune(text)
	if len(runes) > maxAgreementChars {
		text = string(runes[:maxAgreementChars])
	}
	release, err := s.acquire(ctx)
	if err != nil {
		return Analysis{}, err
	}
	defer release()

	resp, err := s.complete(ctx, provider.Request{
		Task:      "eula",
		Model:     s.cfg.Models.EULA,
		MaxTokens: s.cfg.MaxOutputTokens,
		JSON:      true,
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: prompts.EULA()},
			{Role: provider.RoleUser, Content: "<agreement>\n" + text + "\n</agreement>"},
		},
	})
	if err != nil {
		return Analysis{}, err
	}
	var parsed map[string]any
	if err := extractJSON(resp.Content, &parsed); err != nil {
		return Analysis{}, err
	}
	body, err := json.Marshal(parsed)
	if err != nil {
		return Analysis{}, err
	}
	return Analysis{JSON: body, Model: resp.Model, PromptVersion: prompts.EULAVersion}, nil
}
