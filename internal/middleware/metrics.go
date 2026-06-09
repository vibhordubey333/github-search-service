package middleware

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"

	"github.com/vibhordubey333/github-service/internal/observability"
)

// MetricsInterceptor records Prometheus metrics for every RPC.
func MetricsInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		code := status.Code(err).String()
		observability.RPCRequestsTotal.WithLabelValues(info.FullMethod, code).Inc()
		observability.RPCLatency.WithLabelValues(info.FullMethod).Observe(time.Since(start).Seconds())

		return resp, err
	}
}
