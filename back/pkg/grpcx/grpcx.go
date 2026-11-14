package grpcx

import (
	"context"
	"errors"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"sam/pkg/apperr"
)

func CodeFor(k apperr.Kind) codes.Code {
	switch k {
	case apperr.KindInvalid:
		return codes.InvalidArgument
	case apperr.KindNotFound:
		return codes.NotFound
	case apperr.KindConflict:
		return codes.AlreadyExists
	case apperr.KindUnauthenticated:
		return codes.Unauthenticated
	case apperr.KindForbidden:
		return codes.PermissionDenied
	case apperr.KindUnavailable:
		return codes.Unavailable
	case apperr.KindRateLimited, apperr.KindTooLarge:
		return codes.ResourceExhausted
	case apperr.KindTimeout:
		return codes.DeadlineExceeded
	default:
		return codes.Internal
	}
}

func KindFor(c codes.Code) apperr.Kind {
	switch c {
	case codes.InvalidArgument, codes.FailedPrecondition, codes.OutOfRange:
		return apperr.KindInvalid
	case codes.NotFound:
		return apperr.KindNotFound
	case codes.AlreadyExists, codes.Aborted:
		return apperr.KindConflict
	case codes.Unauthenticated:
		return apperr.KindUnauthenticated
	case codes.PermissionDenied:
		return apperr.KindForbidden
	case codes.Unavailable, codes.Canceled:
		return apperr.KindUnavailable
	case codes.ResourceExhausted:
		return apperr.KindRateLimited
	case codes.DeadlineExceeded:
		return apperr.KindTimeout
	default:
		return apperr.KindInternal
	}
}

func ToStatus(err error) error {
	if err == nil {
		return nil
	}
	if _, ok := status.FromError(err); ok {
		return err
	}
	switch {
	case errors.Is(err, context.Canceled):
		return status.Error(codes.Canceled, "request canceled")
	case errors.Is(err, context.DeadlineExceeded):
		return status.Error(codes.DeadlineExceeded, "deadline exceeded")
	}
	return status.Error(CodeFor(apperr.KindOf(err)), apperr.Message(err))
}

func FromStatus(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return err
	}
	return apperr.Wrap(err, KindFor(st.Code()), st.Message())
}

func IsTransient(err error) bool {
	switch apperr.KindOf(err) {
	case apperr.KindUnavailable, apperr.KindTimeout, apperr.KindInternal:
		return true
	}
	return false
}

func UnaryErrors() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, _ *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		resp, err := h(ctx, req)
		return resp, ToStatus(err)
	}
}

func StreamErrors() grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, _ *grpc.StreamServerInfo, h grpc.StreamHandler) error {
		return ToStatus(h(srv, ss))
	}
}

func UnaryLogging(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := h(ctx, req)
		logCall(ctx, log, info.FullMethod, start, err)
		return resp, err
	}
}

func StreamLogging(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, h grpc.StreamHandler) error {
		start := time.Now()
		err := h(srv, ss)
		logCall(ss.Context(), log, info.FullMethod, start, err)
		return err
	}
}

func logCall(ctx context.Context, log *slog.Logger, method string, start time.Time, err error) {
	code := status.Code(err)
	attrs := []any{"method", method, "code", code.String(), "duration_ms", time.Since(start).Milliseconds()}
	switch code {
	case codes.OK:
		log.DebugContext(ctx, "grpc request", attrs...)
	case codes.Internal, codes.Unknown, codes.DataLoss:
		log.ErrorContext(ctx, "grpc request", append(attrs, "err", err)...)
	default:
		log.WarnContext(ctx, "grpc request", append(attrs, "err", err)...)
	}
}

func UnaryRecovery(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if p := recover(); p != nil {
				log.ErrorContext(ctx, "grpc panic", "method", info.FullMethod, "panic", p, "stack", string(debug.Stack()))
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return h(ctx, req)
	}
}

func StreamRecovery(log *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, h grpc.StreamHandler) (err error) {
		defer func() {
			if p := recover(); p != nil {
				log.ErrorContext(ss.Context(), "grpc panic", "method", info.FullMethod, "panic", p, "stack", string(debug.Stack()))
				err = status.Error(codes.Internal, "internal error")
			}
		}()
		return h(srv, ss)
	}
}
