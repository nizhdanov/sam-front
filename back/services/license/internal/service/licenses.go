package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/services/license/internal/domain"
)

func (s *Service) ListEntitlements(ctx context.Context, f domain.ContractFilter, page httpx.PageRequest) ([]domain.Entitlement, int, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, 0, err
	}
	return s.entitlements.List(ctx, f, page)
}

func (s *Service) GetEntitlement(ctx context.Context, id uuid.UUID) (domain.Entitlement, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Entitlement{}, err
	}
	return s.entitlements.Get(ctx, id)
}

func validateEntitlement(e domain.Entitlement) error {
	switch {
	case !e.Metric.Valid():
		return apperr.Invalid("unknown license_metric")
	case e.Quantity < 0:
		return apperr.Invalid("quantity must not be negative")
	case e.ValidFrom != nil && e.ExpiresAt != nil && !e.ExpiresAt.After(*e.ValidFrom):
		return apperr.Invalid("expires_at must be after valid_from")
	}
	return nil
}

func (s *Service) CreateEntitlement(ctx context.Context, e domain.Entitlement) (domain.Entitlement, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Entitlement{}, err
	}
	e.ID = uuid.New()
	if e.ProductID == uuid.Nil {
		return domain.Entitlement{}, apperr.Invalid("product_id is required")
	}
	if err := validateEntitlement(e); err != nil {
		return domain.Entitlement{}, err
	}
	err := s.withProducts(ctx, []uuid.UUID{e.ProductID}, func(ctx context.Context) error {
		if err := s.entitlements.Create(ctx, e); err != nil {
			return err
		}
		if err := s.record(ctx, "contract", e.ID.String(), domain.AuditCreate, contractSummary("Добавлен договор", e),
			map[string]any{"product_id": e.ProductID, "quantity": e.Quantity, "expires_at": e.ExpiresAt}); err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: []uuid.UUID{e.ProductID}})
	})
	if err != nil {
		return domain.Entitlement{}, err
	}
	return s.entitlements.Get(ctx, e.ID)
}

func (s *Service) UpdateEntitlement(ctx context.Context, id uuid.UUID, p domain.EntitlementPatch) (domain.Entitlement, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Entitlement{}, err
	}
	current, err := s.entitlements.Get(ctx, id)
	if err != nil {
		return domain.Entitlement{}, err
	}
	if err := validateEntitlement(applyPatch(current, p)); err != nil {
		return domain.Entitlement{}, err
	}
	products := []uuid.UUID{current.ProductID}
	if p.ProductID != nil {
		products = append(products, *p.ProductID)
	}
	err = s.withProducts(ctx, products, func(ctx context.Context) error {
		if err := s.entitlements.Update(ctx, id, p); err != nil {
			return err
		}
		if err := s.record(ctx, "contract", id.String(), domain.AuditUpdate, contractSummary("Изменён договор", current),
			contractChanges(p)); err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: products})
	})
	if err != nil {
		return domain.Entitlement{}, err
	}
	return s.entitlements.Get(ctx, id)
}

func applyPatch(e domain.Entitlement, p domain.EntitlementPatch) domain.Entitlement {
	if p.Metric != nil {
		e.Metric = *p.Metric
	}
	if p.Quantity != nil {
		e.Quantity = *p.Quantity
	}
	if p.ValidFrom.Set {
		e.ValidFrom = p.ValidFrom.Value
	}
	if p.ExpiresAt.Set {
		e.ExpiresAt = p.ExpiresAt.Value
	}
	return e
}

func (s *Service) DeactivateEntitlement(ctx context.Context, id uuid.UUID) (domain.Entitlement, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Entitlement{}, err
	}
	current, err := s.entitlements.Get(ctx, id)
	if err != nil {
		return domain.Entitlement{}, err
	}
	err = s.withProducts(ctx, []uuid.UUID{current.ProductID}, func(ctx context.Context) error {
		if err := s.entitlements.Deactivate(ctx, id); err != nil {
			return err
		}
		if err := s.record(ctx, "contract", id.String(), domain.AuditUpdate, contractSummary("Деактивирован договор", current),
			map[string]any{"is_active": false}); err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: []uuid.UUID{current.ProductID}})
	})
	if err != nil {
		return domain.Entitlement{}, err
	}
	return s.entitlements.Get(ctx, id)
}

func contractSummary(prefix string, e domain.Entitlement) string {
	if e.PurchaseReference != nil {
		return prefix + " " + *e.PurchaseReference
	}
	return prefix
}

func contractChanges(p domain.EntitlementPatch) map[string]any {
	out := map[string]any{}
	if p.ProductID != nil {
		out["product_id"] = *p.ProductID
	}
	if p.Metric != nil {
		out["license_metric"] = *p.Metric
	}
	if p.Quantity != nil {
		out["quantity"] = *p.Quantity
	}
	if p.ValidFrom.Set {
		out["valid_from"] = p.ValidFrom.Value
	}
	if p.ExpiresAt.Set {
		out["expires_at"] = p.ExpiresAt.Value
	}
	if p.PurchaseReference.Set {
		out["contract_number"] = p.PurchaseReference.Value
	}
	if p.AgreementID.Set {
		out["agreement_id"] = p.AgreementID.Value
	}
	return out
}

func (s *Service) Balances(ctx context.Context, productID *uuid.UUID) ([]domain.Balance, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	sc := domain.Scope{All: true}
	if productID != nil {
		if _, err := s.products.Get(ctx, *productID); err != nil {
			return nil, err
		}
		sc = domain.Scope{ProductIDs: []uuid.UUID{*productID}}
	}
	facts, err := s.installations.ProductFacts(ctx, sc)
	if err != nil {
		return nil, err
	}
	entitlements, err := s.entitlements.Active(ctx, sc)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	balances := []domain.Balance{}
	for _, p := range facts {
		b, _ := domain.EvaluateLicensing(p, entitlements[p.ID], now)
		balances = append(balances, b...)
	}
	return balances, nil
}

func (s *Service) CommercialBalances(ctx context.Context, productID *uuid.UUID) ([]domain.Balance, error) {
	balances, err := s.Balances(ctx, productID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Balance, 0, len(balances))
	for _, b := range balances {
		if b.CostModel == nil || b.CostModel.RequiresEntitlement() || (b.Purchased != nil && *b.Purchased > 0) {
			out = append(out, b)
		}
	}
	return out, nil
}
