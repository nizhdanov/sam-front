package service

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/services/license/internal/domain"
)

type balanceRow struct {
	Product   string `json:"product"`
	Metric    string `json:"metric"`
	Purchased *int   `json:"purchased"`
	Consumed  int    `json:"consumed"`
	Available *int   `json:"available"`
	Deficit   int    `json:"deficit"`
	Status    string `json:"status"`
	Note      string `json:"note,omitempty"`
}

func (s *Service) RunQuery(ctx context.Context, name string, raw []byte) (any, error) {
	switch name {
	case "query_data":
		var q domain.DataQuery
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &q); err != nil {
				return nil, apperr.Invalid("malformed query parameters")
			}
		}
		return s.QueryData(ctx, q)
	case "get_license_balance":
		var p struct {
			Product     string `json:"product"`
			OnlyDeficit bool   `json:"only_deficit"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &p); err != nil {
				return nil, apperr.Invalid("malformed query parameters")
			}
		}
		return s.assistantBalance(ctx, p.Product, p.OnlyDeficit)
	case "get_summary":
		return s.Dashboard(ctx)
	}
	return nil, apperr.Invalid("unknown query " + name)
}

func (s *Service) QueryData(ctx context.Context, q domain.DataQuery) (domain.DataTable, error) {
	c, err := caller(ctx)
	if err != nil {
		return domain.DataTable{}, err
	}
	if !c.IsAdmin() {
		if q.Dataset == domain.DatasetLicenses {
			return domain.DataTable{}, apperr.Forbidden("license data is available to administrators only")
		}
		employee := employeeOf(c)
		if employee == nil {
			return domain.DataTable{}, apperr.Forbidden("user is not linked to an employee")
		}
		q.EmployeeID = employee
	}
	table, err := s.explorer.Query(ctx, q)
	if err != nil {
		return table, err
	}
	idx := slices.Index(table.Columns, "product_id")
	if idx < 0 {
		return table, nil
	}
	var statuses map[uuid.UUID]string
	if c.IsAdmin() {
		if statuses, err = s.licenseStatuses(ctx); err != nil {
			return table, err
		}
		table.Columns = append(table.Columns, "license_status")
	}
	table.Columns = slices.Delete(table.Columns, idx, idx+1)
	for i, row := range table.Rows {
		if statuses != nil {
			row = append(row, licenseStatusOf(row[idx], statuses))
		}
		table.Rows[i] = slices.Delete(row, idx, idx+1)
	}
	return table, nil
}

func (s *Service) licenseStatuses(ctx context.Context) (map[uuid.UUID]string, error) {
	balances, err := s.Balances(ctx, nil)
	if err != nil {
		return nil, err
	}
	out := make(map[uuid.UUID]string, len(balances))
	deficit := map[uuid.UUID]int{}
	for _, b := range balances {
		deficit[b.ProductID] += b.Deficit
		if _, ok := out[b.ProductID]; !ok || b.CalculationStatus != domain.CalcVerified {
			out[b.ProductID] = string(b.CalculationStatus)
		}
	}
	for id, d := range deficit {
		if d > 0 {
			out[id] = fmt.Sprintf("DEFICIT %d", d)
		} else if out[id] == string(domain.CalcVerified) {
			out[id] = "COVERED"
		}
	}
	return out, nil
}

func licenseStatusOf(v any, statuses map[uuid.UUID]string) string {
	raw, ok := v.(string)
	if !ok {
		return "UNMATCHED"
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return "UNMATCHED"
	}
	if status, ok := statuses[id]; ok {
		return status
	}
	return "NOT_REQUIRED"
}

func (s *Service) assistantBalance(ctx context.Context, product string, onlyDeficit bool) (any, error) {
	balances, err := s.Balances(ctx, nil)
	if err != nil {
		return nil, err
	}
	needle := strings.ToLower(strings.TrimSpace(product))
	out := []balanceRow{}
	total := 0
	for _, b := range balances {
		if needle != "" && !strings.Contains(strings.ToLower(b.ProductName), needle) {
			continue
		}
		if onlyDeficit && b.Deficit == 0 {
			continue
		}
		total += b.Deficit
		out = append(out, balanceRow{
			Product: b.ProductName, Metric: string(b.Metric), Purchased: b.Purchased, Consumed: b.Consumed,
			Available: b.Available, Deficit: b.Deficit, Status: string(b.CalculationStatus), Note: b.Note,
		})
	}
	return map[string]any{"rows": out, "total_deficit": total}, nil
}
