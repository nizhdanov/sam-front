package service

import (
	"context"

	"sam/pkg/apperr"
	"sam/services/license/internal/domain"
)

func (s *Service) Dashboard(ctx context.Context) (domain.Dashboard, error) {
	c, err := caller(ctx)
	if err != nil {
		return domain.Dashboard{}, err
	}
	if !c.IsAdmin() {
		employee := employeeOf(c)
		if employee == nil {
			return domain.Dashboard{}, apperr.Forbidden("user is not linked to an employee")
		}
		return s.analytics.Dashboard(ctx, employee)
	}

	d, err := s.analytics.Dashboard(ctx, nil)
	if err != nil {
		return domain.Dashboard{}, err
	}
	balances, err := s.Balances(ctx, nil)
	if err != nil {
		return domain.Dashboard{}, err
	}
	stats := domain.LicenseStats{}
	deficit := map[string]bool{}
	unverified := map[string]bool{}
	for _, b := range balances {
		if b.Deficit > 0 {
			deficit[b.ProductID.String()] = true
			stats.TotalDeficit += b.Deficit
		}
		if b.CalculationStatus != domain.CalcVerified && b.Consumed > 0 {
			unverified[b.ProductID.String()] = true
		}
	}
	stats.ProductsWithDeficit, stats.ProductsUnverified = len(deficit), len(unverified)
	d.Licenses = &stats
	return d, nil
}

func (s *Service) SoftwareAnalytics(ctx context.Context, limit int) ([]domain.SoftwareUsage, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return s.analytics.Software(ctx, limit)
}

func (s *Service) DepartmentAnalytics(ctx context.Context) ([]domain.DepartmentStats, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return s.analytics.Departments(ctx)
}
