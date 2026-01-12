package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"

	"chat-service/internal/models"
	"chat-service/internal/repositories"
	userpb "chat-service/pb/user"
)

type ChatRepositoryMock struct {
	mock.Mock
}

func (m *ChatRepositoryMock) CreateOrGetChat(ctx context.Context, userID int, friendID int) (models.Chat, error) {
	args := m.Called(ctx, userID, friendID)
	var chat models.Chat
	if val := args.Get(0); val != nil {
		chat = val.(models.Chat)
	}
	return chat, args.Error(1)
}

func (m *ChatRepositoryMock) IsParticipant(ctx context.Context, chatID int, userID int) (bool, error) {
	args := m.Called(ctx, chatID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *ChatRepositoryMock) GetChat(ctx context.Context, chatID int) (models.Chat, error) {
	args := m.Called(ctx, chatID)
	var chat models.Chat
	if val := args.Get(0); val != nil {
		chat = val.(models.Chat)
	}
	return chat, args.Error(1)
}

func (m *ChatRepositoryMock) ListChats(ctx context.Context, userID int) ([]models.ChatSummary, error) {
	args := m.Called(ctx, userID)
	var list []models.ChatSummary
	if val := args.Get(0); val != nil {
		list = val.([]models.ChatSummary)
	}
	return list, args.Error(1)
}

func (m *ChatRepositoryMock) HideChatForUser(ctx context.Context, chatID int, userID int) error {
	args := m.Called(ctx, chatID, userID)
	return args.Error(0)
}

func (m *ChatRepositoryMock) UnhideChatForUser(ctx context.Context, chatID int, userID int) error {
	args := m.Called(ctx, chatID, userID)
	return args.Error(0)
}

type MessageRepositoryMock struct {
	mock.Mock
}

func (m *MessageRepositoryMock) CreateChatMessage(ctx context.Context, chatID int, senderID int, content string) (models.Message, error) {
	args := m.Called(ctx, chatID, senderID, content)
	var msg models.Message
	if val := args.Get(0); val != nil {
		msg = val.(models.Message)
	}
	return msg, args.Error(1)
}

func (m *MessageRepositoryMock) GetChatMessagesForUser(ctx context.Context, chatID int, userID int) ([]models.Message, error) {
	args := m.Called(ctx, chatID, userID)
	var msgs []models.Message
	if val := args.Get(0); val != nil {
		msgs = val.([]models.Message)
	}
	return msgs, args.Error(1)
}

func (m *MessageRepositoryMock) GetMessage(ctx context.Context, messageID int) (models.Message, error) {
	args := m.Called(ctx, messageID)
	var msg models.Message
	if val := args.Get(0); val != nil {
		msg = val.(models.Message)
	}
	return msg, args.Error(1)
}

func (m *MessageRepositoryMock) SoftDeleteMessageForUser(ctx context.Context, messageID int, isSender bool) error {
	args := m.Called(ctx, messageID, isSender)
	return args.Error(0)
}

func (m *MessageRepositoryMock) DeleteMessageForAll(ctx context.Context, messageID int, userID int) error {
	args := m.Called(ctx, messageID, userID)
	return args.Error(0)
}

type GroupRepositoryMock struct {
	mock.Mock
}

func (m *GroupRepositoryMock) CreateGroup(ctx context.Context, ownerID int, name string, memberIDs []int) (models.Group, error) {
	args := m.Called(ctx, ownerID, name, memberIDs)
	var group models.Group
	if val := args.Get(0); val != nil {
		group = val.(models.Group)
	}
	return group, args.Error(1)
}

func (m *GroupRepositoryMock) ListGroupsForUser(ctx context.Context, userID int) ([]models.Group, error) {
	args := m.Called(ctx, userID)
	var groups []models.Group
	if val := args.Get(0); val != nil {
		groups = val.([]models.Group)
	}
	return groups, args.Error(1)
}

func (m *GroupRepositoryMock) IsMember(ctx context.Context, groupID int, userID int) (bool, error) {
	args := m.Called(ctx, groupID, userID)
	return args.Bool(0), args.Error(1)
}

func (m *GroupRepositoryMock) GetGroup(ctx context.Context, groupID int) (models.Group, error) {
	args := m.Called(ctx, groupID)
	var group models.Group
	if val := args.Get(0); val != nil {
		group = val.(models.Group)
	}
	return group, args.Error(1)
}

type GroupMessageRepositoryMock struct {
	mock.Mock
}

func (m *GroupMessageRepositoryMock) CreateGroupMessage(ctx context.Context, groupID int, senderID int, content string) (models.GroupMessage, error) {
	args := m.Called(ctx, groupID, senderID, content)
	var msg models.GroupMessage
	if val := args.Get(0); val != nil {
		msg = val.(models.GroupMessage)
	}
	return msg, args.Error(1)
}

func (m *GroupMessageRepositoryMock) ListGroupMessages(ctx context.Context, groupID int) ([]models.GroupMessage, error) {
	args := m.Called(ctx, groupID)
	var msgs []models.GroupMessage
	if val := args.Get(0); val != nil {
		msgs = val.([]models.GroupMessage)
	}
	return msgs, args.Error(1)
}

func (m *GroupMessageRepositoryMock) GetGroupMessage(ctx context.Context, messageID int) (models.GroupMessage, error) {
	args := m.Called(ctx, messageID)
	var msg models.GroupMessage
	if val := args.Get(0); val != nil {
		msg = val.(models.GroupMessage)
	}
	return msg, args.Error(1)
}

func (m *GroupMessageRepositoryMock) DeleteForAll(ctx context.Context, messageID int, senderID int) error {
	args := m.Called(ctx, messageID, senderID)
	return args.Error(0)
}

type UserClientMock struct {
	mock.Mock
}

func (m *UserClientMock) AreFriends(ctx context.Context, userID, friendID int) (bool, error) {
	args := m.Called(ctx, userID, friendID)
	return args.Bool(0), args.Error(1)
}

func (m *UserClientMock) BulkUsers(ctx context.Context, ids []int) ([]*userpb.GetUserResponse, error) {
	args := m.Called(ctx, ids)
	var users []*userpb.GetUserResponse
	if val := args.Get(0); val != nil {
		users = val.([]*userpb.GetUserResponse)
	}
	return users, args.Error(1)
}

var _ repositories.ChatRepository = (*ChatRepositoryMock)(nil)
var _ repositories.MessageRepository = (*MessageRepositoryMock)(nil)
var _ repositories.GroupRepository = (*GroupRepositoryMock)(nil)
var _ repositories.GroupMessageRepository = (*GroupMessageRepositoryMock)(nil)
var _ interface {
	AreFriends(context.Context, int, int) (bool, error)
	BulkUsers(context.Context, []int) ([]*userpb.GetUserResponse, error)
} = (*UserClientMock)(nil)
