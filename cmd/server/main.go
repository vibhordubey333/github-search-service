package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/vibhordubey333/github-service/internal/config"
	ghclient "github.com/vibhordubey333/github-service/internal/github"
	"github.com/vibhordubey333/github-service/internal/service"
	grpctransport "github.com/vibhordubey333/github-service/internal/transport/grpc"
)

func main() {
	if err := godotenv.Load(); err != nil {
		// .env is optional; real env vars may be set in production
	}

	logger, _ := zap.NewProduction()
	defer logger.Sync()

	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}

	//Dependency construction
	githubClient := ghclient.NewClient(
		cfg.GithubBaseURL,
		cfg.GithubToken,
		cfg.RequestTimeout,
		logger,
	)

	searchSvc := service.NewSearchService(githubClient, logger, cfg.MaxConcurrency)
	handler := grpctransport.NewSearchHandler(searchSvc, logger)

	grpcServer, err := grpctransport.NewServer(cfg.Port, handler, logger)
	if err != nil {
		logger.Fatal("failed to create gRPC server", zap.Error(err))
	}

	//Exposing prometheus metrics
	metricsServer := &http.Server{
		Addr:    ":9090",
		Handler: promhttp.Handler(),
	}

	go func() {
		if err := metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("metrics server error", zap.Error(err))
		}
	}()

	go func() {
		if err := grpcServer.Start(); err != nil {
			logger.Fatal("gRPC server failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	sig := <-quit
	logger.Info("received shutdown signal", zap.String("signal", sig.String()))

	// Graceful shutdown with deadline. gRPC requests timeout at 10s; 15s for complete cleanup.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	grpcServer.Shutdown(shutdownCtx)

	if err := metricsServer.Shutdown(shutdownCtx); err != nil {
		logger.Warn("metrics server shutdown error", zap.Error(err))
	}

	logger.Info("server stopped cleanly")
}
