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

const maxResultBytes = 6000

type Definition struct {
	provider.Tool
	Roles []auth.Role
}

var (
	everyone   = auth.UserRoles
	privileged = auth.Privileged
	noParams   = json.RawMessage(`{"type":"object","properties":{}}`)
)

var registry = []Definition{
	{
		Tool: provider.Tool{
			Name:        "get_allowed_software",
			Description: "Каталог программ компании с корпоративным статусом (ALLOWED, FORBIDDEN, RESTRICTED, UNKNOWN). Можно отфильтровать по названию и статусу.",
			Parameters: json.RawMessage(`{"type":"object","properties":{
				"query":{"type":"string","description":"часть названия программы"},
				"policy_status":{"type":"string","enum":["ALLOWED","FORBIDDEN","RESTRICTED","UNKNOWN"]}}}`),
		},
		Roles: everyone,
	},
	{Tool: provider.Tool{Name: "get_my_devices", Description: "Устройства текущего пользователя.", Parameters: noParams}, Roles: everyone},
	{Tool: provider.Tool{Name: "get_my_software", Description: "Программы, установленные на устройствах текущего пользователя.", Parameters: noParams}, Roles: everyone},
	{Tool: provider.Tool{Name: "get_my_compliance", Description: "Нарушения на устройствах текущего пользователя.", Parameters: noParams}, Roles: everyone},
	{
		Tool: provider.Tool{
			Name:        "get_device_inventory",
			Description: "Устройство и установленное на нём ПО по серийному номеру.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"serial_number":{"type":"string"}},"required":["serial_number"]}`),
		},
		Roles: privileged,
	},
	{
		Tool: provider.Tool{
			Name:        "get_employee_inventory",
			Description: "ПО на устройствах сотрудника. Сотрудника можно найти по имени или email.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"имя или email сотрудника"}},"required":["query"]}`),
		},
		Roles: privileged,
	},
	{Tool: provider.Tool{Name: "get_paid_licenses", Description: "Действующие приобретённые лицензии: продукт, метрика, количество, срок действия.", Parameters: noParams}, Roles: privileged},
	{Tool: provider.Tool{Name: "get_license_costs", Description: "Стоимость действующих лицензий.", Parameters: noParams}, Roles: privileged},
	{
		Tool: provider.Tool{
			Name:        "get_license_balance",
			Description: "Лицензионный баланс: куплено, используется, свободно, дефицит. Можно указать продукт.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{"product":{"type":"string","description":"часть названия продукта"}}}`),
		},
		Roles: privileged,
	},
	{Tool: provider.Tool{Name: "get_company_compliance", Description: "Сводка нарушений по компании и наиболее серьёзные нарушения.", Parameters: noParams}, Roles: privileged},
	{Tool: provider.Tool{Name: "get_dashboard_summary", Description: "Общая статистика: устройства, ПО, лицензии, нарушения.", Parameters: noParams}, Roles: privileged},
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
