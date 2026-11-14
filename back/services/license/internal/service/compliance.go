package service

import (
	"context"
	"slices"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/pkg/httpx"
	"sam/services/license/internal/domain"
)

func compact(ids []uuid.UUID) []uuid.UUID {
	out := slices.Clone(ids)
	slices.SortFunc(out, func(a, b uuid.UUID) int { return slices.Compare(a[:], b[:]) })
	return slices.Compact(out)
}

func (s *Service) recalculate(ctx context.Context, sc domain.Scope) error {
	sc.DeviceIDs, sc.ProductIDs = compact(sc.DeviceIDs), compact(sc.ProductIDs)
	if sc.Empty() {
		return nil
	}
	now := s.now()

	facts, err := s.installations.Facts(ctx, sc)
	if err != nil {
		return err
	}
	var desired []domain.Finding
	for _, f := range facts {
		desired = append(desired, domain.InstallationFindings(f)...)
	}
	resolved, err := s.findings.Apply(ctx, desired, domain.InstallationFindingTypes, sc, now)
	if err != nil {
		return err
	}

	if sc.All || len(sc.ProductIDs) > 0 {
		productScope := domain.Scope{All: sc.All, ProductIDs: sc.ProductIDs}
		products, err := s.installations.ProductFacts(ctx, productScope)
		if err != nil {
			return err
		}
		entitlements, err := s.entitlements.Active(ctx, productScope)
		if err != nil {
			return err
		}
		var productFindings []domain.Finding
		for _, p := range products {
			_, f := domain.EvaluateLicensing(p, entitlements[p.ID], now)
			productFindings = append(productFindings, f...)
		}
		n, err := s.findings.Apply(ctx, productFindings, domain.ProductFindingTypes, productScope, now)
		if err != nil {
			return err
		}
		resolved += n
		desired = append(desired, productFindings...)
	}
	s.log.DebugContext(ctx, "compliance recalculated", "detected", len(desired), "resolved", resolved)
	return nil
}

func (s *Service) withProducts(ctx context.Context, productIDs []uuid.UUID, fn func(ctx context.Context) error) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.locks.Products(ctx, productIDs); err != nil {
			return err
		}
		return fn(ctx)
	})
}

func (s *Service) RecalculateAll(ctx context.Context) error {
	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.locks.All(ctx); err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{All: true})
	})
}

func (s *Service) TriggerRecalculation(ctx context.Context) error {
	if _, err := requireAdmin(ctx); err != nil {
		return err
	}
	return s.RecalculateAll(ctx)
}

func (s *Service) RunPeriodicChecks(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := s.RecalculateAll(ctx); err != nil {
				s.log.ErrorContext(ctx, "periodic compliance recalculation failed", "err", err)
			}
		}
	}
}

func (s *Service) scopeFindings(ctx context.Context, f domain.FindingFilter) (domain.FindingFilter, bool, error) {
	c, err := caller(ctx)
	if err != nil {
		return f, false, err
	}
	if c.IsPrivileged() {
		return f, false, nil
	}
	employee := employeeOf(c)
	if employee == nil {
		return f, true, nil
	}
	f.EmployeeID = employee
	if len(f.Types) == 0 {
		f.Types = domain.InstallationFindingTypes
	} else {
		f.Types = slices.DeleteFunc(slices.Clone(f.Types), func(t domain.FindingType) bool {
			return !slices.Contains(domain.InstallationFindingTypes, t)
		})
		if len(f.Types) == 0 {
			return f, true, nil
		}
	}
	return f, false, nil
}

func (s *Service) ListFindings(ctx context.Context, f domain.FindingFilter, page httpx.PageRequest) ([]domain.Finding, int, error) {
	f, none, err := s.scopeFindings(ctx, f)
	if err != nil || none {
		return []domain.Finding{}, 0, err
	}
	items, total, err := s.findings.List(ctx, f, page)
	if err != nil {
		return nil, 0, err
	}
	if !auth.ClaimsFrom(ctx).IsPrivileged() {
		for i := range items {
			items[i].Details = publicDetails(items[i].Details)
		}
	}
	return items, total, nil
}

func publicDetails(details map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"raw_name", "raw_version", "product_name", "version"} {
		if v, ok := details[key]; ok {
			out[key] = v
		}
	}
	return out
}

func (s *Service) GetFinding(ctx context.Context, id uuid.UUID) (domain.Finding, error) {
	f, err := s.findings.Get(ctx, id)
	if err != nil {
		return domain.Finding{}, err
	}
	c, err := caller(ctx)
	if err != nil {
		return domain.Finding{}, err
	}
	if c.IsPrivileged() {
		return f, nil
	}
	if f.DeviceID == nil || !slices.Contains(domain.InstallationFindingTypes, f.Type) {
		return domain.Finding{}, apperr.NotFound("finding not found")
	}
	if _, err := s.GetDevice(ctx, *f.DeviceID); err != nil {
		return domain.Finding{}, apperr.NotFound("finding not found")
	}
	f.Details = publicDetails(f.Details)
	return f, nil
}

func (s *Service) AcknowledgeFinding(ctx context.Context, id uuid.UUID) (domain.Finding, error) {
	c, err := requireAdmin(ctx)
	if err != nil {
		return domain.Finding{}, err
	}
	if err := s.findings.Acknowledge(ctx, id, c.Actor()); err != nil {
		return domain.Finding{}, err
	}
	return s.findings.Get(ctx, id)
}

func (s *Service) ComplianceSummary(ctx context.Context) (domain.ComplianceSummary, error) {
	f, none, err := s.scopeFindings(ctx, domain.FindingFilter{})
	if err != nil {
		return domain.ComplianceSummary{}, err
	}
	if none {
		return domain.ComplianceSummary{BySeverity: map[domain.Severity]int{}, ByType: map[domain.FindingType]int{}}, nil
	}
	return s.findings.Summary(ctx, f)
}
