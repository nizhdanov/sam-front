package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/httpx"
	"sam/pkg/swkey"
	"sam/services/license/internal/domain"
)

func (s *Service) ListProducts(ctx context.Context, f domain.ProductFilter, page httpx.PageRequest) ([]domain.Product, int, error) {
	c, err := caller(ctx)
	if err != nil {
		return nil, 0, err
	}
	if f.Status != "" && !f.Status.Valid() {
		return nil, 0, apperr.Invalid("unknown status")
	}
	if f.LicenseType != "" && !f.LicenseType.Valid() {
		return nil, 0, apperr.Invalid("unknown license_type")
	}
	if f.CostModel != "" && !f.CostModel.Valid() {
		return nil, 0, apperr.Invalid("unknown cost_model")
	}
	f.PublicOnly = !c.IsAdmin()
	if f.PublicOnly {
		f.IncludeArchived = false
	}
	return s.products.List(ctx, f, page)
}

func (s *Service) GetProduct(ctx context.Context, id uuid.UUID) (domain.Product, error) {
	c, err := caller(ctx)
	if err != nil {
		return domain.Product{}, err
	}
	p, err := s.products.Get(ctx, id)
	if err != nil {
		return domain.Product{}, err
	}
	if !c.IsAdmin() && !p.InBPO && p.Status != domain.StatusForbidden && (p.CostModel == nil || *p.CostModel != domain.CostFree) {
		return domain.Product{}, apperr.NotFound("software product not found")
	}
	return p, nil
}

type ProductInput struct {
	CanonicalName  string
	Publisher      *string
	Edition        *string
	Purpose        *string
	Section        *string
	Class          *string
	InBPO          bool
	CurrentVersion *string
	AgreementID    *uuid.UUID
	Status         domain.ProductStatus
	StatusComment  *string
}

func (s *Service) CreateProduct(ctx context.Context, in ProductInput) (domain.Product, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return domain.Product{}, err
	}
	name := strings.TrimSpace(in.CanonicalName)
	if name == "" {
		return domain.Product{}, apperr.Invalid("canonical_name is required")
	}
	if in.Status == "" {
		in.Status = domain.StatusActual
	}
	if !in.Status.Valid() {
		return domain.Product{}, apperr.Invalid("unknown status")
	}
	in.StatusComment = trimmed(in.StatusComment)
	if in.Status == domain.StatusForbidden && in.StatusComment == nil {
		return domain.Product{}, apperr.Invalid("status_comment is required for FORBIDDEN software")
	}
	p := domain.Product{
		ID:             uuid.New(),
		CanonicalName:  name,
		Publisher:      trimmed(in.Publisher),
		Edition:        trimmed(in.Edition),
		Purpose:        trimmed(in.Purpose),
		Section:        trimmed(in.Section),
		Class:          trimmed(in.Class),
		InBPO:          in.InBPO,
		CurrentVersion: trimmed(in.CurrentVersion),
		AgreementID:    in.AgreementID,
		Status:         in.Status,
		StatusComment:  in.StatusComment,
	}
	err := s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.products.Create(ctx, p); err != nil {
			return err
		}
		return s.record(ctx, "product", p.ID.String(), domain.AuditCreate, "Создан продукт "+p.CanonicalName,
			map[string]any{"status": p.Status, "in_bpo": p.InBPO})
	})
	if err != nil {
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
	current, err := s.products.Get(ctx, id)
	if err != nil {
		return domain.Product{}, err
	}
	if p.Status != nil && *p.Status == domain.StatusForbidden {
		comment := current.StatusComment
		if p.StatusComment != nil {
			comment = p.StatusComment
		}
		if trimmed(comment) == nil {
			return domain.Product{}, apperr.Invalid("status_comment is required for FORBIDDEN software")
		}
	}
	err = s.withProducts(ctx, []uuid.UUID{id}, func(ctx context.Context) error {
		if err := s.products.Update(ctx, id, p); err != nil {
			return err
		}
		if err := s.record(ctx, "product", id.String(), domain.AuditUpdate, "Изменён продукт "+current.CanonicalName, productChanges(p)); err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: []uuid.UUID{id}})
	})
	if err != nil {
		return domain.Product{}, err
	}
	return s.products.Get(ctx, id)
}

func (s *Service) DeleteProduct(ctx context.Context, id uuid.UUID) error {
	if _, err := requireAdmin(ctx); err != nil {
		return err
	}
	current, err := s.products.Get(ctx, id)
	if err != nil {
		return err
	}
	installations, contracts, err := s.products.Usage(ctx, id)
	if err != nil {
		return err
	}
	if installations > 0 || contracts > 0 {
		return apperr.Conflict(fmt.Sprintf("product has %d installation(s) and %d contract(s); archive it instead", installations, contracts))
	}
	return s.withProducts(ctx, []uuid.UUID{id}, func(ctx context.Context) error {
		if err := s.products.Delete(ctx, id); err != nil {
			return err
		}
		return s.record(ctx, "product", id.String(), domain.AuditDelete, "Удалён продукт "+current.CanonicalName, nil)
	})
}

func productChanges(p domain.ProductPatch) map[string]any {
	out := map[string]any{}
	set := func(key string, v any, ok bool) {
		if ok {
			out[key] = v
		}
	}
	set("canonical_name", deref(p.CanonicalName), p.CanonicalName != nil)
	set("publisher", deref(p.Publisher), p.Publisher != nil)
	set("edition", deref(p.Edition), p.Edition != nil)
	set("purpose", deref(p.Purpose), p.Purpose != nil)
	set("section", deref(p.Section), p.Section != nil)
	set("class", deref(p.Class), p.Class != nil)
	set("in_bpo", deref(p.InBPO), p.InBPO != nil)
	set("current_version", p.CurrentVersion.Value, p.CurrentVersion.Set)
	set("agreement_id", p.AgreementID.Value, p.AgreementID.Set)
	set("status", deref(p.Status), p.Status != nil)
	set("status_comment", deref(p.StatusComment), p.StatusComment != nil)
	set("archived", deref(p.Archived), p.Archived != nil)
	return out
}

func deref[T any](v *T) any {
	if v == nil {
		return nil
	}
	return *v
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
		if err := s.versions.Create(ctx, v); err != nil {
			return err
		}
		return s.record(ctx, "version", v.ID.String(), domain.AuditCreate, "Добавлена версия "+v.Version,
			map[string]any{"product_id": productID, "lifecycle_status": v.LifecycleStatus})
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
		if err := s.record(ctx, "version", versionID.String(), domain.AuditUpdate, "Изменён статус версии",
			map[string]any{"lifecycle_status": lifecycle}); err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: []uuid.UUID{productID}})
	})
	if err != nil {
		return domain.Version{}, err
	}
	return s.versions.Get(ctx, versionID)
}

func (s *Service) Rules(ctx context.Context, f domain.RuleFilter, page httpx.PageRequest) ([]domain.Alias, int, error) {
	if _, err := requireAdmin(ctx); err != nil {
		return nil, 0, err
	}
	return s.aliases.List(ctx, f, page)
}

type RuleInput struct {
	RawName      string
	RawPublisher string
	RawVersion   string
	ProductID    uuid.UUID
	VersionID    *uuid.UUID
}

func (s *Service) CreateRule(ctx context.Context, in RuleInput) (domain.Alias, error) {
	c, err := requireAdmin(ctx)
	if err != nil {
		return domain.Alias{}, err
	}
	if strings.TrimSpace(in.RawName) == "" {
		return domain.Alias{}, apperr.Invalid("raw_name is required")
	}
	if in.ProductID == uuid.Nil {
		return domain.Alias{}, apperr.Invalid("product_id is required")
	}
	reviewer := c.Actor()
	a := domain.Assignment{
		SourceKey:    swkey.SourceKey(in.RawName, in.RawPublisher, in.RawVersion),
		RawName:      strings.TrimSpace(in.RawName),
		RawPublisher: trimmed(&in.RawPublisher),
		RawVersion:   trimmed(&in.RawVersion),
		ProductID:    in.ProductID,
		VersionID:    in.VersionID,
		Method:       domain.MethodManual,
		ReviewedBy:   &reviewer,
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.assign(ctx, a); err != nil {
			return err
		}
		return s.record(ctx, "rule", a.SourceKey, domain.AuditCreate, "Правило «"+a.RawName+"»",
			map[string]any{"product_id": a.ProductID})
	})
	if err != nil {
		return domain.Alias{}, err
	}
	return s.aliases.BySourceKey(ctx, a.SourceKey)
}

func (s *Service) UpdateRule(ctx context.Context, id, productID uuid.UUID, versionID *uuid.UUID) (domain.Alias, error) {
	c, err := requireAdmin(ctx)
	if err != nil {
		return domain.Alias{}, err
	}
	current, err := s.aliases.Get(ctx, id)
	if err != nil {
		return domain.Alias{}, err
	}
	reviewer := c.Actor()
	a := domain.Assignment{
		SourceKey: current.SourceKey, RawName: current.RawName, RawPublisher: current.RawPublisher,
		RawVersion: current.RawVersion, ProductID: productID, VersionID: versionID,
		Method: domain.MethodManual, ReviewedBy: &reviewer,
	}
	err = s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.assign(ctx, a); err != nil {
			return err
		}
		return s.record(ctx, "rule", id.String(), domain.AuditUpdate, "Правило «"+current.RawName+"»",
			map[string]any{"product_id": map[string]any{"from": current.ProductID, "to": productID}})
	})
	if err != nil {
		return domain.Alias{}, err
	}
	return s.aliases.Get(ctx, id)
}

func (s *Service) DeleteRule(ctx context.Context, id uuid.UUID) error {
	if _, err := requireAdmin(ctx); err != nil {
		return err
	}
	current, err := s.aliases.Get(ctx, id)
	if err != nil {
		return err
	}
	return s.withProducts(ctx, []uuid.UUID{current.ProductID}, func(ctx context.Context) error {
		key, err := s.aliases.Delete(ctx, id)
		if err != nil {
			return err
		}
		devices, err := s.installations.Unassign(ctx, key, domain.MatchRequiresReview)
		if err != nil {
			return err
		}
		if err := s.record(ctx, "rule", id.String(), domain.AuditDelete, "Удалено правило «"+current.RawName+"»", nil); err != nil {
			return err
		}
		return s.recalculate(ctx, domain.Scope{ProductIDs: []uuid.UUID{current.ProductID}, DeviceIDs: devices})
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
