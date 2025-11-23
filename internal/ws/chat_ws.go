package ws

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	grpcclient "chat-service/internal/grpc"
	"chat-service/internal/repositories"
)

// ChatWebSocketHandler handles chat websocket connections.
type ChatWebSocketHandler struct {
	hub        *Hub
	chatRepo   repositories.ChatRepository
	authClient *grpcclient.AuthClient
}

// NewChatWebSocketHandler constructs a ChatWebSocketHandler.
func NewChatWebSocketHandler(hub *Hub, chatRepo repositories.ChatRepository, authClient *grpcclient.AuthClient) *ChatWebSocketHandler {
	return &ChatWebSocketHandler{hub: hub, chatRepo: chatRepo, authClient: authClient}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// Handle upgrades the connection and registers client.
func (h *ChatWebSocketHandler) Handle(c *gin.Context) {
	chatID, err := strconv.Atoi(c.Param("chat_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid chat id"})
		return
	}

	token := c.GetHeader("Authorization")
	if token == "" {
		token = c.Query("token")
		if token != "" {
			token = "Bearer " + token
		}
	}

	userID, err := h.validateToken(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		return
	}

	member, err := h.chatRepo.IsParticipant(c.Request.Context(), chatID, userID)
	if err != nil || !member {
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for chat"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	h.hub.AddChatClient(chatID, conn)

	// Keep connection alive and clean on close
	go func() {
		defer func() {
			h.hub.RemoveChatClient(chatID, conn)
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (h *ChatWebSocketHandler) validateToken(ctx context.Context, header string) (int, error) {
	parts := strings.Split(header, " ")
	if len(parts) == 2 {
		return h.authClient.ValidateToken(ctx, parts[1])
	}
	return 0, fmt.Errorf("invalid token")
}
