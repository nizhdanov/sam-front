package llm

import (
	"context"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"

	llmv1 "sam/gen/go/llm/v1"
	"sam/pkg/auth"
	"sam/pkg/grpcx"
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
