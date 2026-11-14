package grpcx

import (
	"log/slog"

	"google.golang.org/grpc"
	grpchealth "google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"

	"sam/pkg/auth"
)

func NewServer(log *slog.Logger, v *auth.Verifier) (*grpc.Server, *grpchealth.Server) {
	srv := grpc.NewServer(
		grpc.MaxRecvMsgSize(maxMessageSize),
		grpc.ChainUnaryInterceptor(
			UnaryRecovery(log),
			UnaryLogging(log),
			UnaryErrors(),
			auth.UnaryServerInterceptor(v),
		),
		grpc.ChainStreamInterceptor(
			StreamRecovery(log),
			StreamLogging(log),
			StreamErrors(),
			auth.StreamServerInterceptor(v),
		),
	)
	hs := grpchealth.NewServer()
	healthpb.RegisterHealthServer(srv, hs)
	return srv, hs
}
