package api

import (
	"context"
	"testing"
	"time"

	userpb "github.com/khanhdt2001/golang-over-engineering-/proto/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"over-engineering/user/service"
)

type grpcRepository struct{}

func (grpcRepository) CreateUser(_ context.Context, email, _, username string) (service.User, error) {
	return service.User{ID: "user-1", Email: email, Username: username}, nil
}
func (grpcRepository) FindByEmail(context.Context, string) (service.User, string, error) {
	return service.User{}, "", nil
}
func (grpcRepository) UpdateUser(context.Context, string, *string, *string, *string) (service.User, error) {
	return service.User{}, nil
}
func (grpcRepository) CreateSession(context.Context, string, string, time.Time) error { return nil }
func (grpcRepository) FindSessionUser(context.Context, string) (string, error)        { return "", nil }

func TestGRPCSignUp(t *testing.T) {
	server := NewGRPC(service.New(grpcRepository{}))
	response, err := server.SignUp(context.Background(), &userpb.SignUpRequest{Email: "ADA@EXAMPLE.COM", Password: "password", Username: "Ada"})
	if err != nil || response.GetUser().GetEmail() != "ada@example.com" {
		t.Fatalf("unexpected sign-up response: %#v, %v", response, err)
	}

	_, err = server.SignUp(context.Background(), &userpb.SignUpRequest{Email: "ada@example.com", Password: "short", Username: "Ada"})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("invalid request code = %v, want %v", status.Code(err), codes.InvalidArgument)
	}
}
