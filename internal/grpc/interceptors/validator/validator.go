package validator

import (
	"context"
	"log/slog"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// UnaryServerInterceptor returns a new unary server interceptor that validates messages.
func UnaryServerInterceptor(l *slog.Logger) grpc.UnaryServerInterceptor {
	const op = "grpc_auth.validator.interceptor"

	log := l.With(slog.String("op", op))

	v, err := protovalidate.New()
	if err != nil {
		panic("failed to initialize validator: " + err.Error())
	}

	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		msg, ok := req.(proto.Message)
		if !ok {
			return handler(ctx, req)
		}

		if err := v.Validate(msg); err != nil {
			log.Warn("request validation failed",
				slog.String("method", info.FullMethod),
				slog.Any("error", err),
			)

			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return handler(ctx, req)
	}
}
