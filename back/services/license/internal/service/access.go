package service

import (
	"context"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/services/license/internal/domain"
)

func caller(ctx context.Context) (*auth.Claims, error) {
	c := auth.ClaimsFrom(ctx)
	if c == nil {
		return nil, apperr.Unauthenticated("authentication required")
	}
	return c, nil
}

func requireAdmin(ctx context.Context) (*auth.Claims, error) {
	return auth.Require(ctx, auth.RoleAdmin)
}

func deviceOf(c *auth.Claims) (string, error) {
	if c.DeviceSerial == "" {
		return "", apperr.Forbidden("user is not linked to a device")
	}
	return c.DeviceSerial, nil
}

func (s *Service) record(ctx context.Context, entity, id, action, summary string, changes map[string]any) error {
	return s.audit.Record(ctx, domain.AuditEntry{
		Actor:    auth.ClaimsFrom(ctx).Actor(),
		Entity:   entity,
		EntityID: id,
		Action:   action,
		Summary:  summary,
		Changes:  changes,
	})
}
