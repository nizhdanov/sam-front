package service

import (
	"context"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/services/license/internal/domain"
)

func (s *Service) SyncInventory(ctx context.Context, snap domain.Snapshot) (domain.SyncOutcome, error) {
	if _, err := auth.Require(ctx, auth.RoleInventoryWorker, auth.RoleAdmin); err != nil {
		return domain.SyncOutcome{}, err
	}
	if snap.InventoryID == uuid.Nil || snap.SerialNumber == "" || snap.CollectedAt.IsZero() {
		return domain.SyncOutcome{}, apperr.Invalid("inventory_id, serial_number and collected_at are required")
	}

	var out domain.SyncOutcome
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		deviceID, lastInventory, err := s.devices.LockBySerial(ctx, snap.SerialNumber)
		if err != nil {
			return err
		}
		out.DeviceID = deviceID

		processed, err := s.devices.Processed(ctx, snap.InventoryID)
		if err != nil {
			return err
		}
		if processed {
			out.Result = domain.SyncAlreadyProcessed
			out.ActiveInstallations, err = s.installations.CountActive(ctx, deviceID)
			return err
		}

		if lastInventory != nil && snap.CollectedAt.Before(*lastInventory) {
			out.Result = domain.SyncSuperseded
			return s.devices.MarkProcessed(ctx, snap.InventoryID, deviceID, snap.CollectedAt, domain.SyncSuperseded)
		}

		items, err := s.applyAliases(ctx, snap.Items)
		if err != nil {
			return err
		}
		affected, err := s.installations.ActiveProductIDs(ctx, deviceID)
		if err != nil {
			return err
		}
		for _, it := range items {
			if it.ProductID != nil {
				affected = append(affected, *it.ProductID)
			}
		}
		if err := s.locks.Products(ctx, affected); err != nil {
			return err
		}

		if err := s.installations.Upsert(ctx, deviceID, snap.CollectedAt, items); err != nil {
			return err
		}
		if snap.FullSnapshot {
			keys := make([]string, 0, len(items))
			for _, it := range items {
				keys = append(keys, it.SourceKey)
			}
			if err := s.installations.DeactivateMissing(ctx, deviceID, keys, snap.CollectedAt); err != nil {
				return err
			}
		}
		if err := s.devices.MarkInventoried(ctx, deviceID, snap.CollectedAt, snap.InventoryID); err != nil {
			return err
		}
		if err := s.recalculate(ctx, domain.Scope{DeviceIDs: []uuid.UUID{deviceID}, ProductIDs: affected}); err != nil {
			return err
		}
		if err := s.devices.MarkProcessed(ctx, snap.InventoryID, deviceID, snap.CollectedAt, domain.SyncApplied); err != nil {
			return err
		}
		out.Result = domain.SyncApplied
		out.ActiveInstallations, err = s.installations.CountActive(ctx, deviceID)
		return err
	})
	return out, err
}

func (s *Service) applyAliases(ctx context.Context, items []domain.InstallationInput) ([]domain.InstallationInput, error) {
	keys := make([]string, 0, len(items))
	for _, it := range items {
		keys = append(keys, it.SourceKey)
	}
	aliases, err := s.aliases.BySourceKeys(ctx, keys)
	if err != nil {
		return nil, err
	}
	out := make([]domain.InstallationInput, 0, len(items))
	seen := make(map[string]bool, len(items))
	for _, it := range items {
		if it.SourceKey == "" || seen[it.SourceKey] {
			continue
		}
		seen[it.SourceKey] = true
		if a, ok := aliases[it.SourceKey]; ok {
			productID := a.ProductID
			it.ProductID, it.VersionID, it.MatchStatus = &productID, a.VersionID, domain.MatchMatched
		} else {
			it.ProductID, it.VersionID = nil, nil
			if it.MatchStatus == "" || it.MatchStatus == domain.MatchMatched {
				it.MatchStatus = domain.MatchRequiresReview
			}
		}
		out = append(out, it)
	}
	return out, nil
}
