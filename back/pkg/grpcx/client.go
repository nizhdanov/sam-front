package grpcx

import (
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"sam/pkg/auth"
)

const maxMessageSize = 32 << 20

func Dial(addr, service string, timeout time.Duration, signer *auth.ServiceSigner) (*grpc.ClientConn, error) {
	serviceConfig := fmt.Sprintf(`{
		"methodConfig": [{
			"name": [{"service": %q}],
			"timeout": "%gs",
			"retryPolicy": {"maxAttempts": 3, "initialBackoff": "0.2s", "maxBackoff": "2s",
				"backoffMultiplier": 2, "retryableStatusCodes": ["UNAVAILABLE"]}
		}]}`, service, timeout.Seconds())
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(auth.UnaryClientInterceptor(signer)),
		grpc.WithChainStreamInterceptor(auth.StreamClientInterceptor(signer)),
		grpc.WithDefaultServiceConfig(serviceConfig),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMessageSize), grpc.MaxCallSendMsgSize(maxMessageSize)),
	)
	if err != nil {
		return nil, fmt.Errorf("%s client: %w", service, err)
	}
	return conn, nil
}
