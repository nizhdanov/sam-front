package service

import (
	"context"
	"slices"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/pkg/httpx"
	"sam/pkg/swkey"
	"sam/services/license/internal/domain"
)

func (s *Service) Resolve(ctx context.Context, items []domain.RawSoftware) ([]domain.Resolution, error) {
	if _, err := auth.Require(ctx, auth.RoleInventoryWorker, auth.RoleAdmin); err != nil {
		return nil, err
	}

	byKey := make(map[string]domain.RawSoftware, len(items))
	keys := make([]string, 0, len(items))
	for _, it := range items {
		key := it.Key()
		if _, seen := byKey[key]; !seen {
			keys = append(keys, key)
		}
		it.SourceKey = key
		byKey[key] = it
	}

	aliases, err := s.aliases.BySourceKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	reviews, err := s.matching.StatusByKeys(ctx, keys)
	if err != nil {
		return nil, err
	}

	out := make([]domain.Resolution, 0, len(keys))
	for _, key := range keys {
		if a, ok := aliases[key]; ok {
			productID := a.ProductID
			out = append(out, domain.Resolution{SourceKey: key, Status: domain.ResolutionMatched, ProductID: &productID, VersionID: a.VersionID, Method: a.MatchMethod})
			continue
		}
		switch reviews[key] {
		case domain.ReviewPending:
			out = append(out, domain.Resolution{SourceKey: key, Status: domain.ResolutionPendingReview})
			continue
		case domain.ReviewRejected:
			out = append(out, domain.Resolution{SourceKey: key, Status: domain.ResolutionUnknown})
			continue
		}

		res, err := s.resolveNew(ctx, byKey[key])
		if err != nil {
			return nil, err
		}
		out = append(out, res)
	}
	return out, nil
}

func (s *Service) resolveNew(ctx context.Context, item domain.RawSoftware) (domain.Resolution, error) {
	res := domain.Resolution{SourceKey: item.SourceKey, Status: domain.ResolutionUnknown}
	name := swkey.Normalize(item.DisplayName)
	if name != "" {
		found, err := s.products.Candidates(ctx, name, s.matchCfg.MaxCandidates*2)
		if err != nil {
			return res, err
		}
		for _, c := range found {
			if c.Similarity >= s.matchCfg.CandidateThreshold && domain.PublishersCompatible(item.Publisher, c.Publisher) {
				res.Candidates = append(res.Candidates, c)
			}
		}
		if len(res.Candidates) > s.matchCfg.MaxCandidates {
			res.Candidates = res.Candidates[:s.matchCfg.MaxCandidates]
		}
	}

	status, chosen, method := domain.DecideMatch(name, res.Candidates, s.matchCfg)
	res.Status = status
	switch status {
	case domain.ResolutionMatched:
		a := assignmentFor(item, chosen.ProductID, method)
		if err := s.tx.InTx(ctx, func(ctx context.Context) error { return s.assign(ctx, a) }); err != nil {
			return res, err
		}
		alias, err := s.aliases.BySourceKey(ctx, item.SourceKey)
		if err != nil {
			return res, err
		}
		res.ProductID, res.VersionID, res.Method = &alias.ProductID, alias.VersionID, method
	case domain.ResolutionUnknown:
		err := s.matching.UpsertPending(ctx, domain.Suggestion{Software: item, Reason: "no catalog candidates found"})
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

func assignmentFor(item domain.RawSoftware, productID uuid.UUID, method domain.MatchMethod) domain.Assignment {
	return domain.Assignment{
		SourceKey:    item.Key(),
		RawName:      item.DisplayName,
		RawPublisher: trimmed(&item.Publisher),
		RawVersion:   trimmed(&item.Version),
		ProductID:    productID,
		Method:       method,
	}
}

func (s *Service) SaveSuggestion(ctx context.Context, sg domain.Suggestion) (*domain.Alias, error) {
	if _, err := auth.Require(ctx, auth.RoleInventoryWorker, auth.RoleAdmin); err != nil {
		return nil, err
	}
	sg.Software.SourceKey = sg.Software.Key()

	var chosen *domain.Candidate
	if sg.ProductID != nil {
		idx := slices.IndexFunc(sg.Candidates, func(c domain.Candidate) bool { return c.ProductID == *sg.ProductID })
		if idx < 0 {
			sg.Reason = "suggested product is not among candidates: " + sg.Reason
			sg.ProductID = nil
		} else {
			chosen = &sg.Candidates[idx]
		}
	}

	if !domain.AcceptSuggestion(chosen, sg.Software.Publisher, sg.Confidence, s.matchCfg) {
		return nil, s.matching.UpsertPending(ctx, sg)
	}
	a := assignmentFor(sg.Software, chosen.ProductID, sg.Method)
	if err := s.tx.InTx(ctx, func(ctx context.Context) error { return s.assign(ctx, a) }); err != nil {
		return nil, err
	}
	alias, err := s.aliases.BySourceKey(ctx, a.SourceKey)
	if err != nil {
		return nil, err
	}
	return &alias, nil
}

func (s *Service) MatchingTasks(ctx context.Context, status domain.ReviewStatus, page httpx.PageRequest) ([]domain.MatchingTask, int, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return nil, 0, err
	}
	if status == "" {
		status = domain.ReviewPending
	}
	tasks, total, err := s.matching.List(ctx, status, page)
	if err != nil {
		return nil, 0, err
	}
	return tasks, total, s.attachCandidates(ctx, tasks)
}

func (s *Service) MatchingTask(ctx context.Context, id uuid.UUID) (domain.MatchingTask, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return domain.MatchingTask{}, err
	}
	task, err := s.matching.Get(ctx, id, false)
	if err != nil {
		return domain.MatchingTask{}, err
	}
	tasks := []domain.MatchingTask{task}
	if err := s.attachCandidates(ctx, tasks); err != nil {
		return domain.MatchingTask{}, err
	}
	return tasks[0], nil
}

func (s *Service) attachCandidates(ctx context.Context, tasks []domain.MatchingTask) error {
	var ids []uuid.UUID
	for _, t := range tasks {
		ids = append(ids, t.CandidateIDs...)
	}
	products, err := s.products.GetMany(ctx, compact(ids))
	if err != nil {
		return err
	}
	byID := make(map[uuid.UUID]domain.PublicProduct, len(products))
	for _, p := range products {
		byID[p.ID] = p.Public()
	}
	for i := range tasks {
		tasks[i].Candidates = []domain.PublicProduct{}
		for _, id := range tasks[i].CandidateIDs {
			if p, ok := byID[id]; ok {
				tasks[i].Candidates = append(tasks[i].Candidates, p)
			}
		}
	}
	return nil
}

func (s *Service) ApproveMatching(ctx context.Context, id, productID uuid.UUID, versionID *uuid.UUID) (domain.MatchingTask, error) {
	c, err := requireAdmin(ctx)
	if err != nil {
		return domain.MatchingTask{}, err
	}
	reviewer := c.Actor()
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		task, err := s.matching.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if task.Status != domain.ReviewPending {
			return apperr.Conflict("matching task is already reviewed")
		}
		return s.assign(ctx, domain.Assignment{
			SourceKey:    task.SourceKey,
			RawName:      task.RawName,
			RawPublisher: task.RawPublisher,
			RawVersion:   task.RawVersion,
			ProductID:    productID,
			VersionID:    versionID,
			Method:       domain.MethodManual,
			ReviewedBy:   &reviewer,
		})
	})
	if err != nil {
		return domain.MatchingTask{}, err
	}
	return s.MatchingTask(ctx, id)
}

func (s *Service) RejectMatching(ctx context.Context, id uuid.UUID, reason string) (domain.MatchingTask, error) {
	c, err := requireAdmin(ctx)
	if err != nil {
		return domain.MatchingTask{}, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		task, err := s.matching.Get(ctx, id, true)
		if err != nil {
			return err
		}
		if task.Status != domain.ReviewPending {
			return apperr.Conflict("matching task is already reviewed")
		}
		if err := s.matching.Reject(ctx, id, c.Actor(), reason); err != nil {
			return err
		}
		if _, err := s.aliases.BySourceKey(ctx, task.SourceKey); err == nil {
			return nil
		}
		devices, err := s.installations.Unassign(ctx, task.SourceKey, domain.MatchUnknown)
		if err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{DeviceIDs: devices})
	})
	if err != nil {
		return domain.MatchingTask{}, err
	}
	return s.MatchingTask(ctx, id)
}
