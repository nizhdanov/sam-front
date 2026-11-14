package llm

import (
	"context"
	"errors"
	"io"
	"time"

	"google.golang.org/grpc"

	llmv1 "sam/gen/go/llm/v1"
	"sam/pkg/apperr"
	"sam/pkg/auth"
	"sam/pkg/grpcx"
	"sam/services/chat/internal/domain"
)

type Config struct {
	GRPCAddr string        `env:"GRPC_ADDR" envDefault:"localhost:9095"`
	Timeout  time.Duration `env:"TIMEOUT" envDefault:"180s"`
}

func Connect(cfg Config, signer *auth.ServiceSigner) (*grpc.ClientConn, error) {
	return grpcx.Dial(cfg.GRPCAddr, "llm.v1.LLMService", cfg.Timeout, signer)
}

type Client struct{ api llmv1.LLMServiceClient }

func New(conn grpc.ClientConnInterface) *Client {
	return &Client{api: llmv1.NewLLMServiceClient(conn)}
}

func (c *Client) Chat(ctx context.Context, conversationID string, history []domain.Message, message string, onDelta func(string) error) error {
	req := &llmv1.ChatRequest{ConversationId: conversationID, Message: message}
	for _, m := range history {
		req.History = append(req.History, &llmv1.ChatMessage{Role: string(m.Role), Content: m.Content})
	}
	stream, err := c.api.Chat(ctx, req)
	if err != nil {
		return grpcx.FromStatus(err)
	}
	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return grpcx.FromStatus(err)
		}
		switch ev := resp.GetEvent().(type) {
		case *llmv1.ChatResponse_Delta:
			if err := onDelta(ev.Delta); err != nil {
				return err
			}
		case *llmv1.ChatResponse_Error:
			return apperr.Unavailable(ev.Error)
		case *llmv1.ChatResponse_Done:
			return nil
		}
	}
}
