package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/opt"
)

type ProductStatus string

const (
	StatusActual    ProductStatus = "ACTUAL"
	StatusOutdated  ProductStatus = "OUTDATED"
	StatusForbidden ProductStatus = "FORBIDDEN"
)

func (p ProductStatus) Valid() bool {
	switch p {
	case StatusActual, StatusOutdated, StatusForbidden:
		return true
	}
	return false
}

type Lifecycle string

const (
	LifecycleCurrent     Lifecycle = "CURRENT"
	LifecycleOutdated    Lifecycle = "OUTDATED"
	LifecycleUnsupported Lifecycle = "UNSUPPORTED"
	LifecycleUnknown     Lifecycle = "UNKNOWN"
)

func (l Lifecycle) Valid() bool {
	switch l {
	case LifecycleCurrent, LifecycleOutdated, LifecycleUnsupported, LifecycleUnknown:
		return true
	}
	return false
}

type MatchStatus string

const (
	MatchMatched        MatchStatus = "MATCHED"
	MatchUnknown        MatchStatus = "UNKNOWN"
	MatchRequiresReview MatchStatus = "REQUIRES_REVIEW"
	MatchPending        MatchStatus = "PENDING_MATCH"
)

type MatchMethod string

const (
	MethodExact  MatchMethod = "EXACT"
	MethodFuzzy  MatchMethod = "FUZZY"
	MethodLLM    MatchMethod = "LLM"
	MethodManual MatchMethod = "MANUAL"
)

type Product struct {
	ID             uuid.UUID     `json:"id"`
	CanonicalName  string        `json:"canonical_name"`
	Publisher      *string       `json:"publisher"`
	Edition        *string       `json:"edition"`
	Purpose        *string       `json:"purpose"`
	Section        *string       `json:"section"`
	Class          *string       `json:"class"`
	InBPO          bool          `json:"in_bpo"`
	CurrentVersion *string       `json:"current_version"`
	AgreementID    *uuid.UUID    `json:"agreement_id"`
	AgreementName  *string       `json:"agreement_name"`
	LicenseType    *LicenseType  `json:"license_type"`
	CostModel      *CostModel    `json:"cost_model"`
	Status         ProductStatus `json:"status"`
	StatusComment  *string       `json:"status_comment"`
	Purchased      int           `json:"purchased_seats"`
	Installations  int           `json:"active_installations"`
	Devices        int           `json:"devices"`
	ArchivedAt     *time.Time    `json:"archived_at"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at"`
}

type PublicProduct struct {
	ID             uuid.UUID     `json:"id"`
	CanonicalName  string        `json:"canonical_name"`
	Publisher      *string       `json:"publisher"`
	Edition        *string       `json:"edition"`
	Purpose        *string       `json:"purpose"`
	Section        *string       `json:"section"`
	Class          *string       `json:"class"`
	InBPO          bool          `json:"in_bpo"`
	CurrentVersion *string       `json:"current_version"`
	LicenseType    *LicenseType  `json:"license_type"`
	CostModel      *CostModel    `json:"cost_model"`
	Status         ProductStatus `json:"status"`
	StatusComment  *string       `json:"status_comment"`
}

func (p Product) Public() PublicProduct {
	return PublicProduct{
		ID:             p.ID,
		CanonicalName:  p.CanonicalName,
		Publisher:      p.Publisher,
		Edition:        p.Edition,
		Purpose:        p.Purpose,
		Section:        p.Section,
		Class:          p.Class,
		InBPO:          p.InBPO,
		CurrentVersion: p.CurrentVersion,
		LicenseType:    p.LicenseType,
		CostModel:      p.CostModel,
		Status:         p.Status,
		StatusComment:  p.StatusComment,
	}
}

type ProductFilter struct {
	Search          string
	Status          ProductStatus
	Section         string
	Class           string
	LicenseType     LicenseType
	CostModel       CostModel
	InBPO           *bool
	IncludeArchived bool
	PublicOnly      bool
}

type ProductPatch struct {
	CanonicalName  *string
	Publisher      *string
	Edition        *string
	Purpose        *string
	Section        *string
	Class          *string
	InBPO          *bool
	CurrentVersion opt.Field[string]
	AgreementID    opt.Field[uuid.UUID]
	Status         *ProductStatus
	StatusComment  *string
	Archived       *bool
}

func (p ProductPatch) Validate() error {
	if p.CanonicalName != nil && strings.TrimSpace(*p.CanonicalName) == "" {
		return apperr.Invalid("canonical_name must not be empty")
	}
	if p.Status != nil && !p.Status.Valid() {
		return apperr.Invalid("unknown status")
	}
	return nil
}

type Version struct {
	ID              uuid.UUID  `json:"id"`
	ProductID       uuid.UUID  `json:"product_id"`
	Version         string     `json:"version"`
	LifecycleStatus Lifecycle  `json:"lifecycle_status"`
	ReleaseDate     *time.Time `json:"release_date"`
	EndOfSupport    *time.Time `json:"end_of_support"`
	CreatedAt       time.Time  `json:"created_at"`
}

type Alias struct {
	ID           uuid.UUID   `json:"id"`
	SourceKey    string      `json:"source_key"`
	RawName      string      `json:"raw_name"`
	RawPublisher *string     `json:"raw_publisher"`
	RawVersion   *string     `json:"raw_version"`
	ProductID    uuid.UUID   `json:"product_id"`
	ProductName  string      `json:"product_name"`
	VersionID    *uuid.UUID  `json:"version_id"`
	MatchMethod  MatchMethod `json:"match_method"`
	ReviewedBy   *string     `json:"reviewed_by"`
	CreatedAt    time.Time   `json:"created_at"`
}

type RuleFilter struct {
	Search    string
	ProductID *uuid.UUID
}
