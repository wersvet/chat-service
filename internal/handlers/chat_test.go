package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"chat-service/internal/mocks"
	"chat-service/internal/models"
	"chat-service/internal/ws"
	userpb "chat-service/pb/user"
)

func setupChatRouter(handler *ChatHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", 1)
		c.Next()
	})
	r.GET("/chats", handler.ListChats)
	r.POST("/chats/start", handler.StartChat)
	r.GET("/chats/:chat_id/messages", handler.GetChatMessages)
	r.POST("/chats/:chat_id/messages", handler.PostChatMessage)
	return r
}

func TestListChatsSuccess(t *testing.T) {
	chatRepo := new(mocks.ChatRepositoryMock)
	groupRepo := new(mocks.GroupRepositoryMock)
	userClient := new(mocks.UserClientMock)
	handler := NewChatHandler(chatRepo, nil, userClient, groupRepo, nil)
	router := setupChatRouter(handler)

	chatRepo.On("ListChats", mock.Anything, 1).Return([]models.ChatSummary{{ChatID: 3, FriendID: 2}}, nil).Once()
	groupRepo.On("ListGroupsForUser", mock.Anything, 1).Return([]models.Group{{ID: 7, Name: "g"}}, nil).Once()
	userClient.On("BulkUsers", mock.Anything, []int{2}).Return([]*userpb.GetUserResponse{{Id: 2, Username: "bob"}}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	var resp map[string]any
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))

	chatRepo.AssertExpectations(t)
	groupRepo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestListChatsRepoError(t *testing.T) {
	chatRepo := new(mocks.ChatRepositoryMock)
	handler := NewChatHandler(chatRepo, nil, new(mocks.UserClientMock), new(mocks.GroupRepositoryMock), nil)
	router := setupChatRouter(handler)

	chatRepo.On("ListChats", mock.Anything, 1).Return(([]models.ChatSummary)(nil), assert.AnError).Once()

	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusInternalServerError, rec.Code)
	chatRepo.AssertExpectations(t)
}

func TestStartChatSuccess(t *testing.T) {
	chatRepo := new(mocks.ChatRepositoryMock)
	userClient := new(mocks.UserClientMock)
	handler := NewChatHandler(chatRepo, nil, userClient, new(mocks.GroupRepositoryMock), nil)
	router := setupChatRouter(handler)

	body := bytes.NewBufferString(`{"friend_id":2}`)

	userClient.On("AreFriends", mock.Anything, 1, 2).Return(true, nil).Once()
	chatRepo.On("CreateOrGetChat", mock.Anything, 1, 2).Return(models.Chat{ID: 10}, nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/chats/start", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	userClient.AssertExpectations(t)
	chatRepo.AssertExpectations(t)
}

func TestStartChatFriendCheckError(t *testing.T) {
	userClient := new(mocks.UserClientMock)
	handler := NewChatHandler(new(mocks.ChatRepositoryMock), nil, userClient, new(mocks.GroupRepositoryMock), nil)
	router := setupChatRouter(handler)

	userClient.On("AreFriends", mock.Anything, 1, 5).Return(false, assert.AnError).Once()

	req := httptest.NewRequest(http.MethodPost, "/chats/start", bytes.NewBufferString(`{"friend_id":5}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadGateway, rec.Code)
	userClient.AssertExpectations(t)
}

func TestGetChatMessagesSuccess(t *testing.T) {
	chatRepo := new(mocks.ChatRepositoryMock)
	messageRepo := new(mocks.MessageRepositoryMock)
	userClient := new(mocks.UserClientMock)
	handler := NewChatHandler(chatRepo, messageRepo, userClient, nil, nil)
	router := setupChatRouter(handler)

	messageRepo.On("GetChatMessagesForUser", mock.Anything, 5, 1).Return([]models.Message{{ID: 1, ChatID: 5, SenderID: 1}}, nil).Once()
	chatRepo.On("IsParticipant", mock.Anything, 5, 1).Return(true, nil).Once()
	userClient.On("BulkUsers", mock.Anything, []int{1}).Return([]*userpb.GetUserResponse{{Id: 1, Username: "me"}}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/chats/5/messages", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	chatRepo.AssertExpectations(t)
	messageRepo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestGetChatMessagesInvalidID(t *testing.T) {
	handler := NewChatHandler(new(mocks.ChatRepositoryMock), new(mocks.MessageRepositoryMock), new(mocks.UserClientMock), nil, nil)
	router := setupChatRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/chats/abc/messages", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostChatMessageSuccess(t *testing.T) {
	chatRepo := new(mocks.ChatRepositoryMock)
	messageRepo := new(mocks.MessageRepositoryMock)
	hub := ws.NewHub()
	handler := NewChatHandler(chatRepo, messageRepo, nil, nil, hub)
	router := setupChatRouter(handler)

	chatRepo.On("GetChat", mock.Anything, 5).Return(models.Chat{ID: 5, User1ID: 1, User2ID: 2}, nil).Once()
	messageRepo.On("CreateChatMessage", mock.Anything, 5, 1, "hi").Return(models.Message{ID: 7, ChatID: 5, SenderID: 1, Content: "hi"}, nil).Once()
	chatRepo.On("UnhideChatForUser", mock.Anything, 5, 1).Return(nil).Once()
	chatRepo.On("UnhideChatForUser", mock.Anything, 5, 2).Return(nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/chats/5/messages", bytes.NewBufferString(`{"content":"hi"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	chatRepo.AssertExpectations(t)
	messageRepo.AssertExpectations(t)
	assert.NotNil(t, hub)
}

func TestPostChatMessageInvalidID(t *testing.T) {
	handler := NewChatHandler(new(mocks.ChatRepositoryMock), new(mocks.MessageRepositoryMock), nil, nil, ws.NewHub())
	router := setupChatRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/chats/bad/messages", bytes.NewBufferString(`{"content":"hi"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
