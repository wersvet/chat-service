package grpc

import (
	"context"
	"errors"

	authpb "chat-service/pb/auth"
)

// AuthClient wraps the auth-service gRPC client.
type AuthClient struct {
	client authpb.AuthServiceClient
}

// NewAuthClient constructs the wrapper.
func NewAuthClient(client authpb.AuthServiceClient) *AuthClient {
	return &AuthClient{client: client}
}

// ValidateToken verifies the JWT and returns the authenticated user id.
func (a *AuthClient) ValidateToken(ctx context.Context, token string) (int, error) {
	resp, err := a.client.ValidateToken(ctx, &authpb.ValidateTokenRequest{Token: token})
	if err != nil {
		return 0, err
	}
	if !resp.Valid || resp.UserId == 0 {
		return 0, errors.New("invalid token")
	}
	return int(resp.UserId), nil
}

// GetUser fetches user info from auth-service.
func (a *AuthClient) GetUser(ctx context.Context, userID int) (*authpb.GetUserResponse, error) {
	resp, err := a.client.GetUser(ctx, &authpb.GetUserRequest{UserId: int64(userID)})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Id == 0 {
		return nil, errors.New("user not found")
	}
	return resp, nil
}
