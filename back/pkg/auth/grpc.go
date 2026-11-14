package auth

import (
	"context"
	"strings"

	"github.com/go-chi/chi/v5/middleware"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const requestIDHeader = "x-request-id"

var PublicMethods = []string{"/grpc.health.v1.Health/"}

func isPublic(method string) bool {
	for _, p := range PublicMethods {
		if strings.HasPrefix(method, p) {
			return true
		}
	}
	return false
}

func authenticate(ctx context.Context, v *Verifier) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	if ids := md.Get(requestIDHeader); len(ids) > 0 {
		ctx = context.WithValue(ctx, middleware.RequestIDKey, ids[0])
	}
	vals := md.Get("authorization")
	if len(vals) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing bearer token")
	}
	raw, ok := BearerToken(vals[0])
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "malformed authorization header")
	}
	claims, err := v.Verify(raw)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid or expired token")
	}
	if claims.Role.IsUser() {
		ctx = WithToken(ctx, raw)
	}
	return WithClaims(ctx, claims), nil
}

func UnaryServerInterceptor(v *Verifier) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		if isPublic(info.FullMethod) {
			return handler(ctx, req)
		}
		ctx, err := authenticate(ctx, v)
		if err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func StreamServerInterceptor(v *Verifier) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if isPublic(info.FullMethod) {
			return handler(srv, ss)
		}
		ctx, err := authenticate(ss.Context(), v)
		if err != nil {
			return err
		}
		return handler(srv, &wrappedStream{ServerStream: ss, ctx: ctx})
	}
}

type wrappedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedStream) Context() context.Context { return w.ctx }

func outgoing(ctx context.Context, service *ServiceSigner) (context.Context, error) {
	if id := middleware.GetReqID(ctx); id != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, requestIDHeader, id)
	}
	if tok, ok := TokenFrom(ctx); ok {
		return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok), nil
	}
	if service == nil {
		return ctx, nil
	}
	tok, err := service.Token()
	if err != nil {
		return nil, err
	}
	return metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+tok), nil
}

func UnaryClientInterceptor(service *ServiceSigner) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, err := outgoing(ctx, service)
		if err != nil {
			return err
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

func StreamClientInterceptor(service *ServiceSigner) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx, err := outgoing(ctx, service)
		if err != nil {
			return nil, err
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}
