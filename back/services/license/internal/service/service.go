package service

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/google/uuid"

	"sam/pkg/httpx"
	"sam/services/license/internal/domain"
)

type TxRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Locks interface {
	Products(ctx context.Context, ids []uuid.UUID) error
	All(ctx context.Context) error
}

type Products interface {
	List(ctx context.Context, f domain.ProductFilter, page httpx.PageRequest) ([]domain.Product, int, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Product, error)
	GetMany(ctx context.Context, ids []uuid.UUID) ([]domain.Product, error)
	Create(ctx context.Context, p domain.Product) error
	Update(ctx context.Context, id uuid.UUID, p domain.ProductPatch) error
	Delete(ctx context.Context, id uuid.UUID) error
	Usage(ctx context.Context, id uuid.UUID) (installations, contracts int, err error)
	Candidates(ctx context.Context, normalizedName string, limit int) ([]domain.Candidate, error)
	ProductIDsByAgreement(ctx context.Context, agreementID uuid.UUID) ([]uuid.UUID, error)
}

type Versions interface {
	List(ctx context.Context, productID uuid.UUID) ([]domain.Version, error)
	ByProducts(ctx context.Context, productIDs []uuid.UUID) (map[uuid.UUID][]domain.Version, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Version, error)
	Create(ctx context.Context, v domain.Version) error
	SetLifecycle(ctx context.Context, productID, id uuid.UUID, lifecycle domain.Lifecycle) error
}

type Aliases interface {
	List(ctx context.Context, f domain.RuleFilter, page httpx.PageRequest) ([]domain.Alias, int, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Alias, error)
	BySourceKeys(ctx context.Context, keys []string) (map[string]domain.Alias, error)
	BySourceKey(ctx context.Context, key string) (domain.Alias, error)
	Upsert(ctx context.Context, a domain.Assignment) error
	Delete(ctx context.Context, id uuid.UUID) (string, error)
}

type Matching interface {
	StatusByKeys(ctx context.Context, keys []string) (map[string]domain.ReviewStatus, error)
	UpsertPending(ctx context.Context, s domain.Suggestion) error
	List(ctx context.Context, status domain.ReviewStatus, page httpx.PageRequest) ([]domain.MatchingTask, int, error)
	Get(ctx context.Context, id uuid.UUID, lock bool) (domain.MatchingTask, error)
	Approve(ctx context.Context, key string, productID uuid.UUID, reviewer *string) error
	Reject(ctx context.Context, id uuid.UUID, reviewer, reason string) error
}

type Devices interface {
	Get(ctx context.Context, id uuid.UUID) (domain.Device, error)
	BySerial(ctx context.Context, serial string) (domain.Device, error)
	LockBySerial(ctx context.Context, serial string) (uuid.UUID, *time.Time, error)
	MarkInventoried(ctx context.Context, id uuid.UUID, at time.Time, snapshotID uuid.UUID) error
	Update(ctx context.Context, id uuid.UUID, p domain.DevicePatch) error
	MarkProcessed(ctx context.Context, inventoryID, deviceID uuid.UUID, collectedAt time.Time, result domain.SyncResult) error
	Processed(ctx context.Context, inventoryID uuid.UUID) (bool, error)
	History(ctx context.Context, id uuid.UUID) ([]domain.DeviceImport, error)
}

type Installations interface {
	ActiveProductIDs(ctx context.Context, deviceID uuid.UUID) ([]uuid.UUID, error)
	CountActive(ctx context.Context, deviceID uuid.UUID) (int, error)
	Upsert(ctx context.Context, deviceID uuid.UUID, seenAt time.Time, items []domain.InstallationInput) error
	DeactivateMissing(ctx context.Context, deviceID uuid.UUID, keep []string, at time.Time) error
	Assign(ctx context.Context, key string, productID uuid.UUID, versionID *uuid.UUID) ([]uuid.UUID, error)
	Unassign(ctx context.Context, key string, status domain.MatchStatus) ([]uuid.UUID, error)
	DeactivateByKey(ctx context.Context, key string, at time.Time) ([]uuid.UUID, error)
	Facts(ctx context.Context, s domain.Scope) ([]domain.InstallationFacts, error)
	ProductFacts(ctx context.Context, s domain.Scope) ([]domain.ProductFacts, error)
}

type Entitlements interface {
	List(ctx context.Context, f domain.ContractFilter, page httpx.PageRequest) ([]domain.Entitlement, int, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Entitlement, error)
	Active(ctx context.Context, s domain.Scope) (map[uuid.UUID][]domain.Entitlement, error)
	Create(ctx context.Context, e domain.Entitlement) error
	Update(ctx context.Context, id uuid.UUID, p domain.EntitlementPatch) error
	Deactivate(ctx context.Context, id uuid.UUID) error
}

type Agreements interface {
	List(ctx context.Context, page httpx.PageRequest) ([]domain.Agreement, int, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Agreement, error)
	Create(ctx context.Context, a domain.Agreement) error
	Update(ctx context.Context, id uuid.UUID, p domain.AgreementPatch) error
	Delete(ctx context.Context, id uuid.UUID) error
	Similar(ctx context.Context, id uuid.UUID, limit int) ([]domain.SimilarAgreement, error)
	LatestAnalysis(ctx context.Context, agreementID uuid.UUID, contentHash string) (domain.AgreementAnalysis, error)
	SaveAnalysis(ctx context.Context, a domain.AgreementAnalysis) error
	ApproveAnalysis(ctx context.Context, id uuid.UUID, reviewer string) error
}

type Findings interface {
	List(ctx context.Context, f domain.FindingFilter, page httpx.PageRequest) ([]domain.Finding, int, error)
	Get(ctx context.Context, id uuid.UUID) (domain.Finding, error)
	Acknowledge(ctx context.Context, id uuid.UUID, by, comment string) error
	Summary(ctx context.Context, f domain.FindingFilter) (domain.ComplianceSummary, error)
	Apply(ctx context.Context, desired []domain.Finding, types []domain.FindingType, s domain.Scope, now time.Time) (int64, error)
}

type Analytics interface {
	Dashboard(ctx context.Context, expiringWithinDays int) (domain.Dashboard, error)
	FreeSoftware(ctx context.Context) ([]domain.FreeSoftware, error)
}

type Explorer interface {
	Query(ctx context.Context, q domain.DataQuery) (domain.DataTable, error)
}

type Settings interface {
	Get(ctx context.Context, key string, dst any) (bool, error)
	Set(ctx context.Context, key string, value any, by string) error
}

type Audit interface {
	Record(ctx context.Context, e domain.AuditEntry) error
	List(ctx context.Context, f domain.AuditFilter, page httpx.PageRequest) ([]domain.AuditEntry, int, error)
}

type AgreementAnalyzer interface {
	Analyze(ctx context.Context, agreementID uuid.UUID, text string) (analysis []byte, model, promptVersion string, err error)
}

type Deps struct {
	Tx                 TxRunner
	Locks              Locks
	Products           Products
	Versions           Versions
	Aliases            Aliases
	Matching           Matching
	Devices            Devices
	Installations      Installations
	Entitlements       Entitlements
	Agreements         Agreements
	Findings           Findings
	Analytics          Analytics
	Explorer           Explorer
	Settings           Settings
	Audit              Audit
	Analyzer           AgreementAnalyzer
	Log                *slog.Logger
	MatchingConfig     domain.MatchingConfig
	ExpiringWithinDays int
}

type Service struct {
	tx            TxRunner
	locks         Locks
	products      Products
	versions      Versions
	aliases       Aliases
	matching      Matching
	devices       Devices
	installations Installations
	entitlements  Entitlements
	agreements    Agreements
	findings      Findings
	analytics     Analytics
	explorer      Explorer
	settings      Settings
	audit         Audit
	analyzer      AgreementAnalyzer
	log           *slog.Logger
	matchDefaults domain.MatchingConfig
	expiringDays  int
	rematching    atomic.Bool
	now           func() time.Time
}

func New(d Deps) (*Service, error) {
	if d.Tx == nil || d.Locks == nil || d.Products == nil || d.Versions == nil || d.Aliases == nil || d.Matching == nil ||
		d.Devices == nil || d.Installations == nil || d.Entitlements == nil || d.Agreements == nil || d.Findings == nil ||
		d.Analytics == nil || d.Explorer == nil || d.Settings == nil || d.Audit == nil || d.Analyzer == nil || d.Log == nil {
		return nil, errors.New("license service: missing dependency")
	}
	return &Service{
		tx:            d.Tx,
		locks:         d.Locks,
		products:      d.Products,
		versions:      d.Versions,
		aliases:       d.Aliases,
		matching:      d.Matching,
		devices:       d.Devices,
		installations: d.Installations,
		entitlements:  d.Entitlements,
		agreements:    d.Agreements,
		findings:      d.Findings,
		analytics:     d.Analytics,
		explorer:      d.Explorer,
		settings:      d.Settings,
		audit:         d.Audit,
		analyzer:      d.Analyzer,
		log:           d.Log,
		matchDefaults: d.MatchingConfig,
		expiringDays:  d.ExpiringWithinDays,
		now:           time.Now,
	}, nil
}
