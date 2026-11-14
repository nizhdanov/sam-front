package service

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/swkey"
	"sam/services/worker/internal/domain"
)

type Snapshots interface {
	Get(ctx context.Context, id uuid.UUID) (domain.Snapshot, error)
	Claim(ctx context.Context, id uuid.UUID, lease time.Duration) (domain.Lease, error)
	Extend(ctx context.Context, l domain.Lease, lease time.Duration) error
	Finish(ctx context.Context, l domain.Lease, status domain.Status, message string) error
	FailUnfinished(ctx context.Context, id uuid.UUID, message string) error
}

type Matcher interface {
	Resolve(ctx context.Context, items []domain.Item) (map[string]domain.Match, error)
}

type Inventory interface {
	Sync(ctx context.Context, s domain.Snapshot, installations []domain.Installation) (domain.SyncResult, error)
}

type Config struct {
	Lease          time.Duration
	ProcessTimeout time.Duration
}

type Service struct {
	snapshots Snapshots
	matcher   Matcher
	inventory Inventory
	cfg       Config
	log       *slog.Logger
}

func New(snapshots Snapshots, matcher Matcher, inventory Inventory, cfg Config, log *slog.Logger) *Service {
	return &Service{snapshots: snapshots, matcher: matcher, inventory: inventory, cfg: cfg, log: log}
}

type Heartbeat func()

func (s *Service) Process(ctx context.Context, inventoryID uuid.UUID, serial string, heartbeat Heartbeat) error {
	ctx, cancel := context.WithTimeout(ctx, s.cfg.ProcessTimeout)
	defer cancel()
	log := s.log.With("inventory_id", inventoryID, "serial_number", serial)

	snap, err := s.snapshots.Get(ctx, inventoryID)
	if err != nil {
		return err
	}
	if snap.SerialNumber != serial {
		return permanent(errors.New("event serial number does not match the snapshot"))
	}
	if snap.Status.Final() {
		return nil
	}

	lease, err := s.snapshots.Claim(ctx, inventoryID, s.cfg.Lease)
	if errors.Is(err, domain.ErrAlreadyDone) {
		return nil
	}
	if err != nil {
		return err
	}

	stopBeat := s.keepAlive(ctx, lease, heartbeat)
	status, procErr := s.process(ctx, snap)
	stopBeat()

	if procErr != nil {
		if IsPermanent(procErr) {
			if err := s.snapshots.Finish(ctx, lease, domain.StatusFailed, procErr.Error()); err != nil {
				log.Error("mark snapshot failed", "err", err)
			}
			return procErr
		}
		finishCtx, cancelFinish := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancelFinish()
		if err := s.snapshots.Finish(finishCtx, lease, domain.StatusQueued, procErr.Error()); err != nil {
			log.Warn("release snapshot", "err", err)
		}
		return procErr
	}
	if err := s.snapshots.Finish(ctx, lease, status, ""); err != nil {
		return err
	}
	log.Info("snapshot processed", "status", status, "attempt", lease.Attempt)
	return nil
}

func (s *Service) process(ctx context.Context, snap domain.Snapshot) (domain.Status, error) {
	items := make([]domain.Item, 0, len(snap.Software))
	seen := make(map[string]struct{}, len(snap.Software))
	for _, sw := range snap.Software {
		key := swkey.SourceKey(sw.DisplayName, sw.Publisher, sw.Version)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		items = append(items, domain.Item{SourceKey: key, RawSoftware: sw})
	}

	matches, err := s.matcher.Resolve(ctx, items)
	if err != nil {
		return "", err
	}
	installations := make([]domain.Installation, 0, len(items))
	for _, it := range items {
		match, ok := matches[it.SourceKey]
		if !ok {
			match = domain.Match{Status: domain.MatchUnknown}
		}
		installations = append(installations, domain.Installation{Item: it, Match: match})
	}

	result, err := s.inventory.Sync(ctx, snap, installations)
	if err != nil {
		return "", err
	}
	if result == domain.SyncSuperseded {
		return domain.StatusSuperseded, nil
	}
	return domain.StatusCompleted, nil
}

func (s *Service) keepAlive(ctx context.Context, lease domain.Lease, heartbeat Heartbeat) func() {
	interval := s.cfg.Lease / 3
	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				heartbeat()
				if err := s.snapshots.Extend(ctx, lease, s.cfg.Lease); err != nil {
					s.log.Warn("extend snapshot lease", "inventory_id", lease.SnapshotID, "err", err)
				}
			}
		}
	}()
	return func() { close(done) }
}

func (s *Service) GiveUp(ctx context.Context, inventoryID uuid.UUID, cause error) {
	msg := "processing failed after maximum delivery attempts"
	if cause != nil {
		msg += ": " + cause.Error()
	}
	if err := s.snapshots.FailUnfinished(ctx, inventoryID, msg); err != nil {
		s.log.Error("mark snapshot failed", "inventory_id", inventoryID, "err", err)
	}
}

type permanentError struct{ err error }

func (e permanentError) Error() string { return e.err.Error() }

func (e permanentError) Unwrap() error { return e.err }

func permanent(err error) error { return permanentError{err: err} }

func IsPermanent(err error) bool {
	var p permanentError
	if errors.As(err, &p) {
		return true
	}
	switch apperr.KindOf(err) {
	case apperr.KindInvalid, apperr.KindNotFound, apperr.KindForbidden, apperr.KindUnauthenticated:
		return true
	}
	return false
}
