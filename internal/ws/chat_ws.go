package ws

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.opentelemetry.io/otel"

	grpcclient "chat-service/internal/grpc"
	"chat-service/internal/observability"
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

	ctx, span := otel.Tracer("chat-service/ws").Start(c.Request.Context(), "ws.handshake")
	defer span.End()
	c.Request = c.Request.WithContext(ctx)

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
	traceID := span.SpanContext().TraceID().String()
	requestID := observability.RequestIDFromRequest(c.Request)
	info := ConnInfo{
		ConnID:      newConnID(),
		UserID:      userID,
		DeviceID:    observability.DeviceIDFromRequest(c.Request),
		IP:          observability.IPFromRequest(c.Request),
		RequestID:   requestID,
		TraceID:     traceID,
		ConnectedAt: time.Now(),
	}
	h.hub.AddChatClient(chatID, conn, info)

	observability.IncWSActive("chat")
	observability.IncWSEvent("chat", "ws_connect")
	_ = observability.PublishEvent(ctx, "ws_events.chats", observability.EventEnvelope{
		EventType: "ws_events",
		EventName: "ws_connect",
		Payload: map[string]interface{}{
			"ws": map[string]interface{}{
				"kind":        "chat",
				"resource_id": chatID,
				"event":       "ws_connect",
				"conn_id":     info.ConnID,
				"duration_ms": 0,
				"reason":      "",
			},
			"identity": map[string]interface{}{
				"user_id":   info.UserID,
				"device_id": info.DeviceID,
				"ip":        info.IP,
			},
		},
	}, observability.BuildHeaders(requestID, traceID))

	// Keep connection alive and clean on close
	go func() {
		var closeReason string
		defer func() {
			h.hub.RemoveChatClient(chatID, conn)
			observability.DecWSActive("chat")
			observability.IncWSEvent("chat", "ws_disconnect")
			_ = observability.PublishEvent(ctx, "ws_events.chats", observability.EventEnvelope{
				EventType: "ws_events",
				EventName: "ws_disconnect",
				Payload: map[string]interface{}{
					"ws": map[string]interface{}{
						"kind":        "chat",
						"resource_id": chatID,
						"event":       "ws_disconnect",
						"conn_id":     info.ConnID,
						"duration_ms": time.Since(info.ConnectedAt).Milliseconds(),
						"reason":      closeReason,
					},
					"identity": map[string]interface{}{
						"user_id":   info.UserID,
						"device_id": info.DeviceID,
						"ip":        info.IP,
					},
				},
			}, observability.BuildHeaders(requestID, traceID))
			conn.Close()
		}()
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				closeReason = err.Error()
				if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					observability.IncWSEvent("chat", "ws_error")
					_ = observability.PublishEvent(ctx, "ws_events.chats", observability.EventEnvelope{
						EventType: "ws_events",
						EventName: "ws_error",
						Payload: map[string]interface{}{
							"ws": map[string]interface{}{
								"kind":        "chat",
								"resource_id": chatID,
								"event":       "ws_error",
								"conn_id":     info.ConnID,
								"duration_ms": time.Since(info.ConnectedAt).Milliseconds(),
								"reason":      closeReason,
							},
							"identity": map[string]interface{}{
								"user_id":   info.UserID,
								"device_id": info.DeviceID,
								"ip":        info.IP,
							},
						},
					}, observability.BuildHeaders(requestID, traceID))
				}
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
