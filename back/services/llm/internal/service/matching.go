package service

import (
	"context"
	"encoding/json"
	"slices"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/services/llm/internal/prompts"
	"sam/services/llm/internal/provider"
)

type Candidate struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Publisher string `json:"publisher,omitempty"`
	Edition   string `json:"edition,omitempty"`
}

type Software struct {
	DisplayName string `json:"display_name"`
	Publisher   string `json:"publisher"`
	Version     string `json:"version"`
}

type MatchResult struct {
	Status     string  `json:"status"`
	ProductID  *string `json:"product_id"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
	Model      string  `json:"-"`
}

func (s *Service) MatchSoftware(ctx context.Context, sw Software, candidates []Candidate) (MatchResult, error) {
	if _, err := auth.Require(ctx, auth.RoleInventoryWorker, auth.RoleAdmin); err != nil {
		return MatchResult{}, err
	}
	if len(candidates) == 0 {
		return MatchResult{Status: "REQUIRES_REVIEW", Reason: "no candidates"}, nil
	}
	release, err := s.acquire(ctx)
	if err != nil {
		return MatchResult{}, err
	}
	defer release()

	payload, err := json.Marshal(map[string]any{"software": sw, "candidates": candidates})
	if err != nil {
		return MatchResult{}, err
	}
	resp, err := s.complete(ctx, provider.Request{
		Task:      "matching",
		Model:     s.cfg.Models.Matching,
		MaxTokens: 300,
		JSON:      true,
		Messages: []provider.Message{
			{Role: provider.RoleSystem, Content: prompts.Matching()},
			{Role: provider.RoleUser, Content: string(payload)},
		},
	})
	if err != nil {
		return MatchResult{}, err
	}

	var result MatchResult
	if err := extractJSON(resp.Content, &result); err != nil {
		return MatchResult{}, err
	}
	result.Model = resp.Model
	result.Confidence = min(max(result.Confidence, 0), 1)

	valid := result.ProductID != nil && slices.ContainsFunc(candidates, func(c Candidate) bool { return c.ID == *result.ProductID })
	if result.Status != "MATCHED" || !valid {
		if result.Status == "MATCHED" && !valid {
			result.Reason = "model chose a product outside the candidate list"
		}
		result.Status, result.ProductID = "REQUIRES_REVIEW", nil
	}
	if result.Status == "MATCHED" && result.ProductID == nil {
		return MatchResult{}, apperr.Unavailable("AI model returned an incomplete result")
	}
	return result, nil
}
