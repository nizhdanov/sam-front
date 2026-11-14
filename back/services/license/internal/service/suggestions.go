package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/pkg/swfilter"
	"sam/pkg/swkey"
	"sam/services/license/internal/domain"
)

const (
	suggestionBatch   = 40
	suggestionWorkers = 4
)

type SuggestionsStarted struct {
	Status  string `json:"status"`
	Pending int    `json:"pending"`
	Batches int    `json:"batches"`
}

type ApprovalStarted struct {
	Status      string `json:"status"`
	Suggestions int    `json:"suggestions"`
}

func (s *Service) StartCatalogSuggestions(ctx context.Context) (SuggestionsStarted, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return SuggestionsStarted{}, err
	}
	tasks, err := s.pendingTasks(ctx)
	if err != nil {
		return SuggestionsStarted{}, err
	}
	tasks = slices.DeleteFunc(tasks, func(t domain.MatchingTask) bool { return swfilter.Excluded(t.RawName) })
	batches := chunk(tasks, suggestionBatch)
	if len(batches) == 0 {
		return SuggestionsStarted{Status: "nothing to do"}, nil
	}
	if !s.suggesting.CompareAndSwap(false, true) {
		return SuggestionsStarted{}, apperr.Conflict("catalog suggestions are already being generated")
	}
	go func() {
		defer s.suggesting.Store(false)
		bg := context.WithoutCancel(ctx)
		start := time.Now()
		groups, failed := s.normalizeAll(bg, batches)
		for _, g := range groups {
			if err := s.suggestions.Merge(bg, g); err != nil {
				s.log.ErrorContext(bg, "save catalog suggestion", "name", g.CanonicalName, "err", err)
			}
		}
		s.log.InfoContext(bg, "catalog suggestions generated", "tasks", len(tasks), "suggestions", len(groups),
			"failed_batches", failed, "duration", time.Since(start))
		summary := fmt.Sprintf("AI предложил %d продуктов каталога по %d нераспознанным названиям", len(groups), len(tasks))
		if err := s.record(bg, "catalog_suggestion", "", "GENERATE", summary,
			map[string]any{"tasks": len(tasks), "suggestions": len(groups), "failed_batches": failed}); err != nil {
			s.log.ErrorContext(bg, "record catalog suggestions", "err", err)
		}
	}()
	return SuggestionsStarted{Status: "started", Pending: len(tasks), Batches: len(batches)}, nil
}

func (s *Service) normalizeAll(ctx context.Context, batches [][]domain.MatchingTask) (map[string]domain.CatalogSuggestion, int) {
	var (
		mu     sync.Mutex
		wg     sync.WaitGroup
		failed int
		groups = map[string]domain.CatalogSuggestion{}
		queue  = make(chan []domain.MatchingTask)
	)
	for range suggestionWorkers {
		wg.Go(func() {
			for batch := range queue {
				result, err := s.normalizeBatch(ctx, batch)
				mu.Lock()
				if err != nil {
					failed++
					s.log.WarnContext(ctx, "normalize batch failed", "size", len(batch), "err", err)
				}
				for key, g := range result {
					if existing, ok := groups[key]; ok {
						existing.Items = append(existing.Items, g.Items...)
						existing.IsSoftware = existing.IsSoftware && g.IsSoftware
						groups[key] = existing
						continue
					}
					groups[key] = g
				}
				mu.Unlock()
			}
		})
	}
	for _, b := range batches {
		queue <- b
	}
	close(queue)
	wg.Wait()
	return groups, failed
}

func (s *Service) normalizeBatch(ctx context.Context, batch []domain.MatchingTask) (map[string]domain.CatalogSuggestion, error) {
	input := make([]domain.NormalizeInput, 0, len(batch))
	byKey := make(map[string]domain.MatchingTask, len(batch))
	for _, t := range batch {
		input = append(input, domain.NormalizeInput{Key: t.SourceKey, DisplayName: t.RawName, Publisher: text(t.RawPublisher)})
		byKey[t.SourceKey] = t
	}
	normalized, model, err := s.normalizer.Normalize(ctx, input)
	if err != nil {
		return nil, err
	}
	out := map[string]domain.CatalogSuggestion{}
	for _, n := range normalized {
		t, ok := byKey[n.Key]
		if !ok {
			continue
		}
		groupKey := swkey.Normalize(n.CanonicalName)
		if groupKey == "" {
			continue
		}
		item := domain.SuggestionItem{TaskID: t.ID, SourceKey: t.SourceKey, RawName: t.RawName, RawPublisher: t.RawPublisher, RawVersion: t.RawVersion}
		g, exists := out[groupKey]
		if !exists {
			g = domain.CatalogSuggestion{GroupKey: groupKey, CanonicalName: n.CanonicalName, IsSoftware: n.IsSoftware, Model: &model}
			if p := strings.TrimSpace(n.Publisher); p != "" {
				g.Publisher = &p
			}
		}
		g.IsSoftware = g.IsSoftware && n.IsSoftware
		g.Items = append(g.Items, item)
		out[groupKey] = g
	}
	return out, nil
}

func (s *Service) CatalogSuggestions(ctx context.Context, status domain.ReviewStatus, page httpx.PageRequest) ([]domain.CatalogSuggestion, int, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, 0, err
	}
	if status == "" {
		status = domain.ReviewPending
	}
	return s.suggestions.List(ctx, status, page)
}

func (s *Service) StartApproveSuggestions(ctx context.Context, ids []uuid.UUID, all bool) (ApprovalStarted, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return ApprovalStarted{}, err
	}
	if all {
		pending, err := s.suggestions.PendingIDs(ctx)
		if err != nil {
			return ApprovalStarted{}, err
		}
		ids = pending
	}
	if len(ids) == 0 {
		return ApprovalStarted{}, apperr.Invalid("ids or all=true is required")
	}
	if !s.approving.CompareAndSwap(false, true) {
		return ApprovalStarted{}, apperr.Conflict("catalog suggestions are already being approved")
	}
	go func() {
		defer s.approving.Store(false)
		bg := context.WithoutCancel(ctx)
		approved, failed := 0, 0
		for _, id := range ids {
			if err := s.approveSuggestion(bg, id); err != nil {
				failed++
				s.log.WarnContext(bg, "approve catalog suggestion failed", "id", id, "err", err)
				continue
			}
			approved++
		}
		summary := fmt.Sprintf("Применены AI-предложения каталога: %d из %d", approved, len(ids))
		if err := s.record(bg, "catalog_suggestion", "", domain.AuditApprove, summary,
			map[string]any{"approved": approved, "failed": failed}); err != nil {
			s.log.ErrorContext(bg, "record catalog approval", "err", err)
		}
	}()
	return ApprovalStarted{Status: "started", Suggestions: len(ids)}, nil
}

func (s *Service) RejectSuggestion(ctx context.Context, id uuid.UUID) (domain.CatalogSuggestion, error) {
	c, err := requireAdmin(ctx)
	if err != nil {
		return domain.CatalogSuggestion{}, err
	}
	if err := s.suggestions.Resolve(ctx, id, domain.ReviewRejected, nil, c.Actor()); err != nil {
		return domain.CatalogSuggestion{}, err
	}
	return s.suggestions.Get(ctx, id)
}

func (s *Service) approveSuggestion(ctx context.Context, id uuid.UUID) error {
	g, err := s.suggestions.Get(ctx, id)
	if err != nil {
		return err
	}
	if g.Status != domain.ReviewPending {
		return apperr.Conflict("catalog suggestion is already reviewed")
	}
	reviewer := actor(ctx)
	if !g.IsSoftware {
		for _, it := range g.Items {
			if err := s.dismissTask(ctx, it.TaskID, it.SourceKey, "AI: не является самостоятельной программой"); err != nil {
				return err
			}
		}
		return s.suggestions.Resolve(ctx, id, domain.ReviewApproved, nil, reviewer)
	}
	productID, err := s.productFor(ctx, g)
	if err != nil {
		return err
	}
	for _, it := range g.Items {
		a := domain.Assignment{
			SourceKey: it.SourceKey, RawName: it.RawName, RawPublisher: it.RawPublisher, RawVersion: it.RawVersion,
			ProductID: productID, Method: domain.MethodLLM, ReviewedBy: &reviewer,
		}
		if err := s.tx.InTx(ctx, func(ctx context.Context) error { return s.assign(ctx, a) }); err != nil {
			return err
		}
	}
	return s.suggestions.Resolve(ctx, id, domain.ReviewApproved, &productID, reviewer)
}

func (s *Service) productFor(ctx context.Context, g domain.CatalogSuggestion) (uuid.UUID, error) {
	candidates, err := s.products.Candidates(ctx, g.GroupKey, 5)
	if err != nil {
		return uuid.Nil, err
	}
	for _, c := range candidates {
		if swkey.Normalize(c.Name) == g.GroupKey && domain.PublishersCompatible(text(g.Publisher), c.Publisher) {
			return c.ProductID, nil
		}
	}
	p, err := s.CreateProduct(ctx, ProductInput{CanonicalName: g.CanonicalName, Publisher: g.Publisher})
	if apperr.KindOf(err) == apperr.KindConflict {
		p, err = s.findByName(ctx, g)
	}
	return p.ID, err
}

func (s *Service) findByName(ctx context.Context, g domain.CatalogSuggestion) (domain.Product, error) {
	items, _, err := s.products.List(ctx, domain.ProductFilter{Search: g.CanonicalName}, httpx.PageRequest{Page: 1, Limit: 20})
	if err != nil {
		return domain.Product{}, err
	}
	for _, p := range items {
		if swkey.Normalize(p.CanonicalName) == g.GroupKey {
			return p, nil
		}
	}
	return domain.Product{}, apperr.NotFound("product for suggestion not found")
}

func (s *Service) dismissTask(ctx context.Context, taskID uuid.UUID, key, reason string) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.matching.Reject(ctx, taskID, actor(ctx), reason); err != nil {
			return err
		}
		devices, err := s.installations.DeactivateByKey(ctx, key, s.now())
		if err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{DeviceIDs: devices})
	})
}

func chunk[T any](items []T, size int) [][]T {
	var out [][]T
	for len(items) > 0 {
		n := min(size, len(items))
		out = append(out, items[:n])
		items = items[n:]
	}
	return out
}
