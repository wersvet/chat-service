package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	grpcclient "chat-service/internal/grpc"
	"chat-service/internal/models"
	"chat-service/internal/repositories"
	"chat-service/internal/ws"
)

// ChatHandler manages private chat endpoints.
type ChatHandler struct {
	chatRepo    repositories.ChatRepository
	messageRepo repositories.MessageRepository
	userClient  *grpcclient.UserClient
	hub         *ws.Hub
}

// NewChatHandler builds a ChatHandler.
func NewChatHandler(chatRepo repositories.ChatRepository, messageRepo repositories.MessageRepository, userClient *grpcclient.UserClient, hub *ws.Hub) *ChatHandler {
	return &ChatHandler{
		chatRepo:    chatRepo,
		messageRepo: messageRepo,
		userClient:  userClient,
		hub:         hub,
	}
}

// ListChats returns the chats visible to the authenticated user.
func (h *ChatHandler) ListChats(c *gin.Context) {
	userID := c.GetInt("userID")

	chats, err := h.chatRepo.ListChats(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load chats"})
		return
	}

	friendIDs := make([]int, 0, len(chats))
	for _, chat := range chats {
		friendIDs = append(friendIDs, chat.FriendID)
	}

	users, err := h.userClient.BulkUsers(c.Request.Context(), friendIDs)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to load user info"})
		return
	}

	usernameByID := map[int]string{}
	for _, u := range users {
		usernameByID[int(u.GetId())] = u.GetUsername()
	}

	type chatResponse struct {
		ChatID         int       `json:"chat_id"`
		FriendID       int       `json:"friend_id"`
		FriendUsername string    `json:"friend_username,omitempty"`
		CreatedAt      time.Time `json:"created_at"`
	}

	responses := make([]chatResponse, 0, len(chats))
	for _, chat := range chats {
		responses = append(responses, chatResponse{
			ChatID:         chat.ChatID,
			FriendID:       chat.FriendID,
			FriendUsername: usernameByID[chat.FriendID],
			CreatedAt:      chat.Created,
		})
	}

	c.JSON(http.StatusOK, gin.H{"chats": responses})
}

// StartChat creates or returns an existing private chat between users.
func (h *ChatHandler) StartChat(c *gin.Context) {
	var req struct {
		FriendID int `json:"friend_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetInt("userID")
	friends, err := h.userClient.AreFriends(c.Request.Context(), userID, req.FriendID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to validate friendship"})
		return
	}
	if !friends {
		c.JSON(http.StatusForbidden, gin.H{"error": "users are not friends"})
		return
	}

	if userID == req.FriendID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cannot chat with yourself"})
		return
	}

	chat, err := h.chatRepo.CreateOrGetChat(c.Request.Context(), userID, req.FriendID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create chat"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"chat_id": chat.ID})
}

// GetChatMessages returns messages for a chat filtered for the user.
func (h *ChatHandler) GetChatMessages(c *gin.Context) {
	chatID, err := strconv.Atoi(c.Param("chat_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat id"})
		return
	}

	userID := c.GetInt("userID")
	member, err := h.chatRepo.IsParticipant(c.Request.Context(), chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to verify membership"})
		return
	}
	if !member {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a chat member"})
		return
	}

	msgs, err := h.messageRepo.GetChatMessagesForUser(c.Request.Context(), chatID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load messages"})
		return
	}

	senderIDs := make([]int, 0, len(msgs))
	senderIDSet := map[int]struct{}{}
	for _, m := range msgs {
		if _, ok := senderIDSet[m.SenderID]; !ok {
			senderIDSet[m.SenderID] = struct{}{}
			senderIDs = append(senderIDs, m.SenderID)
		}
	}

	users, err := h.userClient.BulkUsers(c.Request.Context(), senderIDs)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to load senders"})
		return
	}
	senderNames := map[int]string{}
	for _, u := range users {
		senderNames[int(u.GetId())] = u.GetUsername()
	}

	type messageResponse struct {
		models.Message
		SenderUsername string `json:"sender_username,omitempty"`
	}

	resp := make([]messageResponse, 0, len(msgs))
	for _, m := range msgs {
		resp = append(resp, messageResponse{Message: m, SenderUsername: senderNames[m.SenderID]})
	}

	c.JSON(http.StatusOK, gin.H{"messages": resp})
}

// PostChatMessage stores a chat message and broadcasts it.
func (h *ChatHandler) PostChatMessage(c *gin.Context) {
	chatID, err := strconv.Atoi(c.Param("chat_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat id"})
		return
	}

	userID := c.GetInt("userID")
	chat, err := h.chatRepo.GetChat(c.Request.Context(), chatID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrChatNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "chat not found"})
		return
	}
	if !isChatParticipant(chat, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a chat member"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := h.messageRepo.CreateChatMessage(c.Request.Context(), chatID, userID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store message"})
		return
	}

	// Ensure chat becomes visible again for both sides once a new message is sent.
	h.chatRepo.UnhideChatForUser(c.Request.Context(), chatID, chat.User1ID)
	h.chatRepo.UnhideChatForUser(c.Request.Context(), chatID, chat.User2ID)

	h.hub.BroadcastChatMessage(chatID, msg)
	c.JSON(http.StatusCreated, msg)
}

// DeleteMessageForMe performs a soft delete of a message for the caller.
func (h *ChatHandler) DeleteMessageForMe(c *gin.Context) {
	chatID, messageID, ok := parseIDs(c)
	if !ok {
		return
	}

	userID := c.GetInt("userID")
	chat, err := h.chatRepo.GetChat(c.Request.Context(), chatID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrChatNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "chat not found"})
		return
	}
	if !isChatParticipant(chat, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	msg, err := h.messageRepo.GetMessage(c.Request.Context(), messageID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrMessageNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "message not found"})
		return
	}
	if msg.ChatID != chatID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message does not belong to chat"})
		return
	}

	isSender := msg.SenderID == userID
	if !isSender && !isChatParticipant(chat, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a participant"})
		return
	}

	if err := h.messageRepo.SoftDeleteMessageForUser(c.Request.Context(), messageID, isSender); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete message"})
		return
	}

	c.Status(http.StatusNoContent)
}

// DeleteMessageForAll marks a message as deleted for everyone (sender only).
func (h *ChatHandler) DeleteMessageForAll(c *gin.Context) {
	chatID, messageID, ok := parseIDs(c)
	if !ok {
		return
	}

	userID := c.GetInt("userID")
	chat, err := h.chatRepo.GetChat(c.Request.Context(), chatID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrChatNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "chat not found"})
		return
	}
	if !isChatParticipant(chat, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	msg, err := h.messageRepo.GetMessage(c.Request.Context(), messageID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrMessageNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "message not found"})
		return
	}
	if msg.ChatID != chatID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message does not belong to chat"})
		return
	}
	if msg.SenderID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only sender can delete for all"})
		return
	}

	if err := h.messageRepo.DeleteMessageForAll(c.Request.Context(), messageID, userID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrMessageNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "could not delete message"})
		return
	}

	h.hub.BroadcastDeletion(chatID, messageID)
	c.Status(http.StatusNoContent)
}

// DeleteChatForMe hides the chat for the requester.
func (h *ChatHandler) DeleteChatForMe(c *gin.Context) {
	chatID, err := strconv.Atoi(c.Param("chat_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat id"})
		return
	}
	userID := c.GetInt("userID")

	chat, err := h.chatRepo.GetChat(c.Request.Context(), chatID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrChatNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "chat not found"})
		return
	}
	if !isChatParticipant(chat, userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not allowed"})
		return
	}

	if err := h.chatRepo.HideChatForUser(c.Request.Context(), chatID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hide chat"})
		return
	}

	c.Status(http.StatusNoContent)
}

func parseIDs(c *gin.Context) (int, int, bool) {
	chatID, err := strconv.Atoi(c.Param("chat_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat id"})
		return 0, 0, false
	}
	msgID, err := strconv.Atoi(c.Param("message_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return 0, 0, false
	}
	return chatID, msgID, true
}

func isChatParticipant(chat models.Chat, userID int) bool {
	return chat.User1ID == userID || chat.User2ID == userID
}
