package grpc

import (
	"context"
	"fmt"
	"net"

	searchv1 "github.com/vibhordubey333/github-service/api/proto/v1"
	"github.com/vibhordubey333/github-service/internal/middleware"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

/*
Encapsulating grpc lifecycle i.e Listen, Serve & shutdiwn and to intercept
*/
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	logger     *zap.Logger
}

// NewServer constructs a gRPC server with the full interceptor chain.
// Interceptor ordering matters:
// 1. Recovery (outermost) — catches panics from all other interceptors
// 2. Logging — logs every request with latency
// 3. Metrics — records Prometheus counters/histograms
func NewServer(port string, handler *SearchHandler, logger *zap.Logger) (*Server, error) {
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			middleware.RecoveryInterceptor(logger),
			middleware.LoggingInterceptor(logger),
			middleware.MetricsInterceptor(),
		),
	)

	// Register the search service
	searchv1.RegisterGithubSearchServiceServer(grpcServer, handler)

	// Health check: Kubernetes liveness/readiness probes use this
	healthServer := health.NewServer()
	grpc_health_v1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", grpc_health_v1.HealthCheckResponse_SERVING)

	/* Allows grpcurl and other tools to discover the API
	without needing the proto files. Disable in production if you
	don't want to expose the API surface to external tooling.
	*/
	//reflection.Register(grpcServer)

	return &Server{
		grpcServer: grpcServer,
		listener:   lis,
		logger:     logger,
	}, nil
}

func (s *Server) Start() error {
	s.logger.Info("gRPC server starting", zap.String("addr", s.listener.Addr().String()))
	return s.grpcServer.Serve(s.listener)
}

func (s *Server) Shutdown(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Info("gRPC server shutdown gracefully")
	case <-ctx.Done():
		s.logger.Warn("graceful shutdown timed out, forcing stop")
		s.grpcServer.Stop()
	}
}
