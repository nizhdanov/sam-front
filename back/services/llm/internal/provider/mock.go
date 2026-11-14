package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
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
	case "catalog":
		resp.Content = mockCatalog(lastContent(req.Messages, RoleUser))
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
	{"get_license_balance", []string{"баланс", "дефицит", "простаива", "хватает", "balance", "deficit"}, `{}`},
	{"query_data", []string{"запрещ", "forbidden"}, `{"dataset":"findings","filters":{"finding_type":"FORBIDDEN_SOFTWARE"}}`},
	{"query_data", []string{"устарев", "outdated"}, `{"dataset":"findings","filters":{"finding_type":"OUTDATED_SOFTWARE"}}`},
	{"query_data", []string{"договор", "contract"}, `{"dataset":"contracts"}`},
	{"query_data", []string{"бесплат", "бпо", "free"}, `{"dataset":"products","filters":{"cost_model":"FREE"}}`},
	{"query_data", []string{"нарушен", "compliance"}, `{"dataset":"findings"}`},
	{"query_data", []string{"устройств", "компьютер", "device"}, `{"dataset":"devices"}`},
	{"query_data", []string{"установ", "программ", "software"}, `{"dataset":"installations"}`},
	{"get_summary", []string{"сводк", "статистик", "kpi", "summary"}, `{}`},
	{"get_my_device", []string{"мой компьютер", "моё устройство", "my device"}, `{}`},
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
	paragraphs := strings.Split(text, "\n")
	find := func(words ...string) []int {
		var out []int
		for _, p := range paragraphs {
			number, body, ok := strings.Cut(strings.TrimPrefix(p, "["), "]")
			n, err := strconv.Atoi(number)
			if !ok || err != nil {
				continue
			}
			lower := strings.ToLower(body)
			for _, w := range words {
				if strings.Contains(lower, w) {
					out = append(out, n)
					break
				}
			}
		}
		return out
	}
	commercial := map[string]any{"value": "UNDEFINED", "paragraphs": []int{}, "quote": ""}
	if refs := find("non-commercial", "noncommercial", "personal use only", "некоммерч"); len(refs) > 0 {
		commercial = map[string]any{"value": "NO", "paragraphs": refs, "quote": ""}
	} else if refs := find("commercial use is permitted", "commercial use permitted"); len(refs) > 0 {
		commercial = map[string]any{"value": "YES", "paragraphs": refs, "quote": ""}
	}
	subscriptions := []any{}
	if refs := find("subscription", "подписк"); len(refs) > 0 {
		subscriptions = append(subscriptions, map[string]any{"kind": "HIDDEN_SUBSCRIPTION", "description": "упоминается подписка", "paragraphs": refs, "quote": ""})
	}
	if refs := find("renew", "продлева"); len(refs) > 0 {
		subscriptions = append(subscriptions, map[string]any{"kind": "AUTO_RENEWAL", "description": "упоминается продление", "paragraphs": refs, "quote": ""})
	}
	result := map[string]any{
		"license_type":          nil,
		"cost_model":            nil,
		"license_type_evidence": map[string]any{"paragraphs": []int{}, "quote": ""},
		"commercial_use":        commercial,
		"subscriptions":         subscriptions,
		"restrictions":          []any{},
		"risks":                 []any{},
		"summary":               "[DEMO] анализ по ключевым словам mock-провайдера, не юридическое заключение",
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

var mockVersionTail = regexp.MustCompile(`(?i)\s*([\(\[]?(x64|x86|64-bit|32-bit)[\)\]]?|\d+([.\-]\d+)*|v\d.*)(\s|$).*$`)

func mockCatalog(payload string) string {
	var items []struct {
		Key         string `json:"key"`
		DisplayName string `json:"display_name"`
		Publisher   string `json:"publisher"`
	}
	_ = json.Unmarshal([]byte(payload), &items)
	out := make([]map[string]any, 0, len(items))
	for _, it := range items {
		name := strings.TrimSpace(mockVersionTail.ReplaceAllString(it.DisplayName, ""))
		if name == "" {
			name = it.DisplayName
		}
		out = append(out, map[string]any{"key": it.Key, "canonical_name": name, "publisher": it.Publisher, "is_software": true})
	}
	body, _ := json.Marshal(map[string]any{"items": out})
	return string(body)
}
