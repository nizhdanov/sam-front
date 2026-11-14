package domain

import (
	"time"

	"github.com/google/uuid"

	"sam/pkg/opt"
)

type Employee struct {
	ID         uuid.UUID `json:"id"`
	ExternalID *string   `json:"external_id"`
	FullName   string    `json:"full_name"`
	Department *string   `json:"department"`
	Position   *string   `json:"position"`
	Email      *string   `json:"email"`
	IsActive   bool      `json:"is_active"`
	Devices    int       `json:"devices_count"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type EmployeePatch struct {
	FullName   *string
	Department *string
	Position   *string
	Email      *string
	IsActive   *bool
}

type Device struct {
	ID              uuid.UUID  `json:"id"`
	SerialNumber    string     `json:"serial_number"`
	Hostname        *string    `json:"hostname"`
	EmployeeID      *uuid.UUID `json:"employee_id"`
	EmployeeName    *string    `json:"employee_name"`
	Department      *string    `json:"department"`
	OperatingSystem *string    `json:"operating_system"`
	LastInventoryAt *time.Time `json:"last_inventory_at"`
	Installations   int        `json:"installations_count"`
	OpenFindings    int        `json:"open_findings_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type DeviceFilter struct {
	Search        string
	SerialNumber  string
	EmployeeID    *uuid.UUID
	Department    string
	HasViolations *bool
}

type DevicePatch struct {
	Hostname        *string
	OperatingSystem *string
	EmployeeID      opt.Field[uuid.UUID]
}

type Installation struct {
	ID              uuid.UUID   `json:"id"`
	DeviceID        uuid.UUID   `json:"device_id"`
	SerialNumber    string      `json:"serial_number"`
	ProductID       *uuid.UUID  `json:"product_id"`
	ProductName     *string     `json:"product_name"`
	PolicyStatus    *string     `json:"policy_status"`
	VersionID       *uuid.UUID  `json:"version_id"`
	Version         *string     `json:"version"`
	LifecycleStatus *string     `json:"lifecycle_status"`
	RawName         string      `json:"raw_name"`
	RawPublisher    *string     `json:"raw_publisher"`
	RawVersion      *string     `json:"raw_version"`
	MatchStatus     MatchStatus `json:"match_status"`
	IsActive        bool        `json:"is_active"`
	FirstSeenAt     time.Time   `json:"first_seen_at"`
	LastSeenAt      time.Time   `json:"last_seen_at"`
	RemovedAt       *time.Time  `json:"removed_at"`
}

type InstallationFilter struct {
	DeviceID       *uuid.UUID
	ProductID      *uuid.UUID
	EmployeeID     *uuid.UUID
	MatchStatus    MatchStatus
	IncludeRemoved bool
}

type SyncResult string

const (
	SyncApplied          SyncResult = "APPLIED"
	SyncAlreadyProcessed SyncResult = "ALREADY_PROCESSED"
	SyncSuperseded       SyncResult = "SUPERSEDED"
)

type InstallationInput struct {
	SourceKey    string
	RawName      string
	RawPublisher string
	RawVersion   string
	ProductID    *uuid.UUID
	VersionID    *uuid.UUID
	MatchStatus  MatchStatus
}

type Snapshot struct {
	InventoryID  uuid.UUID
	ImportID     uuid.UUID
	SerialNumber string
	CollectedAt  time.Time
	FullSnapshot bool
	Items        []InstallationInput
}

type SyncOutcome struct {
	Result              SyncResult
	DeviceID            uuid.UUID
	ActiveInstallations int
}
