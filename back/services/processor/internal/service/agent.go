package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/services/processor/internal/domain"
)

type AgentReport struct {
	SerialNumber string
	CollectedAt  time.Time
	Partial      bool
	Software     []domain.RawSoftware
}

func (s *Service) IngestAgent(ctx context.Context, r AgentReport) (domain.Import, error) {
	claims, err := auth.Require(ctx, auth.RoleAdmin, auth.RoleInventoryAgent)
	if err != nil {
		return domain.Import{}, err
	}
	r.SerialNumber = strings.TrimSpace(r.SerialNumber)
	if r.SerialNumber == "" || len(r.SerialNumber) > s.cfg.MaxFieldBytes {
		return domain.Import{}, apperr.Invalid("serial_number is required")
	}
	if r.CollectedAt.IsZero() {
		r.CollectedAt = time.Now().UTC()
	}
	if r.CollectedAt.After(time.Now().Add(5 * time.Minute)) {
		return domain.Import{}, apperr.Invalid("collected_at is in the future")
	}

	software := make([]domain.RawSoftware, 0, len(r.Software))
	seen := make(map[domain.RawSoftware]struct{}, len(r.Software))
	for _, sw := range r.Software {
		sw = domain.RawSoftware{
			DisplayName: strings.TrimSpace(sw.DisplayName),
			Publisher:   strings.TrimSpace(sw.Publisher),
			Version:     strings.TrimSpace(sw.Version),
		}
		if sw.DisplayName == "" {
			return domain.Import{}, apperr.Invalid("software display_name is required")
		}
		if _, dup := seen[sw]; dup {
			continue
		}
		seen[sw] = struct{}{}
		software = append(software, sw)
	}
	if len(software) > s.cfg.MaxRows {
		return domain.Import{}, apperr.TooLarge("too many software entries")
	}

	snapshotType := domain.SnapshotFull
	if r.Partial {
		snapshotType = domain.SnapshotPartial
	}
	imp := domain.Import{
		ID:           uuid.New(),
		Filename:     "agent:" + r.SerialNumber,
		Source:       domain.SourceAgent,
		Status:       domain.ImportProcessing,
		CreatedBy:    claims.Actor(),
		CollectedAt:  r.CollectedAt,
		TotalRows:    len(software),
		TotalDevices: 1,
	}
	snap := domain.Snapshot{
		ID:           uuid.New(),
		ImportID:     imp.ID,
		SerialNumber: r.SerialNumber,
		CollectedAt:  r.CollectedAt,
		Type:         snapshotType,
		Software:     software,
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.imports.Create(ctx, imp); err != nil {
			return err
		}
		if err := s.snapshots.Insert(ctx, []domain.Snapshot{snap}); err != nil {
			return err
		}
		return s.imports.SetParsed(ctx, imp.ID, len(software), 1)
	})
	if err != nil {
		return domain.Import{}, err
	}
	if _, failed := s.publish(ctx, imp.ID, 0); failed > 0 {
		s.log.WarnContext(ctx, "agent snapshot saved but not queued", "import_id", imp.ID)
	}
	return imp, nil
}
