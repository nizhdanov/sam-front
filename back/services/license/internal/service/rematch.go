package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/pkg/swfilter"
	"sam/services/license/internal/domain"
)

type RematchResult struct {
	Checked   int `json:"checked"`
	Matched   int `json:"matched"`
	Ambiguous int `json:"ambiguous"`
	Unknown   int `json:"unknown"`
	Excluded  int `json:"excluded"`
	Failed    int `json:"failed"`
}

type RematchStarted struct {
	Status  string `json:"status"`
	Pending int    `json:"pending"`
}

func (s *Service) StartRematch(ctx context.Context) (RematchStarted, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return RematchStarted{}, err
	}
	tasks, err := s.pendingTasks(ctx)
	if err != nil {
		return RematchStarted{}, err
	}
	if !s.rematching.CompareAndSwap(false, true) {
		return RematchStarted{}, apperr.Conflict("rematch is already running")
	}
	go func() {
		defer s.rematching.Store(false)
		bg := context.WithoutCancel(ctx)
		start := time.Now()
		res := s.rematch(bg, tasks)
		s.log.InfoContext(bg, "pending matching tasks rematched", "checked", res.Checked, "matched", res.Matched,
			"ambiguous", res.Ambiguous, "unknown", res.Unknown, "excluded", res.Excluded, "failed", res.Failed, "duration", time.Since(start))
		summary := fmt.Sprintf("Повторное сопоставление очереди: сопоставлено %d из %d", res.Matched, res.Checked)
		if err := s.record(bg, "matching", "", "REMATCH", summary, map[string]any{
			"checked": res.Checked, "matched": res.Matched, "ambiguous": res.Ambiguous, "unknown": res.Unknown,
			"excluded": res.Excluded, "failed": res.Failed,
		}); err != nil {
			s.log.ErrorContext(bg, "record rematch", "err", err)
		}
	}()
	return RematchStarted{Status: "started", Pending: len(tasks)}, nil
}

func (s *Service) pendingTasks(ctx context.Context) ([]domain.MatchingTask, error) {
	var all []domain.MatchingTask
	for page := 1; ; page++ {
		items, total, err := s.matching.List(ctx, domain.ReviewPending, httpx.PageRequest{Page: page, Limit: httpx.MaxLimit})
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) == 0 || len(all) >= total {
			return all, nil
		}
	}
}

func (s *Service) rematch(ctx context.Context, tasks []domain.MatchingTask) RematchResult {
	var res RematchResult
	for _, t := range tasks {
		if ctx.Err() != nil {
			break
		}
		res.Checked++
		if reason, ok := swfilter.Classify(t.RawName); ok {
			if err := s.dismissTask(ctx, t.ID, t.SourceKey, "не ПО: "+string(reason)); err != nil {
				res.Failed++
				continue
			}
			res.Excluded++
			continue
		}
		item := domain.RawSoftware{SourceKey: t.SourceKey, DisplayName: t.RawName, Publisher: text(t.RawPublisher), Version: text(t.RawVersion)}
		resolution, err := s.resolveNew(ctx, item)
		if err != nil {
			res.Failed++
			s.log.WarnContext(ctx, "rematch task failed", "task_id", t.ID, "err", err)
			continue
		}
		switch resolution.Status {
		case domain.ResolutionMatched:
			res.Matched++
		case domain.ResolutionAmbiguous:
			res.Ambiguous++
			if err := s.matching.UpsertPending(ctx, domain.Suggestion{
				Software: item, Candidates: resolution.Candidates, Reason: "catalog candidates found on rematch",
			}); err != nil {
				res.Failed++
			}
		default:
			res.Unknown++
		}
	}
	return res
}

func text(s *string) string {
	if s == nil {
		return ""
	}
	return *s
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
