// main.go — the binary entry point.
//
// Architecture decision: One binary, two ports.
//   :8080 — HTTP (REST + WebSocket) served by net/http
//   :9090 — gRPC served by google.golang.org/grpc
//
// Why two ports instead of one?
//   gRPC requires HTTP/2 framing which conflicts with HTTP/1.1 on the same port
//   unless you add a multiplexing layer (cmux). Two ports is simpler and more
//   transparent in Kubernetes NetworkPolicy / KrakenD routing.
//
// Why one binary instead of two?
//   Fewer containers to manage. Same in-memory store is shared between protocols.
//   In practice, if gRPC traffic grows, you'd split them. Until then, YAGNI.

package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"

	"github.com/gnexlayer/demo-service/internal/handler"
	"github.com/gnexlayer/demo-service/internal/store"
)

func main() {
	// slog writes structured JSON logs — required for any log aggregation
	// system (Loki, CloudWatch, ELK). Never use fmt.Println in production code.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Shared store — all three protocol handlers read from and write to this.
	s := store.New()

	// ── HTTP + WebSocket server ──────────────────────────────────────────────
	httpMux := http.NewServeMux()
	handler.HTTP(httpMux, s)
	handler.WS(httpMux, s.Len) // inject s.Len so WS handler stays decoupled

	httpAddr := envOr("HTTP_ADDR", ":8080")
	httpSrv := &http.Server{
		Addr:         httpAddr,
		Handler:      httpMux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second, // longer for WS connections
		IdleTimeout:  120 * time.Second,
	}

	// ── gRPC server ──────────────────────────────────────────────────────────
	grpcAddr := envOr("GRPC_ADDR", ":9090")
	grpcSrv := handler.GRPCServer()
	handler.RegisterUserServiceServer(grpcSrv, handler.NewUserServiceServer(s))

	// ── Start both servers concurrently ─────────────────────────────────────
	// Each runs in its own goroutine. errCh collects fatal errors.
	errCh := make(chan error, 2)

	go func() {
		slog.Info("HTTP server listening", "addr", httpAddr)
		if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	go func() {
		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			errCh <- err
			return
		}
		slog.Info("gRPC server listening", "addr", grpcAddr)
		if err := grpcSrv.Serve(lis); err != nil {
			errCh <- err
		}
	}()

	// ── Graceful shutdown ────────────────────────────────────────────────────
	// Kubernetes sends SIGTERM before killing a pod. We catch it, wait for
	// in-flight requests to finish, then exit cleanly.
	// Without this, Kubernetes might kill the pod mid-request → 500 errors.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)

	select {
	case sig := <-quit:
		slog.Info("shutdown signal received", "signal", sig)
	case err := <-errCh:
		slog.Error("server error", "err", err)
	}

	slog.Info("shutting down — draining connections (30s timeout)")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_ = httpSrv.Shutdown(ctx) // waits for active requests to finish
	grpcSrv.GracefulStop()    // waits for active RPCs to finish

	slog.Info("shutdown complete")
}

// envOr reads an environment variable or returns a default.
// This keeps configuration 12-factor compliant: all config via env vars.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
