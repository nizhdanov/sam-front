package service

import (
	"context"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/auth"
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

func employeeOf(c *auth.Claims) *uuid.UUID {
	id, err := uuid.Parse(c.EmployeeID)
	if err != nil {
		return nil
	}
	return &id
}
