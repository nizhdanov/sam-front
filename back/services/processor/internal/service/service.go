package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/pkg/httpx"
	"sam/services/processor/internal/domain"
	"sam/services/processor/internal/parser"
)

const (
	maxStoredRowErrors = 1000
	previewRows        = 20
)

type TxRunner interface {
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type Imports interface {
	Create(ctx context.Context, i domain.Import) error
	Get(ctx context.Context, id uuid.UUID) (domain.Import, error)
	List(ctx context.Context, page httpx.PageRequest) ([]domain.Import, int, error)
	ByStatus(ctx context.Context, statuses ...domain.ImportStatus) ([]domain.Import, error)
	SetStatus(ctx context.Context, id uuid.UUID, status domain.ImportStatus, message *string) error
	SetParsed(ctx context.Context, id uuid.UUID, rows int, stats domain.RowStats, devices int) error
	Expired(ctx context.Context, before time.Time) ([]uuid.UUID, error)
	ClearStorage(ctx context.Context, id uuid.UUID) error
}

type Snapshots interface {
	Insert(ctx context.Context, snapshots []domain.Snapshot) error
	Unpublished(ctx context.Context, importID uuid.UUID) ([]domain.Snapshot, error)
	ResetFailed(ctx context.Context, importID uuid.UUID) (int64, error)
	MarkPublished(ctx context.Context, id uuid.UUID) error
	Progress(ctx context.Context, importID uuid.UUID) (domain.Progress, error)
}

type ImportErrors interface {
	Insert(ctx context.Context, importID uuid.UUID, errs []domain.ImportError) error
	List(ctx context.Context, importID uuid.UUID, page httpx.PageRequest) ([]domain.ImportError, int, error)
}

type Files interface {
	Save(importID uuid.UUID, r io.Reader, maxBytes int64) (string, int64, error)
	Open(importID uuid.UUID) (io.ReadCloser, error)
	Remove(importID uuid.UUID) error
}

type Publisher interface {
	Publish(ctx context.Context, s domain.Snapshot, attempt int) error
}

type Config struct {
	MaxFileBytes       int64
	MaxRows            int
	MaxFieldBytes      int
	MaxConcurrentLoads int
	ParseConcurrency   int
	Retention          time.Duration
}

type Deps struct {
	Tx        TxRunner
	Imports   Imports
	Snapshots Snapshots
	Errors    ImportErrors
	Files     Files
	Publisher Publisher
	Log       *slog.Logger
}

type Service struct {
	tx        TxRunner
	imports   Imports
	snapshots Snapshots
	errors    ImportErrors
	files     Files
	publisher Publisher
	log       *slog.Logger
	cfg       Config
	uploads   chan struct{}
	queue     chan uuid.UUID
}

func New(d Deps, cfg Config) (*Service, error) {
	if d.Tx == nil || d.Imports == nil || d.Snapshots == nil || d.Errors == nil || d.Files == nil || d.Publisher == nil || d.Log == nil {
		return nil, errors.New("processor service: missing dependency")
	}
	return &Service{
		tx:        d.Tx,
		imports:   d.Imports,
		snapshots: d.Snapshots,
		errors:    d.Errors,
		files:     d.Files,
		publisher: d.Publisher,
		log:       d.Log,
		cfg:       cfg,
		uploads:   make(chan struct{}, cfg.MaxConcurrentLoads),
		queue:     make(chan uuid.UUID, 64),
	}, nil
}

type ImportView struct {
	domain.Import
	Progress domain.Progress
}

type UploadResult struct {
	Import  domain.Import
	Preview parser.Preview
}

func (s *Service) Upload(ctx context.Context, filename string, body io.Reader, collectedAt time.Time, confirm bool) (UploadResult, error) {
	claims, err := auth.Require(ctx, auth.RoleAdmin)
	if err != nil {
		return UploadResult{}, err
	}
	select {
	case s.uploads <- struct{}{}:
		defer func() { <-s.uploads }()
	default:
		return UploadResult{}, apperr.RateLimited("too many uploads in progress, try again later")
	}

	if collectedAt.IsZero() {
		collectedAt = time.Now().UTC()
	}
	imp := domain.Import{
		ID:          uuid.New(),
		Filename:    sanitizeFilename(filename),
		Source:      domain.SourceCSV,
		Status:      domain.ImportAwaiting,
		CreatedBy:   claims.Actor(),
		CollectedAt: collectedAt,
	}
	if confirm {
		imp.Status = domain.ImportPending
	}
	imp.StoragePath, imp.FileSize, err = s.files.Save(imp.ID, body, s.cfg.MaxFileBytes)
	if err != nil {
		return UploadResult{}, err
	}
	preview, err := s.preview(imp.ID)
	if err != nil {
		_ = s.files.Remove(imp.ID)
		return UploadResult{}, apperr.Invalid(err.Error())
	}
	if err := s.imports.Create(ctx, imp); err != nil {
		_ = s.files.Remove(imp.ID)
		return UploadResult{}, err
	}
	if confirm {
		s.enqueue(imp.ID)
	}
	return UploadResult{Import: imp, Preview: preview}, nil
}

func (s *Service) preview(id uuid.UUID) (parser.Preview, error) {
	file, err := s.files.Open(id)
	if err != nil {
		return parser.Preview{}, err
	}
	defer func() { _ = file.Close() }()
	return parser.ReadPreview(file, previewRows, s.cfg.MaxFieldBytes)
}

func (s *Service) Preview(ctx context.Context, id uuid.UUID) (parser.Preview, error) {
	if _, err := auth.Require(ctx, auth.RoleAdmin); err != nil {
		return parser.Preview{}, err
	}
	imp, err := s.imports.Get(ctx, id)
	if err != nil {
		return parser.Preview{}, err
	}
	if imp.StoragePath == "" {
		return parser.Preview{}, apperr.Conflict("import file is no longer stored")
	}
	return s.preview(id)
}

func (s *Service) Confirm(ctx context.Context, id uuid.UUID) (ImportView, error) {
	if _, err := auth.Require(ctx, auth.RoleAdmin); err != nil {
		return ImportView{}, err
	}
	imp, err := s.imports.Get(ctx, id)
	if err != nil {
		return ImportView{}, err
	}
	if imp.Status != domain.ImportAwaiting {
		return ImportView{}, apperr.Conflict("import is not awaiting confirmation")
	}
	if err := s.imports.SetStatus(ctx, id, domain.ImportPending, nil); err != nil {
		return ImportView{}, err
	}
	s.enqueue(id)
	return s.Get(ctx, id)
}

func (s *Service) Cancel(ctx context.Context, id uuid.UUID) (ImportView, error) {
	if _, err := auth.Require(ctx, auth.RoleAdmin); err != nil {
		return ImportView{}, err
	}
	imp, err := s.imports.Get(ctx, id)
	if err != nil {
		return ImportView{}, err
	}
	if imp.Status != domain.ImportAwaiting {
		return ImportView{}, apperr.Conflict("only an import awaiting confirmation can be cancelled")
	}
	if err := s.imports.SetStatus(ctx, id, domain.ImportCancelled, nil); err != nil {
		return ImportView{}, err
	}
	if err := s.files.Remove(id); err != nil {
		s.log.ErrorContext(ctx, "remove cancelled import file", "import_id", id, "err", err)
	} else if err := s.imports.ClearStorage(ctx, id); err != nil {
		s.log.ErrorContext(ctx, "clear cancelled import storage", "import_id", id, "err", err)
	}
	return s.Get(ctx, id)
}

func (s *Service) enqueue(id uuid.UUID) {
	select {
	case s.queue <- id:
	default:
		s.log.Warn("import queue is full, import stays pending until retry", "import_id", id)
	}
}

func (s *Service) RunWorkers(ctx context.Context) error {
	pending, err := s.imports.ByStatus(ctx, domain.ImportPending, domain.ImportParsing)
	if err != nil {
		return err
	}
	for _, imp := range pending {
		s.enqueue(imp.ID)
	}
	done := make(chan struct{})
	for range max(1, s.cfg.ParseConcurrency) {
		go func() {
			defer func() { done <- struct{}{} }()
			for {
				select {
				case <-ctx.Done():
					return
				case id := <-s.queue:
					s.process(ctx, id)
				}
			}
		}()
	}
	for range max(1, s.cfg.ParseConcurrency) {
		<-done
	}
	return nil
}

func (s *Service) process(ctx context.Context, id uuid.UUID) {
	log := s.log.With("import_id", id)
	start := time.Now()
	if err := s.parse(ctx, id); err != nil {
		log.Error("import parsing failed", "err", err)
		msg := apperr.Message(err)
		if errors.Is(err, parser.ErrCritical) {
			msg = err.Error()
		}
		if err := s.imports.SetStatus(ctx, id, domain.ImportFailed, &msg); err != nil {
			log.Error("mark import failed", "err", err)
		}
		return
	}
	published, failed := s.publish(ctx, id, 0)
	log.Info("import parsed", "published", published, "publish_failed", failed, "duration_ms", time.Since(start).Milliseconds())
}

func (s *Service) parse(ctx context.Context, id uuid.UUID) error {
	imp, err := s.imports.Get(ctx, id)
	if err != nil {
		return err
	}
	if err := s.imports.SetStatus(ctx, id, domain.ImportParsing, nil); err != nil {
		return err
	}
	file, err := s.files.Open(id)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	result, err := parser.Parse(file, parser.Limits{MaxRows: s.cfg.MaxRows, MaxFieldBytes: s.cfg.MaxFieldBytes})
	rowErrors := toImportErrors(result.Errors)
	if err != nil {
		if insertErr := s.errors.Insert(ctx, id, append(rowErrors, domain.ImportError{Code: "CRITICAL_FORMAT_ERROR", Message: err.Error()})); insertErr != nil {
			s.log.Error("store import errors", "import_id", id, "err", insertErr)
		}
		return err
	}

	snapshotType := domain.SnapshotFull
	if result.Orphans > 0 {
		snapshotType = domain.SnapshotPartial
	}
	stats := domain.RowStats{DuplicateRows: result.Duplicates, ExcludedRows: result.Excluded, ErrorRows: len(result.Errors)}
	snapshots := make([]domain.Snapshot, 0, len(result.Devices))
	for serial, software := range result.Devices {
		stats.LoadedRows += len(software)
		snapshots = append(snapshots, domain.Snapshot{
			ID:           uuid.New(),
			ImportID:     id,
			SerialNumber: serial,
			CollectedAt:  imp.CollectedAt,
			Type:         snapshotType,
			Software:     software,
		})
	}

	return s.tx.InTx(ctx, func(ctx context.Context) error {
		if err := s.snapshots.Insert(ctx, snapshots); err != nil {
			return err
		}
		if err := s.errors.Insert(ctx, id, rowErrors); err != nil {
			return err
		}
		return s.imports.SetParsed(ctx, id, result.TotalRows, stats, len(snapshots))
	})
}

func toImportErrors(rows []parser.RowError) []domain.ImportError {
	out := make([]domain.ImportError, 0, min(len(rows), maxStoredRowErrors))
	for _, e := range rows[:min(len(rows), maxStoredRowErrors)] {
		row := e.Row
		out = append(out, domain.ImportError{RowNumber: &row, Code: e.Code, Message: e.Message})
	}
	return out
}

func (s *Service) publish(ctx context.Context, importID uuid.UUID, attempt int) (published, failed int) {
	snapshots, err := s.snapshots.Unpublished(ctx, importID)
	if err != nil {
		s.log.Error("load unpublished snapshots", "import_id", importID, "err", err)
		return 0, 0
	}
	for _, snap := range snapshots {
		if err := s.publisher.Publish(ctx, snap, attempt); err != nil {
			s.log.Warn("snapshot left unpublished", "inventory_id", snap.ID, "err", err)
			failed++
			continue
		}
		if err := s.snapshots.MarkPublished(ctx, snap.ID); err != nil {
			s.log.Error("mark snapshot published", "inventory_id", snap.ID, "err", err)
		}
		published++
	}
	return published, failed
}

func (s *Service) Retry(ctx context.Context, id uuid.UUID) (ImportView, error) {
	if _, err := auth.Require(ctx, auth.RoleAdmin); err != nil {
		return ImportView{}, err
	}
	imp, err := s.imports.Get(ctx, id)
	if err != nil {
		return ImportView{}, err
	}
	if imp.Status == domain.ImportAwaiting || imp.Status == domain.ImportCancelled {
		return ImportView{}, apperr.Conflict("import is not confirmed")
	}
	progress, err := s.snapshots.Progress(ctx, id)
	if err != nil {
		return ImportView{}, err
	}

	if progress.Total == 0 {
		if imp.Source != domain.SourceCSV || imp.StoragePath == "" {
			return ImportView{}, apperr.Conflict("import has no stored file to reprocess")
		}
		if err := s.imports.SetStatus(ctx, id, domain.ImportPending, nil); err != nil {
			return ImportView{}, err
		}
		s.enqueue(id)
		return s.Get(ctx, id)
	}

	if _, err := s.snapshots.ResetFailed(ctx, id); err != nil {
		return ImportView{}, err
	}
	if err := s.imports.SetStatus(ctx, id, domain.ImportProcessing, nil); err != nil {
		return ImportView{}, err
	}
	published, failed := s.publish(ctx, id, int(time.Now().Unix()))
	s.log.InfoContext(ctx, "import retried", "import_id", id, "published", published, "publish_failed", failed)
	if failed > 0 {
		return ImportView{}, apperr.Unavailable(fmt.Sprintf("%d snapshot(s) could not be queued, message broker is unavailable", failed))
	}
	return s.Get(ctx, id)
}

func (s *Service) Get(ctx context.Context, id uuid.UUID) (ImportView, error) {
	if _, err := auth.Require(ctx, auth.RoleAdmin); err != nil {
		return ImportView{}, err
	}
	imp, err := s.imports.Get(ctx, id)
	if err != nil {
		return ImportView{}, err
	}
	return s.view(ctx, imp)
}

func (s *Service) view(ctx context.Context, imp domain.Import) (ImportView, error) {
	progress, err := s.snapshots.Progress(ctx, imp.ID)
	if err != nil {
		return ImportView{}, err
	}
	derived := progress.Derive(imp.Status)
	if derived != imp.Status {
		if err := s.imports.SetStatus(ctx, imp.ID, derived, imp.ErrorMessage); err != nil {
			return ImportView{}, err
		}
		imp.Status = derived
		if derived.Terminal() {
			now := time.Now().UTC()
			imp.CompletedAt = &now
		}
	}
	return ImportView{Import: imp, Progress: progress}, nil
}

func (s *Service) List(ctx context.Context, page httpx.PageRequest) ([]ImportView, int, error) {
	if _, err := auth.Require(ctx, auth.RoleAdmin); err != nil {
		return nil, 0, err
	}
	imports, total, err := s.imports.List(ctx, page)
	if err != nil {
		return nil, 0, err
	}
	out := make([]ImportView, 0, len(imports))
	for _, imp := range imports {
		v, err := s.view(ctx, imp)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, nil
}

func (s *Service) Errors(ctx context.Context, id uuid.UUID, page httpx.PageRequest) ([]domain.ImportError, int, error) {
	if _, err := auth.Require(ctx, auth.RoleAdmin); err != nil {
		return nil, 0, err
	}
	if _, err := s.imports.Get(ctx, id); err != nil {
		return nil, 0, err
	}
	return s.errors.List(ctx, id, page)
}

func (s *Service) RunRetention(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			ids, err := s.imports.Expired(ctx, time.Now().Add(-s.cfg.Retention))
			if err != nil {
				s.log.Error("find expired imports", "err", err)
				continue
			}
			for _, id := range ids {
				if err := s.files.Remove(id); err != nil {
					s.log.Error("remove import file", "import_id", id, "err", err)
					continue
				}
				if err := s.imports.ClearStorage(ctx, id); err != nil {
					s.log.Error("clear import storage", "import_id", id, "err", err)
				}
			}
		}
	}
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return "inventory.csv"
	}
	if len(name) > 255 {
		name = name[:255]
	}
	return name
}
