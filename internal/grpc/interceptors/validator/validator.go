package validator

import (
	"context"
	"fmt"
	"log/slog"

	"buf.build/go/protovalidate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type ValidationInterceptor struct {
	v   protovalidate.Validator
	log *slog.Logger
}

// NewInterceptor compiles and caches validation rules ONCE at server boot
func NewInterceptor(l *slog.Logger) (*ValidationInterceptor, error) {
	const op = "grpc_auth.validator.interceptor"

	v, err := protovalidate.New()
	if err != nil {
		return nil, fmt.Errorf("%s: failed to initialize validator: %w", op, err)
	}

	return &ValidationInterceptor{
		v:   v,
		log: l.With(slog.String("op", op)),
	}, nil
}

// UnaryServerInterceptor runs stateless, fast CEL validations on hot path request streams
func (in *ValidationInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		msg, ok := req.(proto.Message)
		if !ok {
			return handler(ctx, req)
		}

		if err := in.v.Validate(msg); err != nil {
			in.log.Warn("request validation failed",
				slog.String("method", info.FullMethod),
				slog.Any("error", err),
			)

			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		return handler(ctx, req)
	}
}
