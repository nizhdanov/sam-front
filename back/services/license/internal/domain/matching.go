package domain

import (
	"strings"
	"time"

	"github.com/google/uuid"

	"sam/pkg/swkey"
)

type RawSoftware struct {
	SourceKey   string
	DisplayName string
	Publisher   string
	Version     string
}

func (r RawSoftware) Key() string {
	if r.SourceKey != "" {
		return r.SourceKey
	}
	return swkey.SourceKey(r.DisplayName, r.Publisher, r.Version)
}

type Candidate struct {
	ProductID  uuid.UUID
	Name       string
	Publisher  string
	Edition    string
	Similarity float64
}

type ResolutionStatus string

const (
	ResolutionMatched       ResolutionStatus = "MATCHED"
	ResolutionAmbiguous     ResolutionStatus = "AMBIGUOUS"
	ResolutionUnknown       ResolutionStatus = "UNKNOWN"
	ResolutionPendingReview ResolutionStatus = "PENDING_REVIEW"
)

type Resolution struct {
	SourceKey  string
	Status     ResolutionStatus
	ProductID  *uuid.UUID
	VersionID  *uuid.UUID
	Method     MatchMethod
	Candidates []Candidate
}

type MatchingConfig struct {
	HighThreshold       float64
	CandidateThreshold  float64
	MinGap              float64
	MaxCandidates       int
	LLMAcceptConfidence float64
}

func DecideMatch(normalizedName string, candidates []Candidate, cfg MatchingConfig) (ResolutionStatus, *Candidate, MatchMethod) {
	if len(candidates) == 0 {
		return ResolutionUnknown, nil, ""
	}

	var exact []int
	for i, c := range candidates {
		if swkey.Normalize(c.Name) == normalizedName {
			exact = append(exact, i)
		}
	}
	switch len(exact) {
	case 0:
	case 1:
		return ResolutionMatched, &candidates[exact[0]], MethodExact
	default:
		return ResolutionAmbiguous, nil, ""
	}

	top := candidates[0]
	gap := top.Similarity
	if len(candidates) > 1 {
		gap = top.Similarity - candidates[1].Similarity
	}
	if top.Similarity >= cfg.HighThreshold && gap >= cfg.MinGap {
		return ResolutionMatched, &candidates[0], MethodFuzzy
	}
	return ResolutionAmbiguous, nil, ""
}

func AcceptSuggestion(chosen *Candidate, publisher string, confidence float64, cfg MatchingConfig) bool {
	return chosen != nil &&
		confidence >= cfg.LLMAcceptConfidence &&
		chosen.Similarity >= cfg.CandidateThreshold &&
		PublishersCompatible(publisher, chosen.Publisher)
}

func PublishersCompatible(raw, product string) bool {
	a, b := swkey.NormalizePublisher(raw), swkey.NormalizePublisher(product)
	if a == "" || b == "" {
		return true
	}
	return a == b || strings.Contains(a, b) || strings.Contains(b, a)
}

func PickVersion(raw string, versions []Version) *Version {
	raw = swkey.NormalizeVersion(raw)
	if raw == "" {
		return nil
	}
	var best *Version
	for i := range versions {
		v := &versions[i]
		candidate := swkey.NormalizeVersion(v.Version)
		if candidate == raw {
			return v
		}
		if strings.HasPrefix(raw, candidate+".") && (best == nil || len(candidate) > len(swkey.NormalizeVersion(best.Version))) {
			best = v
		}
	}
	return best
}

type Suggestion struct {
	Software   RawSoftware
	Candidates []Candidate
	ProductID  *uuid.UUID
	Method     MatchMethod
	Confidence float64
	Reason     string
}

type ReviewStatus string

const (
	ReviewPending  ReviewStatus = "PENDING"
	ReviewApproved ReviewStatus = "APPROVED"
	ReviewRejected ReviewStatus = "REJECTED"
)

type MatchingTask struct {
	ID               uuid.UUID       `json:"id"`
	SourceKey        string          `json:"source_key"`
	RawName          string          `json:"raw_name"`
	RawPublisher     *string         `json:"raw_publisher"`
	RawVersion       *string         `json:"raw_version"`
	CandidateIDs     []uuid.UUID     `json:"-"`
	Candidates       []PublicProduct `json:"candidates"`
	SuggestedProduct *uuid.UUID      `json:"suggested_product_id"`
	MatchMethod      *string         `json:"match_method"`
	Confidence       *float64        `json:"confidence_score"`
	Reason           *string         `json:"reason"`
	Status           ReviewStatus    `json:"status"`
	AffectedDevices  int             `json:"affected_devices"`
	ReviewedBy       *string         `json:"reviewed_by"`
	ReviewedAt       *time.Time      `json:"reviewed_at"`
	CreatedAt        time.Time       `json:"created_at"`
}

type Assignment struct {
	SourceKey    string
	RawName      string
	RawPublisher *string
	RawVersion   *string
	ProductID    uuid.UUID
	VersionID    *uuid.UUID
	Method       MatchMethod
	ReviewedBy   *string
}
