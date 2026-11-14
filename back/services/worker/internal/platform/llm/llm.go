package llm

import (
	"context"
	"time"

	"google.golang.org/grpc"

	llmv1 "sam/gen/go/llm/v1"
	"sam/pkg/auth"
	"sam/pkg/grpcx"
	"sam/services/worker/internal/domain"
)

type Config struct {
	Enabled  bool          `env:"ENABLED" envDefault:"true"`
	GRPCAddr string        `env:"GRPC_ADDR" envDefault:"localhost:9095"`
	Timeout  time.Duration `env:"TIMEOUT" envDefault:"60s"`
}

func Connect(cfg Config, signer *auth.ServiceSigner) (*grpc.ClientConn, error) {
	return grpcx.Dial(cfg.GRPCAddr, "llm.v1.LLMService", cfg.Timeout, signer)
}

type Client struct{ api llmv1.LLMServiceClient }

func New(conn grpc.ClientConnInterface) *Client {
	return &Client{api: llmv1.NewLLMServiceClient(conn)}
}

func (c *Client) Match(ctx context.Context, item domain.Item, candidates []domain.Candidate) (domain.Suggestion, error) {
	req := &llmv1.MatchSoftwareRequest{
		DisplayName: item.DisplayName,
		Publisher:   item.Publisher,
		Version:     item.Version,
		Candidates:  make([]*llmv1.SoftwareCandidate, 0, len(candidates)),
	}
	for _, cand := range candidates {
		req.Candidates = append(req.Candidates, &llmv1.SoftwareCandidate{
			Id: cand.ProductID, Name: cand.Name, Publisher: cand.Publisher, Edition: cand.Edition,
		})
	}
	resp, err := c.api.MatchSoftware(ctx, req)
	if err != nil {
		return domain.Suggestion{}, grpcx.FromStatus(err)
	}
	s := domain.Suggestion{Confidence: resp.GetConfidence(), Reason: resp.GetReason(), Model: resp.GetModel()}
	if resp.GetStatus() == "MATCHED" {
		s.ProductID = resp.GetProductId()
	}
	return s, nil
}
