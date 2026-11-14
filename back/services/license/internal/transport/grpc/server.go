package grpc

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"

	licensev1 "sam/gen/go/license/v1"
	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/pkg/grpcx"
	"sam/services/license/internal/domain"
	"sam/services/license/internal/service"
)

func NewServer(log *slog.Logger, v *auth.Verifier, svc *service.Service) (*grpc.Server, *grpchealth.Server) {
	srv, hs := grpcx.NewServer(log, v)
	licensev1.RegisterCatalogServiceServer(srv, &catalogServer{svc: svc})
	licensev1.RegisterInventoryServiceServer(srv, &inventoryServer{svc: svc})
	licensev1.RegisterAssistantDataServiceServer(srv, &assistantServer{svc: svc})
	return srv, hs
}

type catalogServer struct {
	licensev1.UnimplementedCatalogServiceServer
	svc *service.Service
}

func (s *catalogServer) ResolveSoftwareBatch(ctx context.Context, req *licensev1.ResolveSoftwareBatchRequest) (*licensev1.ResolveSoftwareBatchResponse, error) {
	items := make([]domain.RawSoftware, 0, len(req.GetItems()))
	for _, it := range req.GetItems() {
		items = append(items, rawFromProto(it))
	}
	resolutions, err := s.svc.Resolve(ctx, items)
	if err != nil {
		return nil, err
	}
	resp := &licensev1.ResolveSoftwareBatchResponse{Resolutions: make([]*licensev1.Resolution, 0, len(resolutions))}
	for _, r := range resolutions {
		resp.Resolutions = append(resp.Resolutions, &licensev1.Resolution{
			SourceKey:   r.SourceKey,
			Status:      resolutionStatus(r.Status),
			ProductId:   idString(r.ProductID),
			VersionId:   idString(r.VersionID),
			MatchMethod: string(r.Method),
			Candidates:  candidatesToProto(r.Candidates),
		})
	}
	return resp, nil
}

func (s *catalogServer) SaveMatchingSuggestion(ctx context.Context, req *licensev1.SaveMatchingSuggestionRequest) (*licensev1.SaveMatchingSuggestionResponse, error) {
	candidates, err := candidatesFromProto(req.GetCandidates())
	if err != nil {
		return nil, err
	}
	productID, err := optionalID(req.GetSuggestedProductId())
	if err != nil {
		return nil, err
	}
	alias, err := s.svc.SaveSuggestion(ctx, domain.Suggestion{
		Software:   rawFromProto(req.GetSoftware()),
		Candidates: candidates,
		ProductID:  productID,
		Method:     domain.MatchMethod(req.GetMatchMethod()),
		Confidence: req.GetConfidence(),
		Reason:     req.GetReason(),
	})
	if err != nil {
		return nil, err
	}
	if alias == nil {
		return &licensev1.SaveMatchingSuggestionResponse{}, nil
	}
	return &licensev1.SaveMatchingSuggestionResponse{
		ProductId: alias.ProductID.String(),
		VersionId: idString(alias.VersionID),
		Accepted:  true,
	}, nil
}

type inventoryServer struct {
	licensev1.UnimplementedInventoryServiceServer
	svc *service.Service
}

func (s *inventoryServer) SyncInventory(ctx context.Context, req *licensev1.SyncInventoryRequest) (*licensev1.SyncInventoryResponse, error) {
	inventoryID, err := uuid.Parse(req.GetInventoryId())
	if err != nil {
		return nil, apperr.Invalid("inventory_id must be a UUID")
	}
	importID, _ := uuid.Parse(req.GetImportId())
	snap := domain.Snapshot{
		InventoryID:  inventoryID,
		ImportID:     importID,
		SerialNumber: req.GetSerialNumber(),
		CollectedAt:  req.GetCollectedAt().AsTime(),
		FullSnapshot: req.GetFullSnapshot(),
		Items:        make([]domain.InstallationInput, 0, len(req.GetInstallations())),
	}
	for _, it := range req.GetInstallations() {
		productID, err := optionalID(it.GetProductId())
		if err != nil {
			return nil, err
		}
		versionID, err := optionalID(it.GetVersionId())
		if err != nil {
			return nil, err
		}
		snap.Items = append(snap.Items, domain.InstallationInput{
			SourceKey:    it.GetSourceKey(),
			RawName:      it.GetRawName(),
			RawPublisher: it.GetRawPublisher(),
			RawVersion:   it.GetRawVersion(),
			ProductID:    productID,
			VersionID:    versionID,
			MatchStatus:  domain.MatchStatus(it.GetMatchStatus()),
		})
	}
	out, err := s.svc.SyncInventory(ctx, snap)
	if err != nil {
		return nil, err
	}
	return &licensev1.SyncInventoryResponse{
		Result:              syncResult(out.Result),
		DeviceId:            out.DeviceID.String(),
		ActiveInstallations: int32(out.ActiveInstallations),
	}, nil
}

type assistantServer struct {
	licensev1.UnimplementedAssistantDataServiceServer
	svc *service.Service
}

func (s *assistantServer) RunQuery(ctx context.Context, req *licensev1.RunQueryRequest) (*licensev1.RunQueryResponse, error) {
	if _, err := auth.Require(ctx, auth.UserRoles...); err != nil {
		return nil, err
	}
	result, err := s.svc.RunQuery(ctx, req.GetQuery(), req.GetParams())
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}
	return &licensev1.RunQueryResponse{Result: body}, nil
}

func rawFromProto(r *licensev1.RawSoftware) domain.RawSoftware {
	return domain.RawSoftware{
		SourceKey:   r.GetSourceKey(),
		DisplayName: r.GetDisplayName(),
		Publisher:   r.GetPublisher(),
		Version:     r.GetVersion(),
	}
}

func candidatesToProto(list []domain.Candidate) []*licensev1.Candidate {
	out := make([]*licensev1.Candidate, 0, len(list))
	for _, c := range list {
		out = append(out, &licensev1.Candidate{
			ProductId: c.ProductID.String(), Name: c.Name, Publisher: c.Publisher, Edition: c.Edition, Similarity: c.Similarity,
		})
	}
	return out
}

func candidatesFromProto(list []*licensev1.Candidate) ([]domain.Candidate, error) {
	out := make([]domain.Candidate, 0, len(list))
	for _, c := range list {
		id, err := uuid.Parse(c.GetProductId())
		if err != nil {
			return nil, apperr.Invalid("candidate product_id must be a UUID")
		}
		out = append(out, domain.Candidate{
			ProductID: id, Name: c.GetName(), Publisher: c.GetPublisher(), Edition: c.GetEdition(), Similarity: c.GetSimilarity(),
		})
	}
	return out, nil
}

func optionalID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, apperr.Invalid("malformed UUID " + raw)
	}
	return &id, nil
}

func idString(id *uuid.UUID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func resolutionStatus(s domain.ResolutionStatus) licensev1.ResolutionStatus {
	switch s {
	case domain.ResolutionMatched:
		return licensev1.ResolutionStatus_RESOLUTION_STATUS_MATCHED
	case domain.ResolutionAmbiguous:
		return licensev1.ResolutionStatus_RESOLUTION_STATUS_AMBIGUOUS
	case domain.ResolutionPendingReview:
		return licensev1.ResolutionStatus_RESOLUTION_STATUS_PENDING_REVIEW
	default:
		return licensev1.ResolutionStatus_RESOLUTION_STATUS_UNKNOWN
	}
}

func syncResult(r domain.SyncResult) licensev1.SyncResult {
	switch r {
	case domain.SyncApplied:
		return licensev1.SyncResult_SYNC_RESULT_APPLIED
	case domain.SyncAlreadyProcessed:
		return licensev1.SyncResult_SYNC_RESULT_ALREADY_PROCESSED
	case domain.SyncSuperseded:
		return licensev1.SyncResult_SYNC_RESULT_SUPERSEDED
	default:
		return licensev1.SyncResult_SYNC_RESULT_UNSPECIFIED
	}
}
