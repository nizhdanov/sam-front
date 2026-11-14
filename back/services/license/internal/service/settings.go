package service

import (
	"context"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/services/license/internal/domain"
)

const matchingSettingsKey = "matching"

type MatchingSettings struct {
	AutoApplyConfidence float64 `json:"auto_apply_confidence"`
	FuzzyAutoThreshold  float64 `json:"fuzzy_auto_threshold"`
}

func (s *Service) matchConfig(ctx context.Context) (domain.MatchingConfig, error) {
	cfg := s.matchDefaults
	var stored MatchingSettings
	ok, err := s.settings.Get(ctx, matchingSettingsKey, &stored)
	if err != nil || !ok {
		return cfg, err
	}
	cfg.LLMAcceptConfidence, cfg.HighThreshold = stored.AutoApplyConfidence, stored.FuzzyAutoThreshold
	return cfg, nil
}

func (s *Service) MatchingSettings(ctx context.Context) (MatchingSettings, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return MatchingSettings{}, err
	}
	cfg, err := s.matchConfig(ctx)
	if err != nil {
		return MatchingSettings{}, err
	}
	return MatchingSettings{AutoApplyConfidence: cfg.LLMAcceptConfidence, FuzzyAutoThreshold: cfg.HighThreshold}, nil
}

func (s *Service) UpdateMatchingSettings(ctx context.Context, in MatchingSettings) (MatchingSettings, error) {
	c, err := requireAdmin(ctx)
	if err != nil {
		return MatchingSettings{}, err
	}
	for _, v := range []float64{in.AutoApplyConfidence, in.FuzzyAutoThreshold} {
		if v <= 0 || v > 1 {
			return MatchingSettings{}, apperr.Invalid("thresholds must be in (0, 1]")
		}
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.settings.Set(ctx, matchingSettingsKey, in, c.Actor()); err != nil {
			return err
		}
		return s.record(ctx, "settings", matchingSettingsKey, domain.AuditUpdate, "Изменены пороги сопоставления",
			map[string]any{"auto_apply_confidence": in.AutoApplyConfidence, "fuzzy_auto_threshold": in.FuzzyAutoThreshold})
	})
	if err != nil {
		return MatchingSettings{}, err
	}
	return in, nil
}

func (s *Service) AuditLog(ctx context.Context, f domain.AuditFilter, page httpx.PageRequest) ([]domain.AuditEntry, int, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, 0, err
	}
	return s.audit.List(ctx, f, page)
}
