package service

import (
	"context"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/services/license/internal/domain"
)

func (s *Service) ListDevices(ctx context.Context, f domain.DeviceFilter, page httpx.PageRequest) ([]domain.Device, int, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return nil, 0, err
	}
	return s.devices.List(ctx, f, page)
}

func (s *Service) MyDevices(ctx context.Context, page httpx.PageRequest) ([]domain.Device, int, error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, 0, err
	}
	employee := employeeOf(c)
	if employee == nil {
		return []domain.Device{}, 0, nil
	}
	return s.devices.List(ctx, domain.DeviceFilter{EmployeeID: employee}, page)
}

func (s *Service) GetDevice(ctx context.Context, id uuid.UUID) (domain.Device, error) {
	c, err := caller(ctx)
	if err != nil {
		return domain.Device{}, err
	}
	d, err := s.devices.Get(ctx, id)
	if err != nil {
		return domain.Device{}, err
	}
	if c.IsPrivileged() {
		return d, nil
	}
	if employee := employeeOf(c); employee != nil && d.EmployeeID != nil && *d.EmployeeID == *employee {
		return d, nil
	}
	return domain.Device{}, apperr.NotFound("device not found")
}

func (s *Service) UpdateDevice(ctx context.Context, id uuid.UUID, p domain.DevicePatch) (domain.Device, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Device{}, err
	}
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		products, err := s.installations.ActiveProductIDs(ctx, id)
		if err != nil {
			return err
		}
		if err := s.locks.Products(ctx, products); err != nil {
			return err
		}
		if err := s.devices.Update(ctx, id, p); err != nil {
			return err
		}
		if !p.EmployeeID.Set {
			return nil
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: products})
	})
	if err != nil {
		return domain.Device{}, err
	}
	return s.devices.Get(ctx, id)
}

func (s *Service) DeviceSoftware(ctx context.Context, id uuid.UUID, includeRemoved bool, page httpx.PageRequest) ([]domain.Installation, int, error) {
	if _, err := s.GetDevice(ctx, id); err != nil {
		return nil, 0, err
	}
	return s.installations.List(ctx, domain.InstallationFilter{DeviceID: &id, IncludeRemoved: includeRemoved}, page)
}

func (s *Service) DeviceFindings(ctx context.Context, id uuid.UUID, page httpx.PageRequest) ([]domain.Finding, int, error) {
	if _, err := s.GetDevice(ctx, id); err != nil {
		return nil, 0, err
	}
	return s.ListFindings(ctx, domain.FindingFilter{DeviceID: &id}, page)
}

func (s *Service) ProductInstallations(ctx context.Context, productID uuid.UUID, page httpx.PageRequest) ([]domain.Installation, int, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return nil, 0, err
	}
	if _, err := s.products.Get(ctx, productID); err != nil {
		return nil, 0, err
	}
	return s.installations.List(ctx, domain.InstallationFilter{ProductID: &productID}, page)
}

func (s *Service) ListEmployees(ctx context.Context, search, department string, page httpx.PageRequest) ([]domain.Employee, int, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return nil, 0, err
	}
	return s.employees.List(ctx, search, department, page)
}

func (s *Service) GetEmployee(ctx context.Context, id uuid.UUID) (domain.Employee, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return domain.Employee{}, err
	}
	return s.employees.Get(ctx, id)
}

func (s *Service) UpdateEmployee(ctx context.Context, id uuid.UUID, p domain.EmployeePatch) (domain.Employee, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Employee{}, err
	}
	if p.FullName != nil && *p.FullName == "" {
		return domain.Employee{}, apperr.Invalid("full_name must not be empty")
	}
	if err := s.employees.Update(ctx, id, p); err != nil {
		return domain.Employee{}, err
	}
	return s.employees.Get(ctx, id)
}

func (s *Service) EmployeeDevices(ctx context.Context, id uuid.UUID, page httpx.PageRequest) ([]domain.Device, int, error) {
	if _, err := s.GetEmployee(ctx, id); err != nil {
		return nil, 0, err
	}
	return s.devices.List(ctx, domain.DeviceFilter{EmployeeID: &id}, page)
}

func (s *Service) EmployeeSoftware(ctx context.Context, id uuid.UUID, page httpx.PageRequest) ([]domain.Installation, int, error) {
	if _, err := s.GetEmployee(ctx, id); err != nil {
		return nil, 0, err
	}
	return s.installations.List(ctx, domain.InstallationFilter{EmployeeID: &id}, page)
}
