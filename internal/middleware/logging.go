package middleware

import (
	"context"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func LoggingInterceptor(logger *zap.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()
		resp, err := handler(ctx, req)

		code := status.Code(err)
		logger.Info("rpc",
			zap.String("method", info.FullMethod),
			zap.String("code", code.String()),
			zap.Duration("latency", time.Since(start)),
			zap.Error(err),
		)

		return resp, err
	}
}
