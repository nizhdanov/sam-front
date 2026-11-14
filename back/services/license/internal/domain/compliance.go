package domain

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

type FindingType string

const (
	FindingLicenseDeficit    FindingType = "LICENSE_DEFICIT"
	FindingForbiddenSoftware FindingType = "FORBIDDEN_SOFTWARE"
	FindingOutdatedSoftware  FindingType = "OUTDATED_SOFTWARE"
)

var (
	InstallationFindingTypes = []FindingType{
		FindingForbiddenSoftware, FindingOutdatedSoftware,
	}
	ProductFindingTypes = []FindingType{
		FindingLicenseDeficit,
	}
)

func (t FindingType) Valid() bool {
	return slices.Contains(InstallationFindingTypes, t) || slices.Contains(ProductFindingTypes, t)
}

type Severity string

const (
	SeverityCritical Severity = "CRITICAL"
	SeverityHigh     Severity = "HIGH"
	SeverityMedium   Severity = "MEDIUM"
	SeverityLow      Severity = "LOW"
)

var severities = map[FindingType]Severity{
	FindingLicenseDeficit:    SeverityHigh,
	FindingForbiddenSoftware: SeverityCritical,
	FindingOutdatedSoftware:  SeverityMedium,
}

type FindingStatus string

const (
	FindingOpen         FindingStatus = "OPEN"
	FindingAcknowledged FindingStatus = "ACKNOWLEDGED"
	FindingResolved     FindingStatus = "RESOLVED"
)

type Finding struct {
	ID             uuid.UUID      `json:"id"`
	Key            string         `json:"-"`
	Type           FindingType    `json:"finding_type"`
	Severity       Severity       `json:"severity"`
	Status         FindingStatus  `json:"status"`
	DeviceID       *uuid.UUID     `json:"device_id"`
	SerialNumber   *string        `json:"serial_number,omitempty"`
	ProductID      *uuid.UUID     `json:"product_id"`
	ProductName    *string        `json:"product_name,omitempty"`
	InstallationID *uuid.UUID     `json:"installation_id"`
	Details        map[string]any `json:"details"`
	AcknowledgedBy *string        `json:"acknowledged_by,omitempty"`
	AckComment     *string        `json:"acknowledge_comment,omitempty"`
	AcknowledgedAt *time.Time     `json:"acknowledged_at,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	LastDetectedAt time.Time      `json:"last_detected_at"`
	ResolvedAt     *time.Time     `json:"resolved_at"`
}

type FindingFilter struct {
	Types        []FindingType
	Severity     Severity
	Status       FindingStatus
	DeviceID     *uuid.UUID
	ProductID    *uuid.UUID
	SerialNumber string
}

type InstallationFacts struct {
	ID             uuid.UUID
	DeviceID       uuid.UUID
	ProductID      *uuid.UUID
	ProductName    *string
	Status         *ProductStatus
	CurrentVersion *string
	Version        *string
	RawName        string
	RawVersion     *string
	MatchStatus    MatchStatus
}

func InstallationFindings(f InstallationFacts) []Finding {
	var out []Finding
	add := func(t FindingType, details map[string]any) {
		details["raw_name"] = f.RawName
		if f.RawVersion != nil {
			details["raw_version"] = *f.RawVersion
		}
		if f.ProductName != nil {
			details["product_name"] = *f.ProductName
		}
		deviceID, installationID := f.DeviceID, f.ID
		out = append(out, Finding{
			Key:            fmt.Sprintf("%s:%s", t, f.ID),
			Type:           t,
			Severity:       severities[t],
			DeviceID:       &deviceID,
			ProductID:      f.ProductID,
			InstallationID: &installationID,
			Details:        details,
		})
	}

	if f.ProductID == nil {
		return out
	}
	if f.Status != nil && *f.Status == StatusForbidden {
		add(FindingForbiddenSoftware, map[string]any{"status": *f.Status})
	}
	installed := f.Version
	if installed == nil {
		installed = f.RawVersion
	}
	switch {
	case f.Status != nil && *f.Status == StatusOutdated:
		add(FindingOutdatedSoftware, map[string]any{"version": valueOrNil(installed), "reason": "PRODUCT_OUTDATED"})
	case f.CurrentVersion != nil && installed != nil && CompareVersions(*installed, *f.CurrentVersion) < 0:
		add(FindingOutdatedSoftware, map[string]any{"version": *installed, "current_version": *f.CurrentVersion})
	}
	return out
}

type CalculationStatus string

const (
	CalcVerified          CalculationStatus = "VERIFIED"
	CalcIncomplete        CalculationStatus = "INCOMPLETE"
	CalcUnverified        CalculationStatus = "UNVERIFIED"
	CalcUnsupportedMetric CalculationStatus = "UNSUPPORTED_METRIC"
)

type ProductFacts struct {
	ID        uuid.UUID
	Name      string
	CostModel *CostModel
	Devices   int
}

type Balance struct {
	ProductID         uuid.UUID         `json:"product_id"`
	ProductName       string            `json:"product_name"`
	CostModel         *CostModel        `json:"cost_model"`
	Metric            Metric            `json:"metric"`
	Purchased         *int              `json:"purchased"`
	Consumed          int               `json:"consumed"`
	Available         *int              `json:"available"`
	Deficit           int               `json:"deficit"`
	CalculationStatus CalculationStatus `json:"calculation_status"`
	Note              string            `json:"note,omitempty"`
	LastCalculatedAt  time.Time         `json:"last_calculated_at"`
}

func EvaluateLicensing(p ProductFacts, entitlements []Entitlement, now time.Time) ([]Balance, []Finding) {
	requiresLicense := p.CostModel != nil && p.CostModel.RequiresEntitlement()
	if !requiresLicense && len(entitlements) == 0 {
		return nil, nil
	}

	var valid, expired []Entitlement
	for _, e := range entitlements {
		switch {
		case e.ValidAt(now):
			valid = append(valid, e)
		case e.ExpiredAt(now):
			expired = append(expired, e)
		}
	}

	var findings []Finding
	addFinding := func(t FindingType, suffix string, details map[string]any) {
		details["product_name"] = p.Name
		key := fmt.Sprintf("%s:%s", t, p.ID)
		if suffix != "" {
			key += ":" + suffix
		}
		productID := p.ID
		findings = append(findings, Finding{
			Key: key, Type: t, Severity: severities[t], ProductID: &productID, Details: details,
		})
	}
	installed := p.Devices > 0

	if len(valid) == 0 {
		zero := 0
		balance := Balance{
			ProductID: p.ID, ProductName: p.Name, CostModel: p.CostModel, Metric: MetricUnknown,
			Purchased: &zero, Available: &zero, Consumed: p.Devices, Deficit: p.Devices,
			CalculationStatus: CalcVerified, LastCalculatedAt: now,
		}
		switch {
		case installed && len(expired) > 0:
			balance.Note = "all entitlements have expired"
			addFinding(FindingLicenseDeficit, "", map[string]any{
				"reason": "ALL_CONTRACTS_EXPIRED", "expired_at": latestExpiry(expired), "purchased": 0, "consumed": p.Devices, "deficit": p.Devices,
			})
		case installed && requiresLicense:
			balance.Note = "no entitlements registered for a product that requires a license"
			addFinding(FindingLicenseDeficit, "", map[string]any{
				"reason": "NO_ENTITLEMENTS", "purchased": 0, "consumed": p.Devices, "deficit": p.Devices,
			})
		}
		return []Balance{balance}, findings
	}

	byMetric := map[Metric][]Entitlement{}
	for _, e := range valid {
		byMetric[e.Metric] = append(byMetric[e.Metric], e)
	}
	if _, ok := byMetric[MetricUnlimited]; ok {
		return []Balance{{
			ProductID: p.ID, ProductName: p.Name, CostModel: p.CostModel, Metric: MetricUnlimited,
			Consumed: p.Devices, CalculationStatus: CalcVerified, LastCalculatedAt: now,
		}}, nil
	}

	metrics := make([]Metric, 0, len(byMetric))
	for m := range byMetric {
		metrics = append(metrics, m)
	}
	slices.Sort(metrics)
	mixed := len(metrics) > 1

	balances := make([]Balance, 0, len(metrics))
	for _, m := range metrics {
		purchased := 0
		for _, e := range byMetric[m] {
			purchased += e.Quantity
		}
		b := Balance{ProductID: p.ID, ProductName: p.Name, CostModel: p.CostModel, Metric: m, CalculationStatus: CalcVerified, LastCalculatedAt: now}

		switch m {
		case MetricPerDevice:
			b.Consumed = p.Devices
		default:
			b.Consumed = p.Devices
			b.CalculationStatus = CalcUnsupportedMetric
			b.Note = "usage for this metric cannot be derived from installation data"
			balances = append(balances, b)
			continue
		}

		available := max(0, purchased-b.Consumed)
		b.Purchased, b.Available = &purchased, &available
		b.Deficit = max(0, b.Consumed-purchased)
		if mixed {
			b.CalculationStatus = CalcIncomplete
			b.Note = "product has entitlements with different metrics; quantities are not combined"
		}
		balances = append(balances, b)

		if b.Deficit > 0 && !mixed {
			addFinding(FindingLicenseDeficit, string(m), map[string]any{
				"metric": m, "purchased": purchased, "consumed": b.Consumed, "deficit": b.Deficit,
			})
		}
	}
	return balances, findings
}

func latestExpiry(entitlements []Entitlement) time.Time {
	var latest time.Time
	for _, e := range entitlements {
		if e.ExpiresAt != nil && e.ExpiresAt.After(latest) {
			latest = *e.ExpiresAt
		}
	}
	return latest
}

func valueOrNil[T any](p *T) any {
	if p == nil {
		return nil
	}
	return *p
}

type Scope struct {
	All        bool
	DeviceIDs  []uuid.UUID
	ProductIDs []uuid.UUID
}

func (s Scope) Empty() bool { return !s.All && len(s.DeviceIDs) == 0 && len(s.ProductIDs) == 0 }

type ComplianceSummary struct {
	Open         int                 `json:"open"`
	Acknowledged int                 `json:"acknowledged"`
	BySeverity   map[Severity]int    `json:"by_severity"`
	ByType       map[FindingType]int `json:"by_type"`
}

func CompareVersions(a, b string) int {
	pa, pb := versionParts(a), versionParts(b)
	for i := range max(len(pa), len(pb)) {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			if x < y {
				return -1
			}
			return 1
		}
	}
	return 0
}

func versionParts(v string) []int {
	var parts []int
	n, inNumber := 0, false
	for _, r := range v {
		if r >= '0' && r <= '9' {
			n = n*10 + int(r-'0')
			inNumber = true
			continue
		}
		if inNumber {
			parts = append(parts, n)
			n, inNumber = 0, false
		}
	}
	if inNumber {
		parts = append(parts, n)
	}
	return parts
}
