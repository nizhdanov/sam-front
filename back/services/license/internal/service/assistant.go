package service

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/services/license/internal/domain"
)

const assistantLimit = 50

type assistantParams struct {
	Query        string `json:"query"`
	SerialNumber string `json:"serial_number"`
	EmployeeID   string `json:"employee_id"`
	Product      string `json:"product"`
	PolicyStatus string `json:"policy_status"`
}

type softwareItem struct {
	Name     string `json:"name"`
	Version  string `json:"version,omitempty"`
	Status   string `json:"status,omitempty"`
	Matched  bool   `json:"matched"`
	Device   string `json:"device,omitempty"`
	Category string `json:"policy,omitempty"`
}

type findingItem struct {
	Type     domain.FindingType `json:"type"`
	Severity domain.Severity    `json:"severity"`
	Device   string             `json:"device,omitempty"`
	Product  string             `json:"product,omitempty"`
	Details  map[string]any     `json:"details,omitempty"`
}

func (s *Service) RunQuery(ctx context.Context, name string, raw []byte) (any, error) {
	var p assistantParams
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, apperr.Invalid("malformed query parameters")
		}
	}
	page := httpx.PageRequest{Page: 1, Limit: assistantLimit}

	switch name {
	case "get_allowed_software":
		return s.assistantCatalog(ctx, p, page)
	case "get_my_devices":
		devices, _, err := s.MyDevices(ctx, page)
		return devices, err
	case "get_my_software":
		return s.assistantMySoftware(ctx, page)
	case "get_my_compliance":
		items, _, err := s.ListFindings(ctx, domain.FindingFilter{}, page)
		return findingItems(items), err
	case "get_device_inventory":
		return s.assistantDevice(ctx, p.SerialNumber, page)
	case "get_employee_inventory":
		return s.assistantEmployee(ctx, p, page)
	case "get_paid_licenses", "get_license_costs":
		return s.assistantLicenses(ctx, name == "get_license_costs", page)
	case "get_license_balance":
		return s.assistantBalance(ctx, p.Product)
	case "get_company_compliance":
		if _, err := requirePrivileged(ctx); err != nil {
			return nil, err
		}
		summary, err := s.ComplianceSummary(ctx)
		if err != nil {
			return nil, err
		}
		items, total, err := s.ListFindings(ctx, domain.FindingFilter{}, page)
		return map[string]any{"summary": summary, "top_findings": findingItems(items), "total_open": total}, err
	case "get_dashboard_summary":
		if _, err := requirePrivileged(ctx); err != nil {
			return nil, err
		}
		return s.Dashboard(ctx)
	}
	return nil, apperr.Invalid("unknown query " + name)
}

func (s *Service) assistantCatalog(ctx context.Context, p assistantParams, page httpx.PageRequest) (any, error) {
	f := domain.ProductFilter{Search: p.Query, PolicyStatus: domain.PolicyStatus(strings.ToUpper(p.PolicyStatus))}
	page.Limit = 100
	products, total, err := s.ListProducts(ctx, f, page)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PublicProduct, 0, len(products))
	for _, pr := range products {
		out = append(out, pr.Public())
	}
	return map[string]any{"products": out, "total": total}, nil
}

func (s *Service) assistantMySoftware(ctx context.Context, page httpx.PageRequest) (any, error) {
	devices, _, err := s.MyDevices(ctx, page)
	if err != nil {
		return nil, err
	}
	var out []softwareItem
	for _, d := range devices {
		items, _, err := s.DeviceSoftware(ctx, d.ID, false, httpx.PageRequest{Page: 1, Limit: httpx.MaxLimit})
		if err != nil {
			return nil, err
		}
		out = append(out, softwareItems(items)...)
	}
	return out, nil
}

func (s *Service) assistantDevice(ctx context.Context, serial string, page httpx.PageRequest) (any, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return nil, err
	}
	if serial == "" {
		return nil, apperr.Invalid("serial_number is required")
	}
	devices, _, err := s.devices.List(ctx, domain.DeviceFilter{SerialNumber: serial}, page)
	if err != nil {
		return nil, err
	}
	if len(devices) == 0 {
		return nil, apperr.NotFound("device not found")
	}
	d := devices[0]
	items, _, err := s.installations.List(ctx, domain.InstallationFilter{DeviceID: &d.ID}, httpx.PageRequest{Page: 1, Limit: httpx.MaxLimit})
	if err != nil {
		return nil, err
	}
	return map[string]any{"device": d, "software": softwareItems(items)}, nil
}

func (s *Service) assistantEmployee(ctx context.Context, p assistantParams, page httpx.PageRequest) (any, error) {
	if _, err := requirePrivileged(ctx); err != nil {
		return nil, err
	}
	var employee domain.Employee
	if id, err := uuid.Parse(p.EmployeeID); err == nil {
		if employee, err = s.employees.Get(ctx, id); err != nil {
			return nil, err
		}
	} else {
		found, _, err := s.employees.List(ctx, p.Query, "", httpx.PageRequest{Page: 1, Limit: 5})
		if err != nil {
			return nil, err
		}
		if len(found) == 0 {
			return nil, apperr.NotFound("employee not found")
		}
		if len(found) > 1 {
			return map[string]any{"ambiguous": true, "employees": found}, nil
		}
		employee = found[0]
	}
	items, _, err := s.installations.List(ctx, domain.InstallationFilter{EmployeeID: &employee.ID}, httpx.PageRequest{Page: 1, Limit: httpx.MaxLimit})
	if err != nil {
		return nil, err
	}
	return map[string]any{"employee": employee, "software": softwareItems(items)}, nil
}

func (s *Service) assistantLicenses(ctx context.Context, withCosts bool, page httpx.PageRequest) (any, error) {
	items, _, err := s.ListEntitlements(ctx, nil, true, page)
	if err != nil {
		return nil, err
	}
	type license struct {
		Product   string   `json:"product"`
		Metric    string   `json:"metric"`
		Quantity  int      `json:"quantity"`
		ExpiresAt any      `json:"expires_at,omitempty"`
		UnitPrice *float64 `json:"unit_price,omitempty"`
		Currency  *string  `json:"currency,omitempty"`
		Total     *float64 `json:"total_cost,omitempty"`
	}
	out := make([]license, 0, len(items))
	for _, e := range items {
		l := license{Product: e.ProductName, Metric: string(e.Metric), Quantity: e.Quantity}
		if e.ExpiresAt != nil {
			l.ExpiresAt = e.ExpiresAt.Format("2006-01-02")
		}
		if withCosts && e.UnitPrice != nil {
			total := *e.UnitPrice * float64(e.Quantity)
			l.UnitPrice, l.Currency, l.Total = e.UnitPrice, e.Currency, &total
		}
		out = append(out, l)
	}
	return out, nil
}

func (s *Service) assistantBalance(ctx context.Context, product string) (any, error) {
	balances, err := s.Balances(ctx, nil)
	if err != nil {
		return nil, err
	}
	if product == "" {
		return balances, nil
	}
	needle := strings.ToLower(product)
	var out []domain.Balance
	for _, b := range balances {
		if strings.Contains(strings.ToLower(b.ProductName), needle) {
			out = append(out, b)
		}
	}
	return out, nil
}

func softwareItems(items []domain.Installation) []softwareItem {
	out := make([]softwareItem, 0, len(items))
	for _, i := range items {
		item := softwareItem{Name: i.RawName, Version: value(i.RawVersion), Matched: i.ProductID != nil, Device: i.SerialNumber}
		if i.ProductName != nil {
			item.Name = *i.ProductName
		}
		if i.PolicyStatus != nil {
			item.Category = *i.PolicyStatus
		}
		if i.LifecycleStatus != nil {
			item.Status = *i.LifecycleStatus
		}
		out = append(out, item)
	}
	return out
}

func findingItems(items []domain.Finding) []findingItem {
	out := make([]findingItem, 0, len(items))
	for _, f := range items {
		out = append(out, findingItem{Type: f.Type, Severity: f.Severity, Device: value(f.SerialNumber), Product: value(f.ProductName), Details: f.Details})
	}
	return out
}

func value(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
