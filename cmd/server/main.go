package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/9Ashwin/grpc-buf-demo/db"
	userv1 "github.com/9Ashwin/grpc-buf-demo/gen/go/demo/user/v1"
	"github.com/9Ashwin/grpc-buf-demo/user"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/health"
	healthv1 "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"
)

const (
	grpcAddress        = ":8080"
	grpcEndpoint       = "localhost:8080"
	httpAddress        = ":8081"
	defaultDatabaseURL = "grpc-demo.db"
)

func main() {
	os.Exit(realMain())
}

func realMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = defaultDatabaseURL
	}
	database, err := db.Open(ctx, databaseURL)
	if err != nil {
		slog.Error("database setup failed", "err", err)
		return 1
	}

	if err := run(ctx, slog.Default(), database); err != nil {
		slog.Error("server stopped", "err", err)
		_ = database.Close()
		return 1
	}
	if err := database.Close(); err != nil {
		slog.Error("database close failed", "err", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, logger *slog.Logger, database *sql.DB) error {
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", grpcAddress)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", grpcAddress, err)
	}
	grpcServer := grpc.NewServer()
	userv1.RegisterUserServiceServer(grpcServer, user.NewService(database))
	healthServer := health.NewServer()
	healthv1.RegisterHealthServer(grpcServer, healthServer)
	healthServer.SetServingStatus("", healthv1.HealthCheckResponse_SERVING)
	reflection.Register(grpcServer)

	gateway := runtime.NewServeMux()
	if err := userv1.RegisterUserServiceHandlerFromEndpoint(ctx, gateway, grpcEndpoint, []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}); err != nil {
		return fmt.Errorf("registering HTTP gateway: %w", err)
	}
	httpServer := &http.Server{
		Addr:              httpAddress,
		Handler:           gateway,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	errCh := make(chan error, 2)
	go func() { errCh <- grpcServer.Serve(listener) }()
	go func() { errCh <- httpServer.ListenAndServe() }()
	logger.Info("server started", "grpc", grpcAddress, "http", httpAddress)

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		grpcServer.GracefulStop()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("shutting down HTTP server: %w", err)
		}
		return nil
	}
}
