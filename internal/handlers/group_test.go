package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"chat-service/internal/mocks"
	"chat-service/internal/models"
	"chat-service/internal/ws"
	userpb "chat-service/pb/user"
)

func setupGroupRouter(handler *GroupHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", 1)
		c.Next()
	})
	r.POST("/groups", handler.CreateGroup)
	r.GET("/groups/:group_id/messages", handler.GetGroupMessages)
	r.POST("/groups/:group_id/messages", handler.PostGroupMessage)
	return r
}

func TestCreateGroupSuccess(t *testing.T) {
	groupRepo := new(mocks.GroupRepositoryMock)
	messageRepo := new(mocks.GroupMessageRepositoryMock)
	userClient := new(mocks.UserClientMock)
	handler := NewGroupHandler(groupRepo, messageRepo, userClient, nil, nil)
	router := setupGroupRouter(handler)

	body := bytes.NewBufferString(`{"name":"test","member_ids":[2]}`)

	userClient.On("BulkUsers", mock.Anything, []int{2}).Return([]*userpb.GetUserResponse{{Id: 2, Username: "bob"}}, nil).Once()
	groupRepo.On("CreateGroup", mock.Anything, 1, "test", []int{2}).Return(models.Group{ID: 5, Name: "test"}, nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/groups", body)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	groupRepo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestCreateGroupInvalidBody(t *testing.T) {
	handler := NewGroupHandler(new(mocks.GroupRepositoryMock), new(mocks.GroupMessageRepositoryMock), new(mocks.UserClientMock), nil, nil)
	router := setupGroupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/groups", bytes.NewBufferString(`{"name":5}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetGroupMessagesSuccess(t *testing.T) {
	groupRepo := new(mocks.GroupRepositoryMock)
	messageRepo := new(mocks.GroupMessageRepositoryMock)
	userClient := new(mocks.UserClientMock)
	handler := NewGroupHandler(groupRepo, messageRepo, userClient, nil, nil)
	router := setupGroupRouter(handler)

	groupRepo.On("IsMember", mock.Anything, 9, 1).Return(true, nil).Once()
	messageRepo.On("ListGroupMessages", mock.Anything, 9).Return([]models.GroupMessage{{ID: 1, GroupID: 9, SenderID: 1}}, nil).Once()
	userClient.On("BulkUsers", mock.Anything, []int{1}).Return([]*userpb.GetUserResponse{{Id: 1, Username: "me"}}, nil).Once()

	req := httptest.NewRequest(http.MethodGet, "/groups/9/messages", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	groupRepo.AssertExpectations(t)
	messageRepo.AssertExpectations(t)
	userClient.AssertExpectations(t)
}

func TestGetGroupMessagesInvalidID(t *testing.T) {
	handler := NewGroupHandler(new(mocks.GroupRepositoryMock), new(mocks.GroupMessageRepositoryMock), new(mocks.UserClientMock), nil, nil)
	router := setupGroupRouter(handler)

	req := httptest.NewRequest(http.MethodGet, "/groups/bad/messages", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestPostGroupMessageSuccess(t *testing.T) {
	groupRepo := new(mocks.GroupRepositoryMock)
	messageRepo := new(mocks.GroupMessageRepositoryMock)
	hub := ws.NewHub()
	handler := NewGroupHandler(groupRepo, messageRepo, nil, hub, nil)
	router := setupGroupRouter(handler)

	groupRepo.On("IsMember", mock.Anything, 9, 1).Return(true, nil).Once()
	messageRepo.On("CreateGroupMessage", mock.Anything, 9, 1, "hey").Return(models.GroupMessage{ID: 3, GroupID: 9, SenderID: 1, Content: "hey"}, nil).Once()

	req := httptest.NewRequest(http.MethodPost, "/groups/9/messages", bytes.NewBufferString(`{"content":"hey"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code)
	groupRepo.AssertExpectations(t)
	messageRepo.AssertExpectations(t)
}

func TestPostGroupMessageInvalidID(t *testing.T) {
	handler := NewGroupHandler(new(mocks.GroupRepositoryMock), new(mocks.GroupMessageRepositoryMock), nil, ws.NewHub(), nil)
	router := setupGroupRouter(handler)

	req := httptest.NewRequest(http.MethodPost, "/groups/abc/messages", bytes.NewBufferString(`{"content":"hey"}`))
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
