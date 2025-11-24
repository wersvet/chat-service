package ws

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	grpcclient "chat-service/internal/grpc"
	"chat-service/internal/repositories"

	"github.com/gin-gonic/gin"
)

// GroupWebSocketHandler handles group websocket connections.
type GroupWebSocketHandler struct {
	hub        *Hub
	groupRepo  repositories.GroupRepository
	authClient *grpcclient.AuthClient
}

// NewGroupWebSocketHandler constructs a GroupWebSocketHandler.
func NewGroupWebSocketHandler(hub *Hub, groupRepo repositories.GroupRepository, authClient *grpcclient.AuthClient) *GroupWebSocketHandler {
	return &GroupWebSocketHandler{hub: hub, groupRepo: groupRepo, authClient: authClient}
}

// Handle upgrades and registers a websocket connection for group chats.
func (h *GroupWebSocketHandler) Handle(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("group_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
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

	member, err := h.groupRepo.IsMember(c.Request.Context(), groupID, userID)
	if err != nil || !member {
		c.JSON(http.StatusForbidden, gin.H{"error": "not authorized for group"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	h.hub.AddGroupClient(groupID, conn)

	go func() {
		defer func() {
			h.hub.RemoveGroupClient(groupID, conn)
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()
}

func (h *GroupWebSocketHandler) validateToken(ctx context.Context, header string) (int, error) {
	parts := strings.Split(header, " ")
	if len(parts) == 2 {
		return h.authClient.ValidateToken(ctx, parts[1])
	}
	return 0, fmt.Errorf("invalid token")
}
