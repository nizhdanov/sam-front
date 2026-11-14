package matching

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"golang.org/x/sync/singleflight"

	"sam/services/worker/internal/domain"
)

const batchSize = 500

type Catalog interface {
	Resolve(ctx context.Context, items []domain.Item) ([]domain.Resolution, error)
	SaveSuggestion(ctx context.Context, item domain.Item, candidates []domain.Candidate, s domain.Suggestion, method string) (domain.Match, error)
}

type Model interface {
	Match(ctx context.Context, item domain.Item, candidates []domain.Candidate) (domain.Suggestion, error)
}

type Config struct {
	CacheSize int
	CacheTTL  time.Duration
}

type Matcher struct {
	catalog Catalog
	model   Model
	cache   *expirable.LRU[string, domain.Match]
	group   singleflight.Group
	log     *slog.Logger
}

func New(catalog Catalog, model Model, cfg Config, log *slog.Logger) *Matcher {
	return &Matcher{
		catalog: catalog,
		model:   model,
		cache:   expirable.NewLRU[string, domain.Match](cfg.CacheSize, nil, cfg.CacheTTL),
		log:     log,
	}
}

func (m *Matcher) Resolve(ctx context.Context, items []domain.Item) (map[string]domain.Match, error) {
	out := make(map[string]domain.Match, len(items))
	var misses []domain.Item
	for _, it := range items {
		if match, ok := m.cache.Get(it.SourceKey); ok {
			out[it.SourceKey] = match
			continue
		}
		misses = append(misses, it)
	}

	byKey := make(map[string]domain.Item, len(misses))
	for _, it := range misses {
		byKey[it.SourceKey] = it
	}

	for start := 0; start < len(misses); start += batchSize {
		chunk := misses[start:min(start+batchSize, len(misses))]
		resolutions, err := m.catalog.Resolve(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("resolve software: %w", err)
		}
		for _, r := range resolutions {
			match, err := m.settle(ctx, byKey[r.SourceKey], r)
			if err != nil {
				return nil, err
			}
			out[r.SourceKey] = match
		}
	}
	return out, nil
}

func (m *Matcher) settle(ctx context.Context, item domain.Item, r domain.Resolution) (domain.Match, error) {
	switch r.Status {
	case domain.ResolutionMatched:
		match := domain.Match{ProductID: r.ProductID, VersionID: r.VersionID, Status: domain.MatchMatched}
		m.cache.Add(item.SourceKey, match)
		return match, nil
	case domain.ResolutionPendingReview:
		return domain.Match{Status: domain.MatchRequiresReview}, nil
	case domain.ResolutionUnknown:
		return domain.Match{Status: domain.MatchUnknown}, nil
	}

	v, err, _ := m.group.Do(item.SourceKey, func() (any, error) {
		return m.askModel(ctx, item, r.Candidates)
	})
	if err != nil {
		return domain.Match{}, err
	}
	match := v.(domain.Match)
	if match.Status == domain.MatchMatched {
		m.cache.Add(item.SourceKey, match)
	}
	return match, nil
}

func (m *Matcher) askModel(ctx context.Context, item domain.Item, candidates []domain.Candidate) (domain.Match, error) {
	if m.model == nil {
		return m.catalog.SaveSuggestion(ctx, item, candidates, domain.Suggestion{Reason: "ambiguous match, AI matching is disabled"}, "")
	}
	suggestion, err := m.model.Match(ctx, item, candidates)
	if err != nil {
		m.log.WarnContext(ctx, "llm matching unavailable, leaving software for review", "software", item.DisplayName, "err", err)
		return m.catalog.SaveSuggestion(ctx, item, candidates, domain.Suggestion{Reason: "ambiguous match, AI matching is unavailable"}, "")
	}
	return m.catalog.SaveSuggestion(ctx, item, candidates, suggestion, "LLM")
}
