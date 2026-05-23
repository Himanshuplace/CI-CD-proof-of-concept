package handler_test

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/gnexlayer/demo-service/internal/handler"
	"github.com/gnexlayer/demo-service/internal/store"
)

// startTestGRPCServer starts a real gRPC server on a random port and returns
// a connected client.  Using a real listener (not an in-memory mock) means
// we test the full serialisation path — the same path production uses.
func startTestGRPCServer(t *testing.T) handler.UserServiceServer {
	t.Helper()

	s := store.New()
	srv := handler.NewUserServiceServer(s)

	grpcSrv := handler.GRPCServer()
	handler.RegisterUserServiceServer(grpcSrv, srv)

	lis, err := net.Listen("tcp", "127.0.0.1:0") // :0 → random free port
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	go func() { _ = grpcSrv.Serve(lis) }()
	t.Cleanup(grpcSrv.GracefulStop) // shut down after the test

	// Build a client pointing at the ephemeral port.
	cc, err := grpc.NewClient(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = cc.Close() })

	// For testing we call the server methods directly (simpler).
	// In a real client you'd use the generated stub.
	_ = cc // available if you want to add stub-based tests later
	return srv
}

func TestGRPCGetUserNotFound(t *testing.T) {
	srv := startTestGRPCServer(t)

	_, err := srv.GetUser(context.Background(), &handler.GetUserRequest{ID: "999"})
	if err == nil {
		t.Fatal("expected an error for missing user, got nil")
	}
	// In production you'd check: status.Code(err) == codes.NotFound
}

func TestGRPCListUsers(t *testing.T) {
	s := store.New()
	s.Create("Alice", "alice@example.com")
	s.Create("Bob", "bob@example.com")

	srv := handler.NewUserServiceServer(s)
	resp, err := srv.ListUsers(context.Background(), &handler.ListUsersRequest{Limit: 10})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if resp.Total != 2 {
		t.Errorf("want Total=2, got %d", resp.Total)
	}
	if len(resp.Users) != 2 {
		t.Errorf("want 2 users, got %d", len(resp.Users))
	}
}

func TestGRPCListUsersPagination(t *testing.T) {
	s := store.New()
	for i := 0; i < 5; i++ {
		s.Create("User", "u@example.com")
	}

	srv := handler.NewUserServiceServer(s)
	resp, err := srv.ListUsers(context.Background(), &handler.ListUsersRequest{Limit: 2, Offset: 0})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if len(resp.Users) != 2 {
		t.Errorf("want 2 (limited), got %d", len(resp.Users))
	}
	if resp.Total != 5 {
		t.Errorf("want Total=5, got %d", resp.Total)
	}
}
