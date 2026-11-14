package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrLeaseHeld   = errors.New("snapshot is being processed by another worker")
	ErrLeaseLost   = errors.New("snapshot lease was lost")
	ErrAlreadyDone = errors.New("snapshot is already processed")
)

type Status string

const (
	StatusPending    Status = "PENDING"
	StatusQueued     Status = "QUEUED"
	StatusProcessing Status = "PROCESSING"
	StatusCompleted  Status = "COMPLETED"
	StatusFailed     Status = "FAILED"
	StatusSuperseded Status = "SUPERSEDED"
)

func (s Status) Final() bool {
	return s == StatusCompleted || s == StatusFailed || s == StatusSuperseded
}

type RawSoftware struct {
	DisplayName string `json:"display_name"`
	Publisher   string `json:"publisher"`
	Version     string `json:"version"`
}

type Snapshot struct {
	ID           uuid.UUID
	ImportID     uuid.UUID
	SerialNumber string
	CollectedAt  time.Time
	Full         bool
	Software     []RawSoftware
	Status       Status
	Attempts     int
}

type Lease struct {
	SnapshotID uuid.UUID
	Token      uuid.UUID
	Attempt    int
}

type MatchStatus string

const (
	MatchMatched        MatchStatus = "MATCHED"
	MatchUnknown        MatchStatus = "UNKNOWN"
	MatchRequiresReview MatchStatus = "REQUIRES_REVIEW"
)

type Item struct {
	SourceKey string
	RawSoftware
}

type Match struct {
	ProductID string
	VersionID string
	Status    MatchStatus
}

type Candidate struct {
	ProductID  string
	Name       string
	Publisher  string
	Edition    string
	Similarity float64
}

type ResolutionStatus int

const (
	ResolutionUnknown ResolutionStatus = iota
	ResolutionMatched
	ResolutionAmbiguous
	ResolutionPendingReview
)

type Resolution struct {
	SourceKey  string
	Status     ResolutionStatus
	ProductID  string
	VersionID  string
	Candidates []Candidate
}

type Suggestion struct {
	ProductID  string
	Confidence float64
	Reason     string
	Model      string
}

type SyncResult string

const (
	SyncApplied          SyncResult = "APPLIED"
	SyncAlreadyProcessed SyncResult = "ALREADY_PROCESSED"
	SyncSuperseded       SyncResult = "SUPERSEDED"
)

type Installation struct {
	Item
	Match
}
