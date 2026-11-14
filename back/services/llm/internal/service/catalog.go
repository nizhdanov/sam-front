package service

import (
	"context"
	"encoding/json"
	"strings"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/services/llm/internal/prompts"
	"sam/services/llm/internal/provider"
)

const (
	maxNormalizeBatch   = 60
	catalogOutputTokens = 4000
)

type RawName struct {
	Key         string `json:"key"`
	DisplayName string `json:"display_name"`
	Publisher   string `json:"publisher"`
}

type Normalized struct {
	Key           string `json:"key"`
	CanonicalName string `json:"canonical_name"`
	Publisher     string `json:"publisher"`
	IsSoftware    bool   `json:"is_software"`
}

func (s *Service) NormalizeSoftware(ctx context.Context, items []RawName) ([]Normalized, string, error) {
	if _, err := auth.Require(ctx, auth.RoleLicenseService, auth.RoleAdmin); err != nil {
		return nil, "", err
	}
	if len(items) == 0 || len(items) > maxNormalizeBatch {
		return nil, "", apperr.Invalid("batch must contain 1..60 items")
	}
	release, err := s.acquire(ctx)
	if err != nil {
		return nil, "", err
	}
	defer release()

	payload, err := json.Marshal(items)
	if err != nil {
		return nil, "", err
	}
	resp, err := s.complete(ctx, provider.Request{
		Task:      "catalog",
		Model:     s.cfg.Models.Matching,
		MaxTokens: max(s.cfg.MaxOutputTokens, catalogOutputTokens),
		JSON:      true,
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: prompts.Catalog()},
			{Role: provider.RoleUser, Content: string(payload)},
		},
	})
	if err != nil {
		return nil, "", err
	}
	var parsed struct {
		Items []Normalized `json:"items"`
	}
	if err := extractJSON(resp.Content, &parsed); err != nil {
		return nil, "", err
	}
	known := make(map[string]bool, len(items))
	for _, it := range items {
		known[it.Key] = true
	}
	out := make([]Normalized, 0, len(parsed.Items))
	for _, n := range parsed.Items {
		n.CanonicalName = strings.TrimSpace(n.CanonicalName)
		if known[n.Key] && n.CanonicalName != "" {
			out = append(out, n)
		}
	}
	return out, resp.Model, nil
}
