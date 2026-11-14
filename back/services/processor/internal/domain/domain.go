package domain

import (
	"time"

	"github.com/google/uuid"
)

type Source string

const (
	SourceCSV   Source = "CSV"
	SourceAgent Source = "AGENT"
)

type ImportStatus string

const (
	ImportAwaiting            ImportStatus = "AWAITING_CONFIRMATION"
	ImportCancelled           ImportStatus = "CANCELLED"
	ImportPending             ImportStatus = "PENDING"
	ImportParsing             ImportStatus = "PARSING"
	ImportProcessing          ImportStatus = "PROCESSING"
	ImportCompleted           ImportStatus = "COMPLETED"
	ImportCompletedWithErrors ImportStatus = "COMPLETED_WITH_ERRORS"
	ImportFailed              ImportStatus = "FAILED"
)

func (s ImportStatus) Terminal() bool {
	return s == ImportCompleted || s == ImportCompletedWithErrors || s == ImportFailed || s == ImportCancelled
}

type SnapshotStatus string

const (
	SnapshotPending    SnapshotStatus = "PENDING"
	SnapshotQueued     SnapshotStatus = "QUEUED"
	SnapshotProcessing SnapshotStatus = "PROCESSING"
	SnapshotCompleted  SnapshotStatus = "COMPLETED"
	SnapshotFailed     SnapshotStatus = "FAILED"
	SnapshotSuperseded SnapshotStatus = "SUPERSEDED"
)

type SnapshotType string

const (
	SnapshotFull    SnapshotType = "FULL"
	SnapshotPartial SnapshotType = "PARTIAL"
)

type RawSoftware struct {
	DisplayName string `json:"display_name"`
	Publisher   string `json:"publisher"`
	Version     string `json:"version"`
}

type Import struct {
	ID          uuid.UUID
	Filename    string
	StoragePath string
	Source      Source
	Status      ImportStatus
	FileSize    int64
	TotalRows   int
	RowStats
	TotalDevices int
	CreatedBy    string
	CollectedAt  time.Time
	ErrorMessage *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	CompletedAt  *time.Time
}

type RowStats struct {
	LoadedRows    int
	DuplicateRows int
	ExcludedRows  int
	ErrorRows     int
}

type Progress struct {
	Total      int
	Pending    int
	Queued     int
	Processing int
	Completed  int
	Failed     int
	Superseded int
}

func (p Progress) Finished() int { return p.Completed + p.Failed + p.Superseded }

func (p Progress) Percent() float64 {
	if p.Total == 0 {
		return 0
	}
	return float64(p.Finished()) * 100 / float64(p.Total)
}

func (p Progress) Derive(current ImportStatus) ImportStatus {
	if current == ImportAwaiting || current == ImportCancelled || current == ImportPending || current == ImportParsing || current == ImportFailed {
		return current
	}
	if p.Total == 0 || p.Finished() < p.Total {
		return ImportProcessing
	}
	switch {
	case p.Failed == p.Total:
		return ImportFailed
	case p.Failed > 0:
		return ImportCompletedWithErrors
	default:
		return ImportCompleted
	}
}

type Snapshot struct {
	ID           uuid.UUID
	ImportID     uuid.UUID
	SerialNumber string
	CollectedAt  time.Time
	Type         SnapshotType
	Software     []RawSoftware
	Status       SnapshotStatus
}

type ImportError struct {
	ID         uuid.UUID  `json:"id"`
	SnapshotID *uuid.UUID `json:"snapshot_id"`
	RowNumber  *int       `json:"row_number"`
	Code       string     `json:"error_code"`
	Message    string     `json:"error_message"`
	CreatedAt  time.Time  `json:"created_at"`
}
