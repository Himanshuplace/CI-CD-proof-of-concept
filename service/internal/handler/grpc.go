package handler

// This file implements the UserService gRPC server WITHOUT running protoc.
// We hand-write the types and service descriptor that protoc would generate.
//
// Why show this?
//   - You understand what code generation actually produces.
//   - You can add a Makefile `make proto` later to generate from user.proto.
//   - The hand-written version is functionally identical.
//
// In production, run:
//   protoc --go_out=. --go-grpc_out=. proto/user.proto
// and delete this file — the generated files replace it.

import (
	"context"
	"encoding/json"
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gnexlayer/demo-service/internal/store"
)

// ── Message types (what protoc-gen-go generates from user.proto) ──────────────

type GetUserRequest struct {
	ID string `json:"id"`
}

type ListUsersRequest struct {
	Limit  int32 `json:"limit"`
	Offset int32 `json:"offset"`
}

type UserResponse struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type ListUsersResponse struct {
	Users []UserResponse `json:"users"`
	Total int32          `json:"total"`
}

// ── Server interface (what protoc-gen-go-grpc generates) ─────────────────────

// UserServiceServer is the interface that every gRPC server must implement.
// protoc-gen-go-grpc generates this automatically.
type UserServiceServer interface {
	GetUser(context.Context, *GetUserRequest) (*UserResponse, error)
	ListUsers(context.Context, *ListUsersRequest) (*ListUsersResponse, error)
}

// ── Implementation ────────────────────────────────────────────────────────────

type userServiceServer struct {
	store *store.Store
}

// NewUserServiceServer returns a ready-to-register gRPC server.
func NewUserServiceServer(s *store.Store) UserServiceServer {
	return &userServiceServer{store: s}
}

func (srv *userServiceServer) GetUser(_ context.Context, req *GetUserRequest) (*UserResponse, error) {
	u, err := srv.store.Get(req.ID)
	if errors.Is(err, store.ErrNotFound) {
		// Returning a gRPC status code is the equivalent of an HTTP 404.
		// Clients inspect this code — don't just return a regular error.
		return nil, status.Errorf(codes.NotFound, "user %q not found", req.ID)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "store error: %v", err)
	}
	return &UserResponse{ID: u.ID, Name: u.Name, Email: u.Email}, nil
}

func (srv *userServiceServer) ListUsers(_ context.Context, req *ListUsersRequest) (*ListUsersResponse, error) {
	all := srv.store.List()

	// Apply offset and limit (simple pagination).
	start := int(req.Offset)
	if start > len(all) {
		start = len(all)
	}
	end := len(all)
	if req.Limit > 0 && start+int(req.Limit) < end {
		end = start + int(req.Limit)
	}
	page := all[start:end]

	resp := &ListUsersResponse{Total: int32(len(all))}
	for _, u := range page {
		resp.Users = append(resp.Users, UserResponse{ID: u.ID, Name: u.Name, Email: u.Email})
	}
	return resp, nil
}

// ── Service registration ──────────────────────────────────────────────────────
// What follows is the service descriptor that protoc generates.
// It tells grpc.Server: "I implement service user.v1.UserService with these methods."

// RegisterUserServiceServer registers the server on a grpc.Server.
// This is exactly what protoc-generated code calls.
func RegisterUserServiceServer(s *grpc.Server, srv UserServiceServer) {
	s.RegisterService(&userServiceDesc, srv)
}

// jsonCodec makes our gRPC server speak JSON instead of protobuf binary.
// This lets you test with curl / grpcurl --plaintext without protobuf schemas.
// In production, replace with protobuf encoding (the default).
type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error)        { return json.Marshal(v) }
func (jsonCodec) Unmarshal(data []byte, v any) error   { return json.Unmarshal(data, v) }
func (jsonCodec) Name() string                          { return "proto" } // grpc requires this name

// GRPCServer returns a configured grpc.Server ready to accept connections.
// Call RegisterUserServiceServer(srv, ...) after this, then srv.Serve(listener).
func GRPCServer() *grpc.Server {
	return grpc.NewServer(
		grpc.ForceCodec(jsonCodec{}),
	)
}

var userServiceDesc = grpc.ServiceDesc{
	ServiceName: "user.v1.UserService",
	HandlerType: (*UserServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetUser",
			Handler:    grpcGetUserHandler,
		},
		{
			MethodName: "ListUsers",
			Handler:    grpcListUsersHandler,
		},
	},
	Streams: []grpc.StreamDesc{},
}

func grpcGetUserHandler(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var req GetUserRequest
	if err := dec(&req); err != nil {
		return nil, err
	}
	return srv.(UserServiceServer).GetUser(ctx, &req)
}

func grpcListUsersHandler(srv any, ctx context.Context, dec func(any) error, _ grpc.UnaryServerInterceptor) (any, error) {
	var req ListUsersRequest
	if err := dec(&req); err != nil {
		return nil, err
	}
	return srv.(UserServiceServer).ListUsers(ctx, &req)
}
