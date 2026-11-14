package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"sam/pkg/opt"
)

type DeviceType string

const (
	DevicePhysical DeviceType = "PHYSICAL"
	DeviceVirtual  DeviceType = "VIRTUAL"
)

func (t DeviceType) Valid() bool { return t == DevicePhysical || t == DeviceVirtual }

func DetectDeviceType(serial string) DeviceType {
	s := strings.ToLower(serial)
	for _, prefix := range []string{"vmware", "virtualbox", "parallels"} {
		if strings.HasPrefix(s, prefix) {
			return DeviceVirtual
		}
	}
	return DevicePhysical
}

type DeviceStatus string

const (
	DeviceNew    DeviceStatus = "NEW"
	DeviceActive DeviceStatus = "ACTIVE"
)

func (s DeviceStatus) Valid() bool { return s == DeviceNew || s == DeviceActive }

type Device struct {
	ID              uuid.UUID    `json:"id"`
	SerialNumber    string       `json:"serial_number"`
	Type            DeviceType   `json:"device_type"`
	Status          DeviceStatus `json:"status"`
	Hostname        *string      `json:"hostname"`
	OperatingSystem *string      `json:"operating_system"`
	Responsible     *string      `json:"responsible"`
	Location        *string      `json:"location"`
	LastInventoryAt *time.Time   `json:"last_inventory_at"`
	Installations   int          `json:"installations_count"`
	OpenFindings    int          `json:"open_findings_count"`
	CreatedAt       time.Time    `json:"created_at"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

type DevicePatch struct {
	Type        *DeviceType
	Status      *DeviceStatus
	Hostname    opt.Field[string]
	Responsible opt.Field[string]
	Location    opt.Field[string]
}

type DeviceImport struct {
	InventoryID uuid.UUID  `json:"inventory_id"`
	CollectedAt time.Time  `json:"collected_at"`
	Result      SyncResult `json:"result"`
	ProcessedAt time.Time  `json:"processed_at"`
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
