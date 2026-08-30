package api

import (
	"context"
	"errors"

	userpb "github.com/khanhdt2001/golang-over-engineering-/proto/user/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"over-engineering/user/service"
)

type grpcHandler struct {
	userpb.UnimplementedUserServiceServer
	service *service.Service
}

func NewGRPC(service *service.Service) userpb.UserServiceServer {
	return grpcHandler{service: service}
}

func (h grpcHandler) SignUp(ctx context.Context, request *userpb.SignUpRequest) (*userpb.SignUpResponse, error) {
	user, err := h.service.SignUp(ctx, service.SignUpInput{
		Email: request.GetEmail(), Password: request.GetPassword(), Username: request.GetUsername(),
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return &userpb.SignUpResponse{User: protobufUser(user)}, nil
}

func (h grpcHandler) SignIn(ctx context.Context, request *userpb.SignInRequest) (*userpb.SignInResponse, error) {
	user, token, err := h.service.SignIn(ctx, request.GetEmail(), request.GetPassword())
	if err != nil {
		return nil, grpcError(err)
	}
	return &userpb.SignInResponse{Token: token, User: protobufUser(user)}, nil
}

func (h grpcHandler) UpdateProfile(ctx context.Context, request *userpb.UpdateProfileRequest) (*userpb.UpdateProfileResponse, error) {
	user, err := h.service.UpdateProfile(ctx, tokenFromMetadata(ctx), service.UpdateInput{
		Email: request.Email, Password: request.Password, Username: request.Username,
	})
	if err != nil {
		return nil, grpcError(err)
	}
	return &userpb.UpdateProfileResponse{User: protobufUser(user)}, nil
}

func protobufUser(user service.User) *userpb.User {
	return &userpb.User{Id: user.ID, Email: user.Email, Username: user.Username}
}

func tokenFromMetadata(ctx context.Context) string {
	values := metadata.ValueFromIncomingContext(ctx, "authorization")
	if len(values) == 0 {
		return ""
	}
	return bearerToken(values[0])
}

func grpcError(err error) error {
	switch {
	case errors.Is(err, service.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, service.ErrEmailExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, service.ErrInvalidCredentials), errors.Is(err, service.ErrUnauthorized):
		return status.Error(codes.Unauthenticated, err.Error())
	default:
		return status.Error(codes.Internal, "internal server error")
	}
}
