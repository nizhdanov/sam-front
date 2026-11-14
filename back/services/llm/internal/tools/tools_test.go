package tools

import (
	"context"
	"encoding/json"
	"testing"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/services/llm/internal/provider"
)

type fakeSource struct {
	calls int
	err   error
}

func (f *fakeSource) RunQuery(context.Context, string, json.RawMessage) (json.RawMessage, error) {
	f.calls++
	return json.RawMessage(`{"rows":[]}`), f.err
}

func TestCapabilitiesDoNotInventRequiredSoftwareBaseline(t *testing.T) {
	for _, role := range auth.UserRoles {
		source := &fakeSource{}
		result := NewExecutor(source).Run(context.Background(), role, provider.ToolCall{Name: "get_data_capabilities"})
		var out struct {
			Baseline       bool   `json:"required_software_baseline"`
			MissingReport  bool   `json:"missing_required_software_report"`
			LicenseBalance bool   `json:"license_balance"`
			Scope          string `json:"device_scope"`
			Explanation    string `json:"explanation"`
		}
		if err := json.Unmarshal([]byte(result), &out); err != nil {
			t.Fatal(err)
		}
		if out.Baseline || out.MissingReport || out.Explanation == "" || source.calls != 0 {
			t.Fatalf("invalid capabilities: %s", result)
		}
		if role == auth.RoleAdmin {
			if !out.LicenseBalance || out.Scope != "all_devices" {
				t.Fatalf("incorrect admin capabilities: %s", result)
			}
		} else if out.LicenseBalance || out.Scope != "own_device" {
			t.Fatalf("incorrect user capabilities: %s", result)
		}
	}
}

func TestToolErrorsDistinguishInvalidQueryFromForbidden(t *testing.T) {
	for _, tc := range []struct {
		role auth.Role
		err  error
		code string
	}{
		{auth.RoleAdmin, apperr.Invalid(`group_by "serial_number,product" is not supported`), "invalid_argument"},
		{auth.RoleUser, apperr.Forbidden("dataset is available to administrators only"), "forbidden"},
	} {
		source := &fakeSource{err: tc.err}
		result := NewExecutor(source).Run(context.Background(), tc.role, provider.ToolCall{Name: "query_data"})
		var out map[string]string
		if err := json.Unmarshal([]byte(result), &out); err != nil {
			t.Fatal(err)
		}
		if out["code"] != tc.code || out["error"] != apperr.Message(tc.err) {
			t.Fatalf("lost error semantics: %s", result)
		}
	}
}

func TestMalformedArgumentsAreNotReplacedWithUnfilteredQuery(t *testing.T) {
	source := &fakeSource{}
	result := NewExecutor(source).Run(context.Background(), auth.RoleAdmin, provider.ToolCall{Name: "query_data", Arguments: json.RawMessage(`{"filters":`)})
	var out map[string]string
	if err := json.Unmarshal([]byte(result), &out); err != nil {
		t.Fatal(err)
	}
	if out["code"] != "invalid_argument" || source.calls != 0 {
		t.Fatalf("malformed query executed: %s", result)
	}
}
