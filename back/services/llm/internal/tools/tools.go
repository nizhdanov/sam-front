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
	admins = []auth.Role{auth.RoleAdmin}
	users  = []auth.Role{auth.RoleUser}
)

const adminQueryDescription = `Универсальный запрос к данным SAM. Возвращает таблицу {columns, rows, total, shown}.
Наборы данных (dataset):
- installations — фактические установки ПО на устройствах: device_id, serial_number, device_type, raw_name (имя из ОС), product (продукт каталога), publisher, version, current_version (актуальная версия), match_status, status (ACTUAL, OUTDATED, FORBIDDEN), license_type, cost_model (FREE, PAID, SHAREWARE), first_seen_at, last_seen_at, license_status (COVERED, DEFICIT n, NOT_REQUIRED, UNVERIFIED, UNMATCHED). Фильтры: product, product_id, raw_name, publisher, version, serial_number, device_type, match_status, recognized (true/false), status, license_type, cost_model, in_bpo. group_by: product, publisher, version, serial_number, device_type, match_status, status, license_type, cost_model (считает устройства).
- devices — устройства: device_id, serial_number, device_type (PHYSICAL, VIRTUAL), status (NEW, ACTIVE), hostname, responsible, location, last_inventory_at, installations, open_findings. Фильтры: serial_number, hostname, responsible, location, device_type, status. group_by: device_type, status, location.
- products — каталог: product_id, product, publisher, current_version, section, class, in_bpo, status, status_comment, license_type, cost_model, purchased (действующих мест), devices, license_status. Фильтры: product, product_id, publisher, section, class, status, license_type, cost_model, in_bpo. group_by: publisher, section, class, status, license_type, cost_model, in_bpo.
- contracts — договоры закупки: contract_id, product_id, product, contract_number, metric, quantity, valid_from, expires_at, status (ACTIVE, EXPIRING, EXPIRED, INACTIVE, NOT_STARTED). Фильтры: product, product_id, contract_number, metric, status. group_by: product, metric, status (суммирует quantity).
- findings — нарушения: finding_id, product_id, device_id, finding_type (FORBIDDEN_SOFTWARE, OUTDATED_SOFTWARE, UNSUPPORTED_SOFTWARE, LICENSE_DEFICIT, LICENSE_EXPIRED, LICENSE_UNVERIFIED, UNKNOWN_SOFTWARE, COMMERCIAL_USE_RESTRICTED), severity, status (OPEN, ACKNOWLEDGED), serial_number, raw_name, product, version, current_version, detected_at, comment. Фильтры: finding_type, severity, status, serial_number, product, product_id. group_by: finding_type, severity, status, serial_number, product.
Текстовые фильтры ищут по подстроке, статусы и типы — точное значение.`

const userQueryDescription = `Запрос к данным SAM, доступным пользователю. Возвращает таблицу {columns, rows, total, shown}.
Наборы данных (dataset):
- products — справочник разрешённого бесплатного ПО (перечень БПО) и запрещённого ПО: product_id, product, publisher, current_version (рекомендуемая актуальная версия), section, class, in_bpo, status (ACTUAL, OUTDATED, FORBIDDEN), status_comment (причина запрета), license_type, cost_model. Фильтры: product, publisher, section, class, status, license_type, cost_model, in_bpo.
- installations — ПО, установленное на компьютере пользователя: serial_number, raw_name, product, version, current_version, status. Фильтры: product, raw_name, status, recognized.
- findings — запрещённое и устаревшее ПО на компьютере пользователя: finding_type, raw_name, product, version, current_version, detected_at. Фильтры: finding_type, product.
Текстовые фильтры ищут по подстроке, статусы и типы — точное значение.`

var registry = []Definition{
	{
		Tool: provider.Tool{Name: "query_data", Description: adminQueryDescription, Parameters: json.RawMessage(`{"type":"object","properties":{
		"dataset":{"type":"string","enum":["installations","devices","products","contracts","findings"]},
		"filters":{"type":"object","description":"фильтры выбранного набора данных; значения — строки","properties":{"product":{"type":"string"},"product_id":{"type":"string"},"raw_name":{"type":"string"},"publisher":{"type":"string"},"version":{"type":"string"},"serial_number":{"type":"string"},"device_type":{"type":"string"},"match_status":{"type":"string"},"recognized":{"type":"string"},"status":{"type":"string"},"license_type":{"type":"string"},"cost_model":{"type":"string"},"in_bpo":{"type":"string"},"section":{"type":"string"},"class":{"type":"string"},"hostname":{"type":"string"},"responsible":{"type":"string"},"location":{"type":"string"},"contract_number":{"type":"string"},"metric":{"type":"string"},"finding_type":{"type":"string"},"severity":{"type":"string"}}},
		"group_by":{"type":"string","description":"поле для группировки и подсчёта"},
		"limit":{"type":"integer","minimum":1,"maximum":100}},
		"required":["dataset"]}`)},
		Roles: admins,
	},
	{
		Tool: provider.Tool{Name: "query_data", Description: userQueryDescription, Parameters: json.RawMessage(`{"type":"object","properties":{
		"dataset":{"type":"string","enum":["installations","products","findings"]},
		"filters":{"type":"object","description":"фильтры выбранного набора данных; значения — строки","properties":{"product":{"type":"string"},"product_id":{"type":"string"},"raw_name":{"type":"string"},"publisher":{"type":"string"},"version":{"type":"string"},"serial_number":{"type":"string"},"device_type":{"type":"string"},"match_status":{"type":"string"},"recognized":{"type":"string"},"status":{"type":"string"},"license_type":{"type":"string"},"cost_model":{"type":"string"},"in_bpo":{"type":"string"},"section":{"type":"string"},"class":{"type":"string"},"hostname":{"type":"string"},"responsible":{"type":"string"},"location":{"type":"string"},"contract_number":{"type":"string"},"metric":{"type":"string"},"finding_type":{"type":"string"},"severity":{"type":"string"}}},
		"group_by":{"type":"string","description":"поле для группировки и подсчёта"},
		"limit":{"type":"integer","minimum":1,"maximum":100}},
		"required":["dataset"]}`)},
		Roles: users,
	},
	{
		Tool: provider.Tool{
			Name:        "get_license_balance",
			Description: "Лицензионный баланс «куплено vs установлено» по продуктам: куплено мест по действующим договорам, установлено, свободно (простаивает), дефицит и итоговый дефицит. Можно указать продукт и только дефицитные позиции.",
			Parameters: json.RawMessage(`{"type":"object","properties":{
				"product":{"type":"string","description":"часть названия продукта"},
				"only_deficit":{"type":"boolean"}}}`),
		},
		Roles: admins,
	},
	{
		Tool: provider.Tool{
			Name:        "get_summary",
			Description: "KPI парка: устройства, установки, процент распознанного ПО, дефицит лицензий, простаивающие лицензии, запрещённое и устаревшее ПО, истекающие договоры.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		Roles: admins,
	},
	{
		Tool: provider.Tool{
			Name:        "get_my_device",
			Description: "Карточка компьютера пользователя: серийный номер, тип, количество установленного ПО и открытых нарушений.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		Roles: users,
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
	return slices.ContainsFunc(registry, func(d Definition) bool {
		return d.Name == name && slices.Contains(d.Roles, role)
	})
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
