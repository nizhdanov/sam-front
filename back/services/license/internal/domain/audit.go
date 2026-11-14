package domain

import "time"

type AuditEntry struct {
	ID        int64          `json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	Actor     string         `json:"actor"`
	Entity    string         `json:"entity"`
	EntityID  string         `json:"entity_id,omitempty"`
	Action    string         `json:"action"`
	Summary   string         `json:"summary"`
	Changes   map[string]any `json:"changes,omitempty"`
}

type AuditFilter struct {
	Actor  string
	Entity string
	From   *time.Time
	To     *time.Time
}

const (
	AuditCreate  = "CREATE"
	AuditUpdate  = "UPDATE"
	AuditDelete  = "DELETE"
	AuditApprove = "APPROVE"
	AuditReject  = "REJECT"
)
