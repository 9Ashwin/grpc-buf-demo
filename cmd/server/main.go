package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	userv1 "github.com/9Ashwin/grpc-buf-demo/gen/go/demo/user/v1"
	"github.com/9Ashwin/grpc-buf-demo/user"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

const (
	grpcAddress  = ":8080"
	grpcEndpoint = "localhost:8080"
	httpAddress  = ":8081"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, slog.Default()); err != nil {
		slog.Error("server stopped", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger) error {
	listener, err := net.Listen("tcp", grpcAddress)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", grpcAddress, err)
	}
	grpcServer := grpc.NewServer()
	userv1.RegisterUserServiceServer(grpcServer, user.NewService())
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
