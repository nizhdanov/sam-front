package service

import (
	"context"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/services/license/internal/domain"
)

type DeviceCard struct {
	domain.Device
	History []domain.DeviceImport `json:"imports"`
}

func (s *Service) GetDevice(ctx context.Context, id uuid.UUID) (DeviceCard, error) {
	c, err := caller(ctx)
	if err != nil {
		return DeviceCard{}, err
	}
	d, err := s.devices.Get(ctx, id)
	if err != nil {
		return DeviceCard{}, err
	}
	if !c.IsAdmin() && d.SerialNumber != c.DeviceSerial {
		return DeviceCard{}, apperr.NotFound("device not found")
	}
	return s.deviceCard(ctx, d)
}

func (s *Service) MyDevice(ctx context.Context) (DeviceCard, error) {
	c, err := caller(ctx)
	if err != nil {
		return DeviceCard{}, err
	}
	serial, err := deviceOf(c)
	if err != nil {
		return DeviceCard{}, err
	}
	d, err := s.devices.BySerial(ctx, serial)
	if err != nil {
		return DeviceCard{}, err
	}
	return s.deviceCard(ctx, d)
}

func (s *Service) deviceCard(ctx context.Context, d domain.Device) (DeviceCard, error) {
	history, err := s.devices.History(ctx, d.ID)
	if err != nil {
		return DeviceCard{}, err
	}
	return DeviceCard{Device: d, History: history}, nil
}

func (s *Service) UpdateDevice(ctx context.Context, id uuid.UUID, p domain.DevicePatch) (DeviceCard, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return DeviceCard{}, err
	}
	if p.Type != nil && !p.Type.Valid() {
		return DeviceCard{}, apperr.Invalid("unknown device_type")
	}
	if p.Status != nil && !p.Status.Valid() {
		return DeviceCard{}, apperr.Invalid("unknown status")
	}
	current, err := s.devices.Get(ctx, id)
	if err != nil {
		return DeviceCard{}, err
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.devices.Update(ctx, id, p); err != nil {
			return err
		}
		return s.record(ctx, "device", id.String(), domain.AuditUpdate, "Изменено устройство "+current.SerialNumber, deviceChanges(p))
	})
	if err != nil {
		return DeviceCard{}, err
	}
	return s.GetDevice(ctx, id)
}

func deviceChanges(p domain.DevicePatch) map[string]any {
	out := map[string]any{}
	if p.Type != nil {
		out["device_type"] = *p.Type
	}
	if p.Status != nil {
		out["status"] = *p.Status
	}
	if p.Hostname.Set {
		out["hostname"] = p.Hostname.Value
	}
	if p.Responsible.Set {
		out["responsible"] = p.Responsible.Value
	}
	if p.Location.Set {
		out["location"] = p.Location.Value
	}
	return out
}
