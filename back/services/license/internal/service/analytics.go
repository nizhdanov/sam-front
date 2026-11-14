package service

import (
	"context"

	"github.com/google/uuid"

	"sam/services/license/internal/domain"
)

func (s *Service) Dashboard(ctx context.Context) (domain.Dashboard, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Dashboard{}, err
	}
	d, err := s.analytics.Dashboard(ctx, s.expiringDays)
	if err != nil {
		return domain.Dashboard{}, err
	}
	balances, err := s.Balances(ctx, nil)
	if err != nil {
		return domain.Dashboard{}, err
	}
	deficit, idle := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, b := range balances {
		if b.Deficit > 0 {
			deficit[b.ProductID] = true
			d.Deficit.Seats += b.Deficit
		}
		if b.Available != nil && *b.Available > 0 {
			idle[b.ProductID] = true
			d.Idle.Seats += *b.Available
		}
	}
	d.Deficit.Products, d.Idle.Products = len(deficit), len(idle)
	return d, nil
}

func (s *Service) FreeSoftware(ctx context.Context) ([]domain.FreeSoftware, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return s.analytics.FreeSoftware(ctx)
}
