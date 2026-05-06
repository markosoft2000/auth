package authgrpc

import (
	"context"
	"errors"

	"github.com/markosoft2000/auth/internal/service/auth"
	authv1 "github.com/markosoft2000/auth/pkg/gen/grpc/auth/sso"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *serverAPI) AddApp(ctx context.Context, req *authv1.AddAppRequest) (*authv1.AddAppResponse, error) {
	id, err := s.auth.AddApp(ctx, req.GetName(), req.GetSecret())
	if err != nil {
		if errors.Is(err, auth.ErrAppExists) {
			return nil, status.Error(codes.AlreadyExists, "app already exists")
		}

		return nil, status.Error(codes.Internal, "failed to add app")
	}

	return &authv1.AddAppResponse{Id: id.String()}, nil
}

func (s *serverAPI) RemoveApp(ctx context.Context, req *authv1.RemoveAppRequest) (*authv1.RemoveAppResponse, error) {
	appID, err := convertStringToUUIDv7(req.GetId())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid app ID")
	}

	err = s.auth.RemoveApp(ctx, appID)
	if err != nil {
		if errors.Is(err, auth.ErrAppNotFound) {
			return nil, status.Error(codes.NotFound, "app not found")
		}

		return nil, status.Error(codes.Internal, "failed to remove app")
	}

	return &authv1.RemoveAppResponse{}, nil
}
