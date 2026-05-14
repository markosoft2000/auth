package interceptors

import (
	"context"

	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ServerLimiter struct {
	// Heavy bucket enforces strict boundaries on CPU-bound cryptographic endpoints
	heavyLimiter *rate.Limiter
	// Light bucket provides high-volume throughput allowances for low-cost logic paths
	lightLimiter *rate.Limiter
}

// NewSplitLimiter creates a dual-bucket manager mapping limits to task intensity
func NewSplitLimiter(heavyRate, heavyBurst, lightRate, lightBurst int) *ServerLimiter {
	return &ServerLimiter{
		heavyLimiter: rate.NewLimiter(rate.Limit(heavyRate), heavyBurst),
		lightLimiter: rate.NewLimiter(rate.Limit(lightRate), lightBurst),
	}
}

// UnaryServerInterceptor returns a compiled gRPC middleware hook targeting specific service paths
func (l *ServerLimiter) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// Extract target method path (Format: "/package.Service/Method")
		switch info.FullMethod {
		case "/auth.Auth/Login", "/auth.Auth/Register":
			// HEAVY GATE: for long CPU-bound requests
			if !l.heavyLimiter.Allow() {
				return nil, status.Error(codes.ResourceExhausted, "cryptographic hashing capacity saturated; backoff applied")
			}

		default:
			// LIGHT GATE: Pass non-intensive calls (like Logout, Refresh-token) through relaxed boundaries
			if !l.lightLimiter.Allow() {
				return nil, status.Error(codes.ResourceExhausted, "global traffic limit reached")
			}
		}

		// Continue processing down the gRPC middleware chain smoothly
		return handler(ctx, req)
	}
}
