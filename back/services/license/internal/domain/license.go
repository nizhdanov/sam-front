package domain

import (
	"time"

	"github.com/google/uuid"

	"sam/pkg/opt"
)

type LicenseType string

const (
	LicenseOpenSource LicenseType = "OPEN_SOURCE"
	LicenseCommercial LicenseType = "COMMERCIAL"
	LicenseTrial      LicenseType = "TRIAL"
)

func (t LicenseType) Valid() bool {
	switch t {
	case LicenseOpenSource, LicenseCommercial, LicenseTrial:
		return true
	}
	return false
}

type CostModel string

const (
	CostFree      CostModel = "FREE"
	CostPaid      CostModel = "PAID"
	CostShareware CostModel = "SHAREWARE"
)

func (c CostModel) Valid() bool {
	switch c {
	case CostFree, CostPaid, CostShareware:
		return true
	}
	return false
}

func (c CostModel) RequiresEntitlement() bool { return c == CostPaid || c == CostShareware }

type Metric string

const (
	MetricPerDevice  Metric = "PER_DEVICE"
	MetricUnlimited  Metric = "UNLIMITED"
	MetricConcurrent Metric = "CONCURRENT"
	MetricUnknown    Metric = "UNKNOWN"
)

func (m Metric) Valid() bool {
	switch m {
	case MetricPerDevice, MetricUnlimited, MetricConcurrent, MetricUnknown:
		return true
	}
	return false
}

type Entitlement struct {
	ID                uuid.UUID  `json:"id"`
	ProductID         uuid.UUID  `json:"product_id"`
	ProductName       string     `json:"product_name"`
	AgreementID       *uuid.UUID `json:"agreement_id"`
	Metric            Metric     `json:"license_metric"`
	Quantity          int        `json:"quantity"`
	ValidFrom         *time.Time `json:"valid_from"`
	ExpiresAt         *time.Time `json:"expires_at"`
	PurchaseReference *string    `json:"purchase_reference"`
	IsActive          bool       `json:"is_active"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (e Entitlement) ValidAt(t time.Time) bool {
	if !e.IsActive {
		return false
	}
	if e.ValidFrom != nil && e.ValidFrom.After(t) {
		return false
	}
	return e.ExpiresAt == nil || e.ExpiresAt.After(t)
}

func (e Entitlement) ExpiredAt(t time.Time) bool {
	return e.IsActive && e.ExpiresAt != nil && !e.ExpiresAt.After(t)
}

type ContractFilter struct {
	ProductID      *uuid.UUID
	ActiveOnly     bool
	ExpiringWithin int
}

type EntitlementPatch struct {
	ProductID         *uuid.UUID
	AgreementID       opt.Field[uuid.UUID]
	Metric            *Metric
	Quantity          *int
	ValidFrom         opt.Field[time.Time]
	ExpiresAt         opt.Field[time.Time]
	PurchaseReference opt.Field[string]
}

type Agreement struct {
	ID                uuid.UUID   `json:"id"`
	Name              string      `json:"name"`
	LicenseType       LicenseType `json:"license_type"`
	CostModel         CostModel   `json:"cost_model"`
	AgreementText     *string     `json:"agreement_text,omitempty"`
	FileName          *string     `json:"file_name"`
	SourceURL         *string     `json:"source_url"`
	CommercialAllowed *bool       `json:"commercial_allowed"`
	ReviewStatus      string      `json:"review_status"`
	Products          int         `json:"products_count"`
	CreatedAt         time.Time   `json:"created_at"`
	UpdatedAt         time.Time   `json:"updated_at"`
}

type AgreementPatch struct {
	Name              *string
	LicenseType       *LicenseType
	CostModel         *CostModel
	AgreementText     opt.Field[string]
	FileName          opt.Field[string]
	SourceURL         opt.Field[string]
	CommercialAllowed opt.Field[bool]
	ReviewStatus      *string
}

type SimilarAgreement struct {
	ID          uuid.UUID   `json:"id"`
	Name        string      `json:"name"`
	LicenseType LicenseType `json:"license_type"`
	CostModel   CostModel   `json:"cost_model"`
	Similarity  float64     `json:"similarity"`
}

type AgreementAnalysis struct {
	ID            uuid.UUID  `json:"id"`
	AgreementID   uuid.UUID  `json:"agreement_id"`
	ContentHash   string     `json:"content_hash"`
	Model         string     `json:"model"`
	PromptVersion string     `json:"prompt_version"`
	Analysis      []byte     `json:"-"`
	Status        string     `json:"status"`
	ReviewedBy    *string    `json:"reviewed_by"`
	ReviewedAt    *time.Time `json:"reviewed_at"`
	CreatedAt     time.Time  `json:"created_at"`
}
