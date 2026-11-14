package service

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/pkg/swkey"
	"sam/services/license/internal/domain"
)

func (s *Service) ListProducts(ctx context.Context, f domain.ProductFilter, page httpx.PageRequest) ([]domain.Product, int, error) {
	if _, err := caller(ctx); err != nil {
		return nil, 0, err
	}
	if f.PolicyStatus != "" && !f.PolicyStatus.Valid() {
		return nil, 0, apperr.Invalid("unknown policy_status")
	}
	return s.products.List(ctx, f, page)
}

func (s *Service) GetProduct(ctx context.Context, id uuid.UUID) (domain.Product, error) {
	if _, err := caller(ctx); err != nil {
		return domain.Product{}, err
	}
	return s.products.Get(ctx, id)
}

type ProductInput struct {
	CanonicalName string
	Publisher     *string
	Edition       *string
	AgreementID   *uuid.UUID
	PolicyStatus  domain.PolicyStatus
	PolicyComment *string
	Description   *string
}

func (s *Service) CreateProduct(ctx context.Context, in ProductInput) (domain.Product, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Product{}, err
	}
	name := strings.TrimSpace(in.CanonicalName)
	if name == "" {
		return domain.Product{}, apperr.Invalid("canonical_name is required")
	}
	if in.PolicyStatus == "" {
		in.PolicyStatus = domain.PolicyUnknown
	}
	if !in.PolicyStatus.Valid() {
		return domain.Product{}, apperr.Invalid("unknown policy_status")
	}
	in.PolicyComment = trimmed(in.PolicyComment)
	if in.PolicyStatus == domain.PolicyForbidden && in.PolicyComment == nil {
		return domain.Product{}, apperr.Invalid("policy_comment is required for FORBIDDEN software")
	}
	p := domain.Product{
		ID:            uuid.New(),
		CanonicalName: name,
		Publisher:     trimmed(in.Publisher),
		Edition:       trimmed(in.Edition),
		AgreementID:   in.AgreementID,
		PolicyStatus:  in.PolicyStatus,
		PolicyComment: in.PolicyComment,
		Description:   trimmed(in.Description),
	}
	if err := s.products.Create(ctx, p); err != nil {
		return domain.Product{}, err
	}
	return s.products.Get(ctx, p.ID)
}

func (s *Service) UpdateProduct(ctx context.Context, id uuid.UUID, p domain.ProductPatch) (domain.Product, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Product{}, err
	}
	if err := p.Validate(); err != nil {
		return domain.Product{}, err
	}
	if p.PolicyStatus != nil && *p.PolicyStatus == domain.PolicyForbidden {
		comment := p.PolicyComment
		if comment == nil {
			current, err := s.products.Get(ctx, id)
			if err != nil {
				return domain.Product{}, err
			}
			comment = current.PolicyComment
		}
		if trimmed(comment) == nil {
			return domain.Product{}, apperr.Invalid("policy_comment is required for FORBIDDEN software")
		}
	}
	err := s.withProducts(ctx, []uuid.UUID{id}, func(ctx context.Context) error {
		if err := s.products.Update(ctx, id, p); err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: []uuid.UUID{id}})
	})
	if err != nil {
		return domain.Product{}, err
	}
	return s.products.Get(ctx, id)
}

func (s *Service) Versions(ctx context.Context, productID uuid.UUID) ([]domain.Version, error) {
	if _, err := s.GetProduct(ctx, productID); err != nil {
		return nil, err
	}
	return s.versions.List(ctx, productID)
}

type VersionInput struct {
	Version         string
	LifecycleStatus domain.Lifecycle
	ReleaseDate     *time.Time
	EndOfSupport    *time.Time
}

func (s *Service) CreateVersion(ctx context.Context, productID uuid.UUID, in VersionInput) (domain.Version, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Version{}, err
	}
	v := domain.Version{
		ID:              uuid.New(),
		ProductID:       productID,
		Version:         strings.TrimSpace(in.Version),
		LifecycleStatus: in.LifecycleStatus,
		ReleaseDate:     in.ReleaseDate,
		EndOfSupport:    in.EndOfSupport,
	}
	if v.Version == "" {
		return domain.Version{}, apperr.Invalid("version is required")
	}
	if v.LifecycleStatus == "" {
		v.LifecycleStatus = domain.LifecycleUnknown
	}
	if !v.LifecycleStatus.Valid() {
		return domain.Version{}, apperr.Invalid("unknown lifecycle_status")
	}
	err := s.withProducts(ctx, []uuid.UUID{productID}, func(ctx context.Context) error {
		return s.versions.Create(ctx, v)
	})
	if err != nil {
		return domain.Version{}, err
	}
	return s.versions.Get(ctx, v.ID)
}

func (s *Service) SetVersionLifecycle(ctx context.Context, productID, versionID uuid.UUID, lifecycle domain.Lifecycle) (domain.Version, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Version{}, err
	}
	if !lifecycle.Valid() {
		return domain.Version{}, apperr.Invalid("unknown lifecycle_status")
	}
	err := s.withProducts(ctx, []uuid.UUID{productID}, func(ctx context.Context) error {
		if err := s.versions.SetLifecycle(ctx, productID, versionID, lifecycle); err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: []uuid.UUID{productID}})
	})
	if err != nil {
		return domain.Version{}, err
	}
	return s.versions.Get(ctx, versionID)
}

func (s *Service) Aliases(ctx context.Context, productID uuid.UUID) ([]domain.Alias, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, err
	}
	return s.aliases.ByProduct(ctx, productID)
}

type AliasInput struct {
	RawName      string
	RawPublisher string
	RawVersion   string
	VersionID    *uuid.UUID
}

func (s *Service) CreateAlias(ctx context.Context, productID uuid.UUID, in AliasInput) (domain.Alias, error) {
	c, err := requireAdmin(ctx)
	if err != nil {
		return domain.Alias{}, err
	}
	if strings.TrimSpace(in.RawName) == "" {
		return domain.Alias{}, apperr.Invalid("raw_name is required")
	}
	reviewer := c.Actor()
	a := domain.Assignment{
		SourceKey:    swkey.SourceKey(in.RawName, in.RawPublisher, in.RawVersion),
		RawName:      strings.TrimSpace(in.RawName),
		RawPublisher: trimmed(&in.RawPublisher),
		RawVersion:   trimmed(&in.RawVersion),
		ProductID:    productID,
		VersionID:    in.VersionID,
		Method:       domain.MethodManual,
		ReviewedBy:   &reviewer,
	}
	if err := s.tx.InTx(ctx, func(ctx context.Context) error { return s.assign(ctx, a) }); err != nil {
		return domain.Alias{}, err
	}
	return s.aliases.BySourceKey(ctx, a.SourceKey)
}

func (s *Service) DeleteAlias(ctx context.Context, productID, aliasID uuid.UUID) error {
	if _, err := requireAdmin(ctx); err != nil {
		return err
	}
	return s.withProducts(ctx, []uuid.UUID{productID}, func(ctx context.Context) error {
		key, err := s.aliases.Delete(ctx, productID, aliasID)
		if err != nil {
			return err
		}
		devices, err := s.installations.Unassign(ctx, key, domain.MatchRequiresReview)
		if err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: []uuid.UUID{productID}, DeviceIDs: devices})
	})
}

func (s *Service) assign(ctx context.Context, a domain.Assignment) error {
	affected := []uuid.UUID{a.ProductID}
	if previous, err := s.aliases.BySourceKey(ctx, a.SourceKey); err == nil {
		affected = append(affected, previous.ProductID)
	} else if apperr.KindOf(err) != apperr.KindNotFound {
		return err
	}
	if err := s.locks.Products(ctx, affected); err != nil {
		return err
	}

	if a.VersionID == nil && a.RawVersion != nil {
		versions, err := s.versions.ByProducts(ctx, []uuid.UUID{a.ProductID})
		if err != nil {
			return err
		}
		if v := domain.PickVersion(*a.RawVersion, versions[a.ProductID]); v != nil {
			a.VersionID = &v.ID
		}
	}
	if a.VersionID != nil {
		v, err := s.versions.Get(ctx, *a.VersionID)
		if err != nil {
			return err
		}
		if v.ProductID != a.ProductID {
			return apperr.Invalid("version does not belong to the product")
		}
	}

	if err := s.aliases.Upsert(ctx, a); err != nil {
		return err
	}
	devices, err := s.installations.Assign(ctx, a.SourceKey, a.ProductID, a.VersionID)
	if err != nil {
		return err
	}
	if err := s.matching.Approve(ctx, a.SourceKey, a.ProductID, a.ReviewedBy); err != nil {
		return err
	}
	return s.recalculate(ctx, domain.Scope{ProductIDs: affected, DeviceIDs: devices})
}

func trimmed(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}
