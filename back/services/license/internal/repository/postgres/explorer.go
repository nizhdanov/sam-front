package postgres

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type column struct {
	name string
	expr string
}

type filter struct {
	expr  string
	exact bool
}

type dataset struct {
	from      string
	where     string
	scope     string
	columns   []column
	filters   map[string]filter
	groups    map[string]string
	countExpr string
	order     string
}

var datasets = map[domain.Dataset]dataset{
	domain.DatasetInstallations: {
		from: `installations i
			JOIN devices d ON d.id = i.device_id
			LEFT JOIN software_products p ON p.id = i.product_id
			LEFT JOIN software_versions v ON v.id = i.version_id
			LEFT JOIN license_agreements a ON a.id = p.agreement_id`,
		where: "i.is_active",
		scope: "d.employee_id",
		columns: []column{
			{"product_id", "p.id"},
			{"serial_number", "d.serial_number"},
			{"hostname", "d.hostname"},
			{"product", "COALESCE(p.canonical_name, i.raw_name)"},
			{"publisher", "COALESCE(p.publisher, i.raw_publisher)"},
			{"version", "COALESCE(v.version, i.raw_version)"},
			{"match_status", "i.match_status"},
			{"policy_status", "p.policy_status"},
			{"lifecycle_status", "v.lifecycle_status"},
			{"license_type", "a.license_type"},
			{"last_seen_at", "i.last_seen_at"},
		},
		filters: map[string]filter{
			"product":          {expr: "COALESCE(p.canonical_name, i.raw_name)"},
			"publisher":        {expr: "COALESCE(p.publisher, i.raw_publisher)"},
			"version":          {expr: "COALESCE(v.version, i.raw_version)"},
			"serial_number":    {expr: "d.serial_number"},
			"match_status":     {expr: "i.match_status", exact: true},
			"policy_status":    {expr: "p.policy_status", exact: true},
			"lifecycle_status": {expr: "v.lifecycle_status", exact: true},
			"license_type":     {expr: "a.license_type", exact: true},
		},
		groups: map[string]string{
			"product":          "COALESCE(p.canonical_name, i.raw_name)",
			"publisher":        "COALESCE(p.publisher, i.raw_publisher)",
			"version":          "COALESCE(p.canonical_name, i.raw_name) || ' ' || COALESCE(v.version, i.raw_version, '')",
			"serial_number":    "d.serial_number",
			"match_status":     "i.match_status",
			"policy_status":    "p.policy_status",
			"lifecycle_status": "v.lifecycle_status",
			"license_type":     "a.license_type",
		},
		countExpr: "COUNT(DISTINCT d.id)",
		order:     "d.serial_number, 4",
	},
	domain.DatasetDevices: {
		from:  "devices d",
		where: "TRUE",
		scope: "d.employee_id",
		columns: []column{
			{"serial_number", "d.serial_number"},
			{"hostname", "d.hostname"},
			{"operating_system", "d.operating_system"},
			{"last_inventory_at", "d.last_inventory_at"},
			{"installations", "(SELECT COUNT(*) FROM installations i WHERE i.device_id = d.id AND i.is_active)"},
			{"open_findings", "(SELECT COUNT(*) FROM compliance_findings f WHERE f.device_id = d.id AND f.status = 'OPEN')"},
		},
		filters: map[string]filter{
			"serial_number":    {expr: "d.serial_number"},
			"hostname":         {expr: "d.hostname"},
			"operating_system": {expr: "d.operating_system"},
		},
		groups: map[string]string{
			"operating_system": "d.operating_system",
		},
		countExpr: "COUNT(*)",
		order:     "d.serial_number",
	},
	domain.DatasetProducts: {
		from:  "software_products p LEFT JOIN license_agreements a ON a.id = p.agreement_id",
		where: "p.archived_at IS NULL",
		columns: []column{
			{"product_id", "p.id"},
			{"product", "p.canonical_name"},
			{"publisher", "p.publisher"},
			{"edition", "p.edition"},
			{"policy_status", "p.policy_status"},
			{"license_type", "a.license_type"},
			{"devices", "(SELECT COUNT(DISTINCT i.device_id) FROM installations i WHERE i.product_id = p.id AND i.is_active)"},
		},
		filters: map[string]filter{
			"product":       {expr: "p.canonical_name"},
			"publisher":     {expr: "p.publisher"},
			"policy_status": {expr: "p.policy_status", exact: true},
			"license_type":  {expr: "a.license_type", exact: true},
		},
		groups: map[string]string{
			"publisher":     "p.publisher",
			"policy_status": "p.policy_status",
			"license_type":  "a.license_type",
		},
		countExpr: "COUNT(*)",
		order:     "p.canonical_name",
	},
	domain.DatasetLicenses: {
		from:  "license_entitlements e JOIN software_products p ON p.id = e.product_id",
		where: "TRUE",
		columns: []column{
			{"product", "p.canonical_name"},
			{"metric", "e.license_metric"},
			{"quantity", "e.quantity"},
			{"valid_from", "e.valid_from"},
			{"expires_at", "e.expires_at"},
			{"status", licenseStatusExpr},
			{"purchase_reference", "e.purchase_reference"},
		},
		filters: map[string]filter{
			"product": {expr: "p.canonical_name"},
			"metric":  {expr: "e.license_metric", exact: true},
			"status":  {expr: licenseStatusExpr, exact: true},
		},
		groups: map[string]string{
			"product": "p.canonical_name",
			"metric":  "e.license_metric",
			"status":  licenseStatusExpr,
		},
		countExpr: "SUM(e.quantity)::bigint",
		order:     "p.canonical_name",
	},
	domain.DatasetFindings: {
		from: `compliance_findings f
			LEFT JOIN devices d ON d.id = f.device_id
			LEFT JOIN software_products p ON p.id = f.product_id`,
		where: "TRUE",
		scope: "d.employee_id",
		columns: []column{
			{"finding_type", "f.finding_type"},
			{"severity", "f.severity"},
			{"status", "f.status"},
			{"serial_number", "d.serial_number"},
			{"product", "p.canonical_name"},
			{"last_detected_at", "f.last_detected_at"},
		},
		filters: map[string]filter{
			"finding_type":  {expr: "f.finding_type", exact: true},
			"severity":      {expr: "f.severity", exact: true},
			"status":        {expr: "f.status", exact: true},
			"serial_number": {expr: "d.serial_number"},
			"product":       {expr: "p.canonical_name"},
		},
		groups: map[string]string{
			"finding_type":  "f.finding_type",
			"severity":      "f.severity",
			"status":        "f.status",
			"serial_number": "d.serial_number",
			"product":       "p.canonical_name",
		},
		countExpr: "COUNT(*)",
		order:     "CASE f.severity WHEN 'CRITICAL' THEN 0 WHEN 'HIGH' THEN 1 WHEN 'MEDIUM' THEN 2 ELSE 3 END, 4",
	},
}

const licenseStatusExpr = `CASE WHEN NOT e.is_active THEN 'INACTIVE'
	WHEN e.expires_at IS NOT NULL AND e.expires_at <= NOW() THEN 'EXPIRED'
	WHEN e.valid_from IS NOT NULL AND e.valid_from > NOW() THEN 'NOT_STARTED'
	ELSE 'ACTIVE' END`

type Explorer struct{ pool *pgxpool.Pool }

func NewExplorer(pool *pgxpool.Pool) *Explorer { return &Explorer{pool: pool} }

func (r *Explorer) Query(ctx context.Context, q domain.DataQuery) (domain.DataTable, error) {
	ds, ok := datasets[q.Dataset]
	if !ok {
		return domain.DataTable{}, apperr.Invalid("unknown dataset " + string(q.Dataset))
	}
	limit := q.Limit
	if limit <= 0 || limit > domain.MaxQueryRows {
		limit = domain.MaxQueryRows
	}

	conds := []string{ds.where}
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if q.EmployeeID != nil {
		if ds.scope == "" {
			return domain.DataTable{}, apperr.Forbidden("dataset is not available")
		}
		conds = append(conds, ds.scope+" = "+arg(*q.EmployeeID))
	}
	names := make([]string, 0, len(q.Filters))
	for name := range q.Filters {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		value := strings.TrimSpace(q.Filters[name])
		if value == "" {
			continue
		}
		f, ok := ds.filters[name]
		if !ok {
			return domain.DataTable{}, apperr.Invalid(fmt.Sprintf("filter %q is not supported for %s", name, q.Dataset))
		}
		if f.exact {
			conds = append(conds, f.expr+" = "+arg(strings.ToUpper(value)))
		} else {
			conds = append(conds, f.expr+" ILIKE "+arg("%"+escapeLike(value)+"%"))
		}
	}
	where := strings.Join(conds, " AND ")

	if q.GroupBy != "" {
		expr, ok := ds.groups[q.GroupBy]
		if !ok {
			return domain.DataTable{}, apperr.Invalid(fmt.Sprintf("group_by %q is not supported for %s", q.GroupBy, q.Dataset))
		}
		sql := fmt.Sprintf(`SELECT %s, %s, COUNT(*) OVER () FROM %s WHERE %s GROUP BY 1 ORDER BY 2 DESC, 1 LIMIT %d`,
			expr, ds.countExpr, ds.from, where, limit)
		table := domain.DataTable{Dataset: q.Dataset, GroupBy: q.GroupBy, Columns: []string{q.GroupBy, countColumn(q.Dataset)}}
		return r.fetch(ctx, sql, args, table)
	}

	exprs := make([]string, 0, len(ds.columns))
	table := domain.DataTable{Dataset: q.Dataset}
	for _, c := range ds.columns {
		if q.EmployeeID != nil && c.name == "devices" {
			continue
		}
		exprs = append(exprs, c.expr)
		table.Columns = append(table.Columns, c.name)
	}
	sql := fmt.Sprintf(`SELECT %s, COUNT(*) OVER () FROM %s WHERE %s ORDER BY %s LIMIT %d`,
		strings.Join(exprs, ", "), ds.from, where, ds.order, limit)
	return r.fetch(ctx, sql, args, table)
}

func (r *Explorer) fetch(ctx context.Context, sql string, args []any, table domain.DataTable) (domain.DataTable, error) {
	rows, err := pg.DB(ctx, r.pool).Query(ctx, sql, args...)
	if err != nil {
		return table, fmt.Errorf("query %s: %w", table.Dataset, err)
	}
	defer rows.Close()
	table.Rows = [][]any{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return table, fmt.Errorf("scan %s: %w", table.Dataset, err)
		}
		last := len(values) - 1
		table.Total = int(toInt64(values[last]))
		row := make([]any, last)
		for i, v := range values[:last] {
			row[i] = plain(v)
		}
		table.Rows = append(table.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return table, fmt.Errorf("query %s: %w", table.Dataset, err)
	}
	table.Shown = len(table.Rows)
	return table, nil
}

func countColumn(d domain.Dataset) string {
	switch d {
	case domain.DatasetInstallations:
		return "devices"
	case domain.DatasetLicenses:
		return "quantity"
	}
	return "count"
}

func plain(v any) any {
	switch t := v.(type) {
	case time.Time:
		return t.Format(time.DateOnly)
	case [16]byte:
		return fmt.Sprintf("%x-%x-%x-%x-%x", t[0:4], t[4:6], t[6:8], t[8:10], t[10:16])
	}
	return v
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int32:
		return int64(t)
	case int:
		return int64(t)
	}
	return 0
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}
