package llm

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	llmv1 "sam/gen/go/llm/v1"
	"sam/pkg/auth"
	"sam/pkg/grpcx"
	"sam/services/license/internal/domain"
)

type Config struct {
	GRPCAddr string        `env:"GRPC_ADDR" envDefault:"localhost:9095"`
	Timeout  time.Duration `env:"TIMEOUT" envDefault:"120s"`
}

func Connect(cfg Config, signer *auth.ServiceSigner) (*grpc.ClientConn, error) {
	return grpcx.Dial(cfg.GRPCAddr, "llm.v1.LLMService", cfg.Timeout, signer)
}

type Client struct{ api llmv1.LLMServiceClient }

func New(conn grpc.ClientConnInterface) *Client {
	return &Client{api: llmv1.NewLLMServiceClient(conn)}
}

func (c *Client) Analyze(ctx context.Context, agreementID uuid.UUID, text string) ([]byte, string, string, error) {
	resp, err := c.api.AnalyzeAgreement(ctx, &llmv1.AnalyzeAgreementRequest{AgreementId: agreementID.String(), AgreementText: text})
	if err != nil {
		return nil, "", "", grpcx.FromStatus(err)
	}
	return resp.GetAnalysisJson(), resp.GetModel(), resp.GetPromptVersion(), nil
}

func (c *Client) Normalize(ctx context.Context, items []domain.NormalizeInput) ([]domain.Normalized, string, error) {
	req := &llmv1.NormalizeSoftwareRequest{Items: make([]*llmv1.RawSoftwareName, 0, len(items))}
	for _, it := range items {
		req.Items = append(req.Items, &llmv1.RawSoftwareName{Key: it.Key, DisplayName: it.DisplayName, Publisher: it.Publisher})
	}
	resp, err := c.api.NormalizeSoftware(ctx, req)
	if err != nil {
		return nil, "", grpcx.FromStatus(err)
	}
	out := make([]domain.Normalized, 0, len(resp.GetItems()))
	for _, n := range resp.GetItems() {
		out = append(out, domain.Normalized{Key: n.GetKey(), CanonicalName: n.GetCanonicalName(), Publisher: n.GetPublisher(), IsSoftware: n.GetIsSoftware()})
	}
	return out, resp.GetModel(), nil
}
