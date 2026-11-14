package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/opt"
)

type PolicyStatus string

const (
	PolicyAllowed    PolicyStatus = "ALLOWED"
	PolicyForbidden  PolicyStatus = "FORBIDDEN"
	PolicyRestricted PolicyStatus = "RESTRICTED"
	PolicyUnknown    PolicyStatus = "UNKNOWN"
)

func (p PolicyStatus) Valid() bool {
	switch p {
	case PolicyAllowed, PolicyForbidden, PolicyRestricted, PolicyUnknown:
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
	ID            uuid.UUID    `json:"id"`
	CanonicalName string       `json:"canonical_name"`
	Publisher     *string      `json:"publisher"`
	Edition       *string      `json:"edition"`
	AgreementID   *uuid.UUID   `json:"agreement_id"`
	AgreementName *string      `json:"agreement_name"`
	LicenseType   *LicenseType `json:"license_type"`
	PolicyStatus  PolicyStatus `json:"policy_status"`
	Description   *string      `json:"description"`
	Installations int          `json:"active_installations"`
	ArchivedAt    *time.Time   `json:"archived_at"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type PublicProduct struct {
	ID            uuid.UUID    `json:"id"`
	CanonicalName string       `json:"canonical_name"`
	Publisher     *string      `json:"publisher"`
	Edition       *string      `json:"edition"`
	PolicyStatus  PolicyStatus `json:"policy_status"`
	Description   *string      `json:"description"`
}

func (p Product) Public() PublicProduct {
	return PublicProduct{
		ID:            p.ID,
		CanonicalName: p.CanonicalName,
		Publisher:     p.Publisher,
		Edition:       p.Edition,
		PolicyStatus:  p.PolicyStatus,
		Description:   p.Description,
	}
}

type ProductFilter struct {
	Search          string
	PolicyStatus    PolicyStatus
	IncludeArchived bool
}

type ProductPatch struct {
	CanonicalName *string
	Publisher     *string
	Edition       *string
	AgreementID   opt.Field[uuid.UUID]
	PolicyStatus  *PolicyStatus
	Description   *string
	Archived      *bool
}

func (p ProductPatch) Validate() error {
	if p.CanonicalName != nil && strings.TrimSpace(*p.CanonicalName) == "" {
		return apperr.Invalid("canonical_name must not be empty")
	}
	if p.PolicyStatus != nil && !p.PolicyStatus.Valid() {
		return apperr.Invalid("unknown policy_status")
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
	VersionID    *uuid.UUID  `json:"version_id"`
	MatchMethod  MatchMethod `json:"match_method"`
	ReviewedBy   *string     `json:"reviewed_by"`
	CreatedAt    time.Time   `json:"created_at"`
}
