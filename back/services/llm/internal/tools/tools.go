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
- products — каталог: product_id, product, publisher, current_version, purpose (назначение), section, class, in_bpo, status, status_comment, license_type, cost_model, purchased (действующих мест), devices, license_status. Фильтры: product, product_id, publisher, purpose, section, class, status, license_type, cost_model, in_bpo. group_by: publisher, section, class, status, license_type, cost_model, in_bpo.
- contracts — договоры закупки: contract_id, product_id, product, contract_number, metric, quantity, valid_from, expires_at, status (ACTIVE, EXPIRING, EXPIRED, INACTIVE, NOT_STARTED). Фильтры: product, product_id, contract_number, metric, status. group_by: product, metric, status (суммирует quantity).
- findings — нарушения: finding_id, product_id, device_id, finding_type (FORBIDDEN_SOFTWARE — запрещённое ПО, OUTDATED_SOFTWARE — версия ниже актуальной, LICENSE_DEFICIT — установлено больше, чем куплено), severity, status (OPEN, ACKNOWLEDGED), serial_number, raw_name, product, version, current_version, detected_at, comment. Фильтры: finding_type, severity, status, serial_number, product, product_id. group_by: finding_type, severity, status, serial_number, product.
Текстовые фильтры ищут по подстроке, статусы и типы — точное значение.
Для таблицы с несколькими колонками НЕ указывай group_by: все колонки уже есть в строках.
group_by — ровно одно из перечисленных для dataset полей, а не список колонок. license_status — вычисленное поле ответа, фильтра и группировки по нему нет.
Нет данных об обязательном ПО для рабочих мест; этот запрос показывает только фактические установки и не определяет недостающие программы.`

const userQueryDescription = `Запрос к данным SAM, доступным пользователю. Возвращает таблицу {columns, rows, total, shown}.
Наборы данных (dataset):
- products — справочник разрешённого бесплатного ПО (перечень БПО) и запрещённого ПО: product_id, product, publisher, current_version (рекомендуемая актуальная версия), purpose (назначение), section, class, in_bpo, status (ACTUAL, OUTDATED, FORBIDDEN), status_comment (причина запрета), license_type, cost_model. Фильтры: product, publisher, purpose, section, class, status, license_type, cost_model, in_bpo.
- installations — ПО, установленное на компьютере пользователя: serial_number, raw_name, product, version, current_version, status. Фильтры: product, raw_name, status, recognized.
- findings — запрещённое и устаревшее ПО на компьютере пользователя: finding_type, raw_name, product, version, current_version, detected_at. Фильтры: finding_type, product.
Текстовые фильтры ищут по подстроке, статусы и типы — точное значение.
Для таблицы с несколькими колонками не указывай group_by. Перечня обязательных программ по рабочим местам в системе нет.`

var registry = []Definition{
	{
		Tool: provider.Tool{
			Name:        "get_data_capabilities",
			Description: "Какие данные и отчёты поддерживает SAM. Вызывай для вопросов об отсутствующем или обязательном ПО и при неясности, можно ли построить отчёт. Не возвращает пользовательские данные.",
			Parameters:  json.RawMessage(`{"type":"object","properties":{}}`),
		},
		Roles: auth.UserRoles,
	},
	{
		Tool: provider.Tool{Name: "query_data", Description: adminQueryDescription, Parameters: json.RawMessage(`{"type":"object","properties":{
		"dataset":{"type":"string","enum":["installations","devices","products","contracts","findings"]},
		"filters":{"type":"object","description":"фильтры выбранного набора данных; значения — строки","properties":{"product":{"type":"string"},"product_id":{"type":"string"},"raw_name":{"type":"string"},"publisher":{"type":"string"},"purpose":{"type":"string"},"version":{"type":"string"},"serial_number":{"type":"string"},"device_type":{"type":"string"},"match_status":{"type":"string"},"recognized":{"type":"string"},"status":{"type":"string"},"license_type":{"type":"string"},"cost_model":{"type":"string"},"in_bpo":{"type":"string"},"section":{"type":"string"},"class":{"type":"string"},"hostname":{"type":"string"},"responsible":{"type":"string"},"location":{"type":"string"},"contract_number":{"type":"string"},"metric":{"type":"string"},"finding_type":{"type":"string"},"severity":{"type":"string"}}},
		"group_by":{"type":"string","description":"Одно разрешённое для dataset поле для подсчёта. Для обычной таблицы не передавай. Нельзя список колонок или license_status."},
		"limit":{"type":"integer","minimum":1,"maximum":100}},
		"required":["dataset"]}`)},
		Roles: admins,
	},
	{
		Tool: provider.Tool{Name: "query_data", Description: userQueryDescription, Parameters: json.RawMessage(`{"type":"object","properties":{
		"dataset":{"type":"string","enum":["installations","products","findings"]},
		"filters":{"type":"object","description":"фильтры выбранного набора данных; значения — строки","properties":{"product":{"type":"string"},"product_id":{"type":"string"},"raw_name":{"type":"string"},"publisher":{"type":"string"},"purpose":{"type":"string"},"version":{"type":"string"},"serial_number":{"type":"string"},"device_type":{"type":"string"},"match_status":{"type":"string"},"recognized":{"type":"string"},"status":{"type":"string"},"license_type":{"type":"string"},"cost_model":{"type":"string"},"in_bpo":{"type":"string"},"section":{"type":"string"},"class":{"type":"string"},"hostname":{"type":"string"},"responsible":{"type":"string"},"location":{"type":"string"},"contract_number":{"type":"string"},"metric":{"type":"string"},"finding_type":{"type":"string"},"severity":{"type":"string"}}},
		"group_by":{"type":"string","description":"Одно разрешённое для dataset поле для подсчёта. Для обычной таблицы не передавай. Нельзя список колонок или license_status."},
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
		return errorResult(apperr.Forbidden("инструмент недоступен для роли пользователя"))
	}
	if call.Name == "get_data_capabilities" {
		return dataCapabilities(role)
	}
	args := call.Arguments
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	if !json.Valid(args) {
		return errorResult(apperr.Invalid("arguments must be valid JSON; correct the arguments and retry"))
	}
	result, err := e.data.RunQuery(ctx, call.Name, args)
	if err != nil {
		return errorResult(err)
	}
	return truncate(string(result))
}

func errorResult(err error) string {
	body, _ := json.Marshal(map[string]string{"error": apperr.Message(err), "code": apperr.KindOf(err).String()})
	return string(body)
}

func dataCapabilities(role auth.Role) string {
	body, _ := json.Marshal(map[string]any{
		"installed_software":               true,
		"device_scope":                     map[bool]string{true: "all_devices", false: "own_device"}[role == auth.RoleAdmin],
		"license_balance":                  role == auth.RoleAdmin,
		"required_software_baseline":       false,
		"missing_required_software_report": false,
		"explanation":                      "В системе есть фактические установки ПО, но нет перечня обязательных программ по рабочим местам. Поэтому определить недостающее ПО без заданного пользователем списка нельзя. Каталог и перечень БПО не означают, что все эти программы обязательны на каждом ПК. Дефицит лицензий — отдельный отчёт по уже установленным продуктам; он не указывает конкретные нелицензированные ПК.",
		"table_query":                      "Для таблицы фактических установок используй query_data с dataset=installations без group_by. group_by предназначен только для подсчёта по одному разрешённому полю.",
	})
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
