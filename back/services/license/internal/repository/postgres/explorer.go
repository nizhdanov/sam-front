package postgres

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"sam/pkg/apperr"
	pg "sam/pkg/postgres"
	"sam/services/license/internal/domain"
)

type column struct {
	name      string
	expr      string
	adminOnly bool
}

type filterKind int

const (
	textFilter filterKind = iota
	exactFilter
	uuidFilter
	boolFilter
	phraseFilter
)

type filter struct {
	expr string
	kind filterKind
}

type dataset struct {
	from      string
	where     string
	scope     string
	userWhere string
	adminOnly bool
	columns   []column
	filters   map[string]filter
	groups    map[string]string
	countExpr string
	order     string
}

const installationsFrom = `installations i
	JOIN devices d ON d.id = i.device_id
	LEFT JOIN software_products p ON p.id = i.product_id
	LEFT JOIN software_versions v ON v.id = i.version_id
	LEFT JOIN license_agreements a ON a.id = p.agreement_id`

const productName = "COALESCE(p.canonical_name, i.raw_name)"
const installedVersion = "COALESCE(v.version, i.raw_version)"

var datasets = map[domain.Dataset]dataset{
	domain.DatasetInstallations: {
		from:  installationsFrom,
		where: "i.is_active",
		scope: "d.serial_number",
		columns: []column{
			{name: "product_id", expr: "p.id"},
			{name: "device_id", expr: "d.id"},
			{name: "serial_number", expr: "d.serial_number"},
			{name: "device_type", expr: "d.device_type"},
			{name: "raw_name", expr: "i.raw_name"},
			{name: "product", expr: "p.canonical_name"},
			{name: "publisher", expr: "COALESCE(p.publisher, i.raw_publisher)"},
			{name: "version", expr: installedVersion},
			{name: "current_version", expr: "p.current_version"},
			{name: "match_status", expr: "i.match_status"},
			{name: "status", expr: "p.status"},
			{name: "license_type", expr: "a.license_type"},
			{name: "cost_model", expr: "a.cost_model"},
			{name: "first_seen_at", expr: "i.first_seen_at"},
			{name: "last_seen_at", expr: "i.last_seen_at"},
		},
		filters: map[string]filter{
			"product":       {expr: productName},
			"product_id":    {expr: "p.id", kind: uuidFilter},
			"raw_name":      {expr: "i.raw_name"},
			"publisher":     {expr: "COALESCE(p.publisher, i.raw_publisher)"},
			"version":       {expr: installedVersion},
			"serial_number": {expr: "d.serial_number"},
			"device_type":   {expr: "d.device_type", kind: exactFilter},
			"match_status":  {expr: "i.match_status", kind: exactFilter},
			"recognized":    {expr: "(i.product_id IS NOT NULL)", kind: boolFilter},
			"status":        {expr: "p.status", kind: exactFilter},
			"license_type":  {expr: "a.license_type", kind: exactFilter},
			"cost_model":    {expr: "a.cost_model", kind: exactFilter},
			"in_bpo":        {expr: "p.in_bpo", kind: boolFilter},
		},
		groups: map[string]string{
			"product":       productName,
			"publisher":     "COALESCE(p.publisher, i.raw_publisher)",
			"version":       productName + " || ' ' || COALESCE(" + installedVersion + ", '')",
			"serial_number": "d.serial_number",
			"device_type":   "d.device_type",
			"match_status":  "i.match_status",
			"status":        "p.status",
			"license_type":  "a.license_type",
			"cost_model":    "a.cost_model",
		},
		countExpr: "COUNT(DISTINCT d.id)",
		order:     "d.serial_number, i.raw_name",
	},
	domain.DatasetDevices: {
		from:  "devices d",
		where: "TRUE",
		scope: "d.serial_number",
		columns: []column{
			{name: "device_id", expr: "d.id"},
			{name: "serial_number", expr: "d.serial_number"},
			{name: "device_type", expr: "d.device_type"},
			{name: "status", expr: "d.status"},
			{name: "hostname", expr: "d.hostname"},
			{name: "responsible", expr: "d.responsible"},
			{name: "location", expr: "d.location"},
			{name: "last_inventory_at", expr: "d.last_inventory_at"},
			{name: "installations", expr: "(SELECT COUNT(*) FROM installations i WHERE i.device_id = d.id AND i.is_active)"},
			{name: "open_findings", expr: "(SELECT COUNT(*) FROM compliance_findings f WHERE f.device_id = d.id AND f.status = 'OPEN')"},
		},
		filters: map[string]filter{
			"serial_number": {expr: "d.serial_number"},
			"hostname":      {expr: "d.hostname"},
			"responsible":   {expr: "d.responsible"},
			"location":      {expr: "d.location"},
			"device_type":   {expr: "d.device_type", kind: exactFilter},
			"status":        {expr: "d.status", kind: exactFilter},
		},
		groups: map[string]string{
			"device_type": "d.device_type",
			"status":      "d.status",
			"location":    "d.location",
		},
		countExpr: "COUNT(*)",
		order:     "d.serial_number",
	},
	domain.DatasetProducts: {
		from:      "software_products p LEFT JOIN license_agreements a ON a.id = p.agreement_id",
		where:     "p.archived_at IS NULL",
		userWhere: "(p.in_bpo OR a.cost_model = 'FREE' OR p.status = 'FORBIDDEN')",
		columns: []column{
			{name: "product_id", expr: "p.id"},
			{name: "product", expr: "p.canonical_name"},
			{name: "publisher", expr: "p.publisher"},
			{name: "current_version", expr: "p.current_version"},
			{name: "purpose", expr: "p.purpose"},
			{name: "section", expr: "p.section"},
			{name: "class", expr: "p.class"},
			{name: "in_bpo", expr: "p.in_bpo"},
			{name: "status", expr: "p.status"},
			{name: "status_comment", expr: "p.status_comment"},
			{name: "license_type", expr: "a.license_type"},
			{name: "cost_model", expr: "a.cost_model"},
			{name: "purchased", expr: activeSeatsExpr, adminOnly: true},
			{name: "devices", expr: "(SELECT COUNT(DISTINCT i.device_id) FROM installations i WHERE i.product_id = p.id AND i.is_active)", adminOnly: true},
		},
		filters: map[string]filter{
			"product":      {expr: "p.canonical_name"},
			"product_id":   {expr: "p.id", kind: uuidFilter},
			"publisher":    {expr: "p.publisher"},
			"purpose":      {expr: "p.purpose", kind: phraseFilter},
			"section":      {expr: "p.section"},
			"class":        {expr: "p.class"},
			"status":       {expr: "p.status", kind: exactFilter},
			"license_type": {expr: "a.license_type", kind: exactFilter},
			"cost_model":   {expr: "a.cost_model", kind: exactFilter},
			"in_bpo":       {expr: "p.in_bpo", kind: boolFilter},
		},
		groups: map[string]string{
			"publisher":    "p.publisher",
			"section":      "p.section",
			"class":        "p.class",
			"status":       "p.status",
			"license_type": "a.license_type",
			"cost_model":   "a.cost_model",
			"in_bpo":       "p.in_bpo::text",
		},
		countExpr: "COUNT(*)",
		order:     "p.canonical_name",
	},
	domain.DatasetContracts: {
		from:      "license_entitlements e JOIN software_products p ON p.id = e.product_id",
		where:     "TRUE",
		adminOnly: true,
		columns: []column{
			{name: "contract_id", expr: "e.id"},
			{name: "product_id", expr: "p.id"},
			{name: "product", expr: "p.canonical_name"},
			{name: "contract_number", expr: "e.purchase_reference"},
			{name: "metric", expr: "e.license_metric"},
			{name: "quantity", expr: "e.quantity"},
			{name: "valid_from", expr: "e.valid_from"},
			{name: "expires_at", expr: "e.expires_at"},
			{name: "status", expr: contractStatusExpr},
		},
		filters: map[string]filter{
			"product":         {expr: "p.canonical_name"},
			"product_id":      {expr: "p.id", kind: uuidFilter},
			"contract_number": {expr: "e.purchase_reference"},
			"metric":          {expr: "e.license_metric", kind: exactFilter},
			"status":          {expr: contractStatusExpr, kind: exactFilter},
		},
		groups: map[string]string{
			"product": "p.canonical_name",
			"metric":  "e.license_metric",
			"status":  contractStatusExpr,
		},
		countExpr: "SUM(e.quantity)::bigint",
		order:     "p.canonical_name, e.expires_at NULLS LAST",
	},
	domain.DatasetFindings: {
		from: `compliance_findings f
			LEFT JOIN devices d ON d.id = f.device_id
			LEFT JOIN software_products p ON p.id = f.product_id`,
		where: "f.status <> 'RESOLVED'",
		scope: "d.serial_number",
		columns: []column{
			{name: "finding_id", expr: "f.id"},
			{name: "product_id", expr: "p.id"},
			{name: "device_id", expr: "d.id"},
			{name: "finding_type", expr: "f.finding_type"},
			{name: "severity", expr: "f.severity"},
			{name: "status", expr: "f.status"},
			{name: "serial_number", expr: "d.serial_number"},
			{name: "raw_name", expr: "f.details->>'raw_name'"},
			{name: "product", expr: "COALESCE(p.canonical_name, f.details->>'product_name')"},
			{name: "version", expr: "COALESCE(f.details->>'version', f.details->>'raw_version')"},
			{name: "current_version", expr: "p.current_version"},
			{name: "detected_at", expr: "f.created_at"},
			{name: "comment", expr: "f.acknowledge_comment"},
		},
		filters: map[string]filter{
			"finding_type":  {expr: "f.finding_type", kind: exactFilter},
			"severity":      {expr: "f.severity", kind: exactFilter},
			"status":        {expr: "f.status", kind: exactFilter},
			"serial_number": {expr: "d.serial_number"},
			"product":       {expr: "COALESCE(p.canonical_name, f.details->>'product_name')"},
			"product_id":    {expr: "p.id", kind: uuidFilter},
		},
		groups: map[string]string{
			"finding_type":  "f.finding_type",
			"severity":      "f.severity",
			"status":        "f.status",
			"serial_number": "d.serial_number",
			"product":       "COALESCE(p.canonical_name, f.details->>'product_name')",
		},
		countExpr: "COUNT(*)",
		order:     "CASE f.severity WHEN 'CRITICAL' THEN 0 WHEN 'HIGH' THEN 1 WHEN 'MEDIUM' THEN 2 ELSE 3 END, d.serial_number",
	},
}

const activeSeatsExpr = `(SELECT COALESCE(SUM(e.quantity), 0) FROM license_entitlements e
	WHERE e.product_id = p.id AND e.is_active
	  AND (e.valid_from IS NULL OR e.valid_from <= NOW()) AND (e.expires_at IS NULL OR e.expires_at > NOW()))`

const contractStatusExpr = `CASE WHEN NOT e.is_active THEN 'INACTIVE'
	WHEN e.expires_at IS NOT NULL AND e.expires_at <= NOW() THEN 'EXPIRED'
	WHEN e.valid_from IS NOT NULL AND e.valid_from > NOW() THEN 'NOT_STARTED'
	WHEN e.expires_at IS NOT NULL AND e.expires_at <= NOW() + INTERVAL '30 days' THEN 'EXPIRING'
	ELSE 'ACTIVE' END`

type Explorer struct{ pool *pgxpool.Pool }

func NewExplorer(pool *pgxpool.Pool) *Explorer { return &Explorer{pool: pool} }

func (r *Explorer) Query(ctx context.Context, q domain.DataQuery) (domain.DataTable, error) {
	ds, ok := datasets[q.Dataset]
	if !ok {
		return domain.DataTable{}, apperr.Invalid("unknown dataset " + string(q.Dataset))
	}
	maxRows := domain.MaxQueryRows
	if q.Export {
		maxRows = domain.MaxExportRows
	}
	limit := q.Limit
	if limit <= 0 || limit > maxRows {
		limit = maxRows
	}
	restricted := !q.Admin

	conds := []string{ds.where}
	var args []any
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}
	if restricted {
		switch {
		case ds.adminOnly:
			return domain.DataTable{}, apperr.Forbidden("dataset is available to administrators only")
		case ds.scope != "":
			if q.DeviceSerial == "" {
				return domain.DataTable{}, apperr.Forbidden("user is not linked to a device")
			}
			conds = append(conds, ds.scope+" = "+arg(q.DeviceSerial))
		case ds.userWhere != "":
			conds = append(conds, ds.userWhere)
		}
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
		switch f.kind {
		case exactFilter:
			conds = append(conds, f.expr+" = "+arg(strings.ToUpper(value)))
		case uuidFilter:
			id, err := uuid.Parse(value)
			if err != nil {
				return domain.DataTable{}, apperr.Invalid(name + " must be a UUID")
			}
			conds = append(conds, f.expr+" = "+arg(id))
		case boolFilter:
			b, err := strconv.ParseBool(value)
			if err != nil {
				return domain.DataTable{}, apperr.Invalid(name + " must be true or false")
			}
			conds = append(conds, f.expr+" = "+arg(b))
		case phraseFilter:
			for _, stem := range stems(value) {
				conds = append(conds, f.expr+" ILIKE "+arg("%"+escapeLike(stem)+"%"))
			}
		default:
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
		if restricted && c.adminOnly {
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
	case domain.DatasetContracts:
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

func stems(phrase string) []string {
	var out []string
	for _, word := range strings.FieldsFunc(phrase, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		runes := []rune(word)
		switch {
		case len(runes) < 3:
			continue
		case len(runes) > 5:
			runes = runes[:len(runes)-2]
		case len(runes) > 3:
			runes = runes[:len(runes)-1]
		}
		out = append(out, string(runes))
	}
	return out
}
