package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const demoNote = "[DEMO] Ответ сформирован тестовым провайдером без обращения к языковой модели."

type Mock struct{}

func NewMock() *Mock { return &Mock{} }

func (m *Mock) Name() string { return "mock" }

func (m *Mock) Complete(_ context.Context, req Request) (Response, error) {
	resp := Response{Model: "mock"}
	switch req.Task {
	case "matching":
		resp.Content = `{"status":"REQUIRES_REVIEW","product_id":null,"confidence":0,"reason":"[DEMO] mock provider does not perform AI matching"}`
	case "eula":
		resp.Content = mockEULA(lastContent(req.Messages, RoleUser))
	default:
		resp.Content, resp.ToolCalls = mockChat(req)
	}
	resp.Usage = Usage{PromptTokens: approxTokens(req.Messages), CompletionTokens: len(resp.Content) / 4}
	return resp, nil
}

func (m *Mock) Stream(ctx context.Context, req Request) (<-chan StreamEvent, error) {
	resp, err := m.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	events := make(chan StreamEvent)
	go func() {
		defer close(events)
		for _, word := range strings.SplitAfter(resp.Content, " ") {
			select {
			case events <- StreamEvent{Delta: word}:
			case <-ctx.Done():
				return
			}
		}
		select {
		case events <- StreamEvent{Usage: resp.Usage}:
		case <-ctx.Done():
		}
	}()
	return events, nil
}

var toolKeywords = []struct {
	tool     string
	keywords []string
	args     string
}{
	{"get_license_costs", []string{"стоим", "цен", "cost", "price"}, `{}`},
	{"get_license_balance", []string{"баланс", "дефицит", "хватает", "balance", "deficit"}, `{}`},
	{"get_paid_licenses", []string{"платн", "куплен", "paid"}, `{}`},
	{"get_company_compliance", []string{"нарушен", "комплаенс", "compliance", "violation"}, `{}`},
	{"get_my_compliance", []string{"нарушен", "проблем", "compliance"}, `{}`},
	{"get_dashboard_summary", []string{"сводк", "статистик", "dashboard", "summary"}, `{}`},
	{"get_my_software", []string{"мои программ", "у меня установ", "my software"}, `{}`},
	{"get_my_devices", []string{"мой компьютер", "мои устройств", "my device"}, `{}`},
	{"get_allowed_software", []string{"запрещ", "forbidden"}, `{"policy_status":"FORBIDDEN"}`},
	{"get_allowed_software", []string{"разреш", "можно установ", "allowed", "каталог"}, `{"policy_status":"ALLOWED"}`},
}

func mockChat(req Request) (string, []ToolCall) {
	last := req.Messages[len(req.Messages)-1]
	if last.Role == RoleTool {
		data := last.Content
		if len(data) > 1500 {
			data = data[:1500] + "…"
		}
		return fmt.Sprintf("%s\n\nДанные инструмента %s:\n%s", demoNote, last.Name, data), nil
	}

	question := strings.ToLower(last.Content)
	available := make([]string, 0, len(req.Tools))
	for _, t := range req.Tools {
		available = append(available, t.Name)
	}
	for _, rule := range toolKeywords {
		if !slices.Contains(available, rule.tool) {
			continue
		}
		for _, kw := range rule.keywords {
			if strings.Contains(question, kw) {
				return "", []ToolCall{{ID: "mock-call", Name: rule.tool, Arguments: json.RawMessage(rule.args)}}
			}
		}
	}
	return demoNote + "\n\nЯ могу рассказать о разрешённом ПО, ваших устройствах и нарушениях лицензирования. Уточните вопрос.", nil
}

func mockEULA(text string) string {
	lower := strings.ToLower(text)
	contains := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(lower, w) {
				return true
			}
		}
		return false
	}
	var commercial *bool
	switch {
	case contains("non-commercial", "noncommercial", "personal use only", "некоммерч"):
		f := false
		commercial = &f
	case contains("commercial use is permitted", "commercial use permitted"):
		t := true
		commercial = &t
	}
	result := map[string]any{
		"license_type":           "UNKNOWN",
		"commercial_use_allowed": commercial,
		"redistribution_allowed": nil,
		"requires_subscription":  contains("subscription", "подписк"),
		"device_limit":           nil,
		"user_limit":             nil,
		"restrictions":           []any{},
		"risk_level":             "MEDIUM",
		"requires_human_review":  true,
		"summary":                "[DEMO] keyword-based analysis by the mock provider, not a legal review",
	}
	body, _ := json.Marshal(result)
	return string(body)
}

func lastContent(messages []Message, role Role) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == role {
			return messages[i].Content
		}
	}
	return ""
}

func approxTokens(messages []Message) int {
	n := 0
	for _, m := range messages {
		n += len(m.Content) / 4
	}
	return n
}
