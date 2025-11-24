package grpc

import (
	"context"
	"errors"

	userpb "github.com/wersvet/user-service/proto/user"
)

// UserClient wraps the user-service gRPC client.
type UserClient struct {
	client userpb.UserInternalClient
}

// NewUserClient constructs the wrapper.
func NewUserClient(client userpb.UserInternalClient) *UserClient {
	return &UserClient{client: client}
}

// AreFriends verifies friendship between two users.
func (u *UserClient) AreFriends(ctx context.Context, userID, friendID int) (bool, error) {
	resp, err := u.client.AreFriends(ctx, &userpb.AreFriendsRequest{UserId: int64(userID), FriendId: int64(friendID)})
	if err != nil {
		return false, err
	}
	return resp.GetAreFriends(), nil
}

// GetUser retrieves user details.
func (u *UserClient) GetUser(ctx context.Context, userID int) (*userpb.GetUserResponse, error) {
	resp, err := u.client.GetUser(ctx, &userpb.GetUserRequest{UserId: int64(userID)})
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.GetId() == 0 {
		return nil, errors.New("user not found")
	}
	return resp, nil
}

// BulkUsers fetches multiple users in one call.
func (u *UserClient) BulkUsers(ctx context.Context, ids []int) ([]*userpb.GetUserResponse, error) {
	if len(ids) == 0 {
		return []*userpb.GetUserResponse{}, nil
	}
	id64s := make([]int64, 0, len(ids))
	for _, id := range ids {
		id64s = append(id64s, int64(id))
	}

	resp, err := u.client.BulkUsers(ctx, &userpb.BulkUsersRequest{Ids: id64s})
	if err != nil {
		return nil, err
	}
	return resp.GetUsers(), nil
}
