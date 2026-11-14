package domain

import (
	"time"

	"github.com/google/uuid"
)

type NormalizeInput struct {
	Key         string
	DisplayName string
	Publisher   string
}

type Normalized struct {
	Key           string
	CanonicalName string
	Publisher     string
	IsSoftware    bool
}

type SuggestionItem struct {
	TaskID       uuid.UUID `json:"task_id"`
	SourceKey    string    `json:"source_key"`
	RawName      string    `json:"raw_name"`
	RawPublisher *string   `json:"raw_publisher,omitempty"`
	RawVersion   *string   `json:"raw_version,omitempty"`
}

type CatalogSuggestion struct {
	ID            uuid.UUID        `json:"id"`
	GroupKey      string           `json:"-"`
	CanonicalName string           `json:"canonical_name"`
	Publisher     *string          `json:"publisher"`
	IsSoftware    bool             `json:"is_software"`
	Items         []SuggestionItem `json:"items"`
	Model         *string          `json:"model"`
	Status        ReviewStatus     `json:"status"`
	ProductID     *uuid.UUID       `json:"product_id"`
	ReviewedBy    *string          `json:"reviewed_by"`
	ReviewedAt    *time.Time       `json:"reviewed_at"`
	CreatedAt     time.Time        `json:"created_at"`
}
