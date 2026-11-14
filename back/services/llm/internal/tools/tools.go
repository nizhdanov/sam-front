package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"unicode/utf8"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/services/llm/internal/provider"
)

const maxResultBytes = 12000

type Definition struct {
	provider.Tool
	Roles []auth.Role
}

var (
	everyone = auth.UserRoles
	admins   = []auth.Role{auth.RoleAdmin}
)

var registry = []Definition{
	{
		Tool: provider.Tool{
			Name: "query_data",
			Description: `Универсальный запрос к данным инвентаризации. Возвращает таблицу {columns, rows, total, shown}.
Наборы данных (dataset):
- installations: установленное ПО по устройствам (serial_number, hostname, product, publisher, version, match_status, policy_status, lifecycle_status, license_type, last_seen_at, license_status). Фильтры: product, publisher, version, serial_number, match_status, policy_status, lifecycle_status, license_type. group_by: product, publisher, version, serial_number, match_status, policy_status, lifecycle_status, license_type (считает число устройств).
- devices: устройства (serial_number, hostname, operating_system, last_inventory_at, installations, open_findings). Фильтры: serial_number, hostname, operating_system. group_by: operating_system.
- products: каталог ПО (product, publisher, edition, policy_status, license_type, devices, license_status). Фильтры: product, publisher, policy_status, license_type. group_by: publisher, policy_status, license_type.
- licenses: зарегистрированные лицензии (product, metric, quantity, valid_from, expires_at, status, purchase_reference). Фильтры: product, metric, status (ACTIVE, EXPIRED, INACTIVE, NOT_STARTED). group_by: product, metric, status (суммирует quantity).
- findings: нарушения (finding_type, severity, status, serial_number, product, last_detected_at). Фильтры: finding_type, severity, status, serial_number, product. group_by: finding_type, severity, status, serial_number, product.
Текстовые фильтры ищут по подстроке, остальные — точное значение. total — сколько строк всего, shown — сколько показано.`,
			Parameters: json.RawMessage(`{"type":"object","properties":{
				"dataset":{"type":"string","enum":["installations","devices","products","licenses","findings"]},
				"filters":{"type":"object","description":"фильтры, допустимые для выбранного набора данных","properties":{"product":{"type":"string"},"publisher":{"type":"string"},"version":{"type":"string"},"serial_number":{"type":"string"},"hostname":{"type":"string"},"operating_system":{"type":"string"},"match_status":{"type":"string"},"policy_status":{"type":"string"},"lifecycle_status":{"type":"string"},"license_type":{"type":"string"},"metric":{"type":"string"},"status":{"type":"string"},"finding_type":{"type":"string"},"severity":{"type":"string"}}},
				"group_by":{"type":"string","description":"поле для группировки и подсчёта"},
				"limit":{"type":"integer","minimum":1,"maximum":100}},
				"required":["dataset"]}`),
		},
		Roles: everyone,
	},
	{
		Tool: provider.Tool{
			Name:        "get_license_balance",
			Description: "Лицензионный баланс по продуктам: куплено, используется, свободно, дефицит и итоговый дефицит. Можно указать продукт и только дефицитные позиции.",
			Parameters: json.RawMessage(`{"type":"object","properties":{
				"product":{"type":"string","description":"часть названия продукта"},
				"only_deficit":{"type":"boolean"}}}`),
		},
		Roles: admins,
	},
	{
		Tool: provider.Tool{
			Name:        "get_summary",
			Description: "Общая сводка: устройства, ПО, лицензионный дефицит, нарушения по критичности.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		Roles: everyone,
	},
}

func ForRole(role auth.Role) []provider.Tool {
	var out []provider.Tool
	for _, d := range registry {
		if slices.Contains(d.Roles, role) {
			out = append(out, d.Tool)
		}
	}
	return out
}

func Allowed(role auth.Role, name string) bool {
	for _, d := range registry {
		if d.Name == name {
			return slices.Contains(d.Roles, role)
		}
	}
	return false
}

type DataSource interface {
	RunQuery(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error)
}

type Executor struct{ data DataSource }

func NewExecutor(data DataSource) *Executor { return &Executor{data: data} }

func (e *Executor) Run(ctx context.Context, role auth.Role, call provider.ToolCall) string {
	if !Allowed(role, call.Name) {
		return errorResult("инструмент недоступен для роли пользователя")
	}
	args := call.Arguments
	if len(args) == 0 || !json.Valid(args) {
		args = json.RawMessage(`{}`)
	}
	result, err := e.data.RunQuery(ctx, call.Name, args)
	if err != nil {
		return errorResult(apperr.Message(err))
	}
	return truncate(string(result))
}

func errorResult(msg string) string {
	body, _ := json.Marshal(map[string]string{"error": msg})
	return string(body)
}

func truncate(s string) string {
	if len(s) <= maxResultBytes {
		return s
	}
	cut := maxResultBytes
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	body, _ := json.Marshal(map[string]string{"truncated_result": s[:cut], "note": fmt.Sprintf("показаны первые %d байт", cut)})
	return string(body)
}
