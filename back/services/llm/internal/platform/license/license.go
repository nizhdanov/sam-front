package license

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc"

	licensev1 "sam/gen/go/license/v1"
	"sam/pkg/grpcx"
)

type Config struct {
	GRPCAddr string        `env:"GRPC_ADDR" envDefault:"localhost:9093"`
	Timeout  time.Duration `env:"TIMEOUT" envDefault:"15s"`
}

func Connect(cfg Config) (*grpc.ClientConn, error) {
	return grpcx.Dial(cfg.GRPCAddr, "license.v1.AssistantDataService", cfg.Timeout, nil)
}

type Client struct {
	api licensev1.AssistantDataServiceClient
}

func New(conn grpc.ClientConnInterface) *Client {
	return &Client{api: licensev1.NewAssistantDataServiceClient(conn)}
}

func (c *Client) RunQuery(ctx context.Context, name string, params json.RawMessage) (json.RawMessage, error) {
	resp, err := c.api.RunQuery(ctx, &licensev1.RunQueryRequest{Query: name, Params: params})
	if err != nil {
		return nil, grpcx.FromStatus(err)
	}
	return resp.GetResult(), nil
}
