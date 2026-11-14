package license

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	licensev1 "sam/gen/go/license/v1"
	"sam/pkg/auth"
	"sam/pkg/grpcx"
	"sam/services/worker/internal/domain"
)

type Config struct {
	GRPCAddr string        `env:"GRPC_ADDR" envDefault:"localhost:9093"`
	Timeout  time.Duration `env:"TIMEOUT" envDefault:"60s"`
}

func Connect(cfg Config, signer *auth.ServiceSigner) (*grpc.ClientConn, error) {
	return grpcx.Dial(cfg.GRPCAddr, "license.v1.InventoryService", cfg.Timeout, signer)
}

type Client struct {
	catalog   licensev1.CatalogServiceClient
	inventory licensev1.InventoryServiceClient
}

func New(conn grpc.ClientConnInterface) *Client {
	return &Client{
		catalog:   licensev1.NewCatalogServiceClient(conn),
		inventory: licensev1.NewInventoryServiceClient(conn),
	}
}

func (c *Client) Resolve(ctx context.Context, items []domain.Item) ([]domain.Resolution, error) {
	req := &licensev1.ResolveSoftwareBatchRequest{Items: make([]*licensev1.RawSoftware, 0, len(items))}
	for _, it := range items {
		req.Items = append(req.Items, rawToProto(it))
	}
	resp, err := c.catalog.ResolveSoftwareBatch(ctx, req)
	if err != nil {
		return nil, grpcx.FromStatus(err)
	}
	out := make([]domain.Resolution, 0, len(resp.GetResolutions()))
	for _, r := range resp.GetResolutions() {
		res := domain.Resolution{
			SourceKey: r.GetSourceKey(),
			Status:    resolutionStatus(r.GetStatus()),
			ProductID: r.GetProductId(),
			VersionID: r.GetVersionId(),
		}
		for _, c := range r.GetCandidates() {
			res.Candidates = append(res.Candidates, domain.Candidate{
				ProductID: c.GetProductId(), Name: c.GetName(), Publisher: c.GetPublisher(), Edition: c.GetEdition(), Similarity: c.GetSimilarity(),
			})
		}
		out = append(out, res)
	}
	return out, nil
}

func (c *Client) SaveSuggestion(ctx context.Context, item domain.Item, candidates []domain.Candidate, s domain.Suggestion, method string) (domain.Match, error) {
	req := &licensev1.SaveMatchingSuggestionRequest{
		Software:           rawToProto(item),
		SuggestedProductId: s.ProductID,
		MatchMethod:        method,
		Confidence:         s.Confidence,
		Reason:             s.Reason,
	}
	for _, cand := range candidates {
		req.Candidates = append(req.Candidates, &licensev1.Candidate{
			ProductId: cand.ProductID, Name: cand.Name, Publisher: cand.Publisher, Edition: cand.Edition, Similarity: cand.Similarity,
		})
	}
	resp, err := c.catalog.SaveMatchingSuggestion(ctx, req)
	if err != nil {
		return domain.Match{}, grpcx.FromStatus(err)
	}
	if !resp.GetAccepted() {
		return domain.Match{Status: domain.MatchRequiresReview}, nil
	}
	return domain.Match{ProductID: resp.GetProductId(), VersionID: resp.GetVersionId(), Status: domain.MatchMatched}, nil
}

func (c *Client) Sync(ctx context.Context, s domain.Snapshot, installations []domain.Installation) (domain.SyncResult, error) {
	req := &licensev1.SyncInventoryRequest{
		InventoryId:   s.ID.String(),
		ImportId:      s.ImportID.String(),
		SerialNumber:  s.SerialNumber,
		CollectedAt:   timestamppb.New(s.CollectedAt),
		FullSnapshot:  s.Full,
		Installations: make([]*licensev1.InstallationInput, 0, len(installations)),
	}
	for _, in := range installations {
		req.Installations = append(req.Installations, &licensev1.InstallationInput{
			SourceKey:    in.SourceKey,
			RawName:      in.DisplayName,
			RawPublisher: in.Publisher,
			RawVersion:   in.Version,
			ProductId:    in.ProductID,
			VersionId:    in.VersionID,
			MatchStatus:  string(in.Status),
		})
	}
	resp, err := c.inventory.SyncInventory(ctx, req)
	if err != nil {
		return "", grpcx.FromStatus(err)
	}
	switch resp.GetResult() {
	case licensev1.SyncResult_SYNC_RESULT_SUPERSEDED:
		return domain.SyncSuperseded, nil
	case licensev1.SyncResult_SYNC_RESULT_ALREADY_PROCESSED:
		return domain.SyncAlreadyProcessed, nil
	default:
		return domain.SyncApplied, nil
	}
}

func rawToProto(it domain.Item) *licensev1.RawSoftware {
	return &licensev1.RawSoftware{SourceKey: it.SourceKey, DisplayName: it.DisplayName, Publisher: it.Publisher, Version: it.Version}
}

func resolutionStatus(s licensev1.ResolutionStatus) domain.ResolutionStatus {
	switch s {
	case licensev1.ResolutionStatus_RESOLUTION_STATUS_MATCHED:
		return domain.ResolutionMatched
	case licensev1.ResolutionStatus_RESOLUTION_STATUS_AMBIGUOUS:
		return domain.ResolutionAmbiguous
	case licensev1.ResolutionStatus_RESOLUTION_STATUS_PENDING_REVIEW:
		return domain.ResolutionPendingReview
	default:
		return domain.ResolutionUnknown
	}
}
