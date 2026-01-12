package ws

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"chat-service/internal/models"
	"chat-service/internal/observability"
)

// Hub maintains active websocket rooms.
type Hub struct {
	chatRooms     map[int]map[*websocket.Conn]bool
	groupRooms    map[int]map[*websocket.Conn]bool
	chatConnInfo  map[int]map[*websocket.Conn]ConnInfo
	groupConnInfo map[int]map[*websocket.Conn]ConnInfo
	mu            sync.RWMutex
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{
		chatRooms:     make(map[int]map[*websocket.Conn]bool),
		groupRooms:    make(map[int]map[*websocket.Conn]bool),
		chatConnInfo:  make(map[int]map[*websocket.Conn]ConnInfo),
		groupConnInfo: make(map[int]map[*websocket.Conn]ConnInfo),
	}
}

// AddChatClient registers a websocket connection to a chat room.
func (h *Hub) AddChatClient(chatID int, conn *websocket.Conn, info ConnInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.chatRooms[chatID]; !ok {
		h.chatRooms[chatID] = make(map[*websocket.Conn]bool)
	}
	h.chatRooms[chatID][conn] = true
	if _, ok := h.chatConnInfo[chatID]; !ok {
		h.chatConnInfo[chatID] = make(map[*websocket.Conn]ConnInfo)
	}
	h.chatConnInfo[chatID][conn] = info
}

// RemoveChatClient removes a chat websocket connection.
func (h *Hub) RemoveChatClient(chatID int, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.chatRooms[chatID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.chatRooms, chatID)
		}
	}
	if infos, ok := h.chatConnInfo[chatID]; ok {
		delete(infos, conn)
		if len(infos) == 0 {
			delete(h.chatConnInfo, chatID)
		}
	}
}

// BroadcastChatMessage sends message to all clients in a chat.
func (h *Hub) BroadcastChatMessage(chatID int, msg models.Message) {
	h.mu.RLock()
	conns := h.chatRooms[chatID]
	h.mu.RUnlock()

	event := models.ChatEvent{Type: "message", Message: &msg}
	payload, _ := json.Marshal(event)
	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("websocket write error: %v", err)
			conn.Close()
			h.RemoveChatClient(chatID, conn)
			h.publishWSError("chat", chatID, conn, err)
		}
	}
}

// BroadcastDeletion notifies clients of a delete-for-all event.
func (h *Hub) BroadcastDeletion(chatID int, messageID int) {
	h.mu.RLock()
	conns := h.chatRooms[chatID]
	h.mu.RUnlock()

	event := models.ChatEvent{Type: "delete_for_all", MessageID: messageID}
	payload, _ := json.Marshal(event)
	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("websocket write error: %v", err)
			conn.Close()
			h.RemoveChatClient(chatID, conn)
			h.publishWSError("chat", chatID, conn, err)
		}
	}
}

// AddGroupClient registers a websocket connection to a group room.
func (h *Hub) AddGroupClient(groupID int, conn *websocket.Conn, info ConnInfo) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.groupRooms[groupID]; !ok {
		h.groupRooms[groupID] = make(map[*websocket.Conn]bool)
	}
	h.groupRooms[groupID][conn] = true
	if _, ok := h.groupConnInfo[groupID]; !ok {
		h.groupConnInfo[groupID] = make(map[*websocket.Conn]ConnInfo)
	}
	h.groupConnInfo[groupID][conn] = info
}

// RemoveGroupClient removes a group websocket connection.
func (h *Hub) RemoveGroupClient(groupID int, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conns, ok := h.groupRooms[groupID]; ok {
		delete(conns, conn)
		if len(conns) == 0 {
			delete(h.groupRooms, groupID)
		}
	}
	if infos, ok := h.groupConnInfo[groupID]; ok {
		delete(infos, conn)
		if len(infos) == 0 {
			delete(h.groupConnInfo, groupID)
		}
	}
}

// BroadcastGroupMessage sends message to all clients in a group.
func (h *Hub) BroadcastGroupMessage(groupID int, msg models.GroupMessage) {
	h.mu.RLock()
	conns := h.groupRooms[groupID]
	h.mu.RUnlock()

	event := models.GroupEvent{Type: "message", Message: &msg}
	payload, _ := json.Marshal(event)
	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("websocket write error: %v", err)
			conn.Close()
			h.RemoveGroupClient(groupID, conn)
			h.publishWSError("group", groupID, conn, err)
		}
	}
}

// BroadcastGroupDeletion notifies clients of a delete-for-all event.
func (h *Hub) BroadcastGroupDeletion(groupID int, messageID int) {
	h.mu.RLock()
	conns := h.groupRooms[groupID]
	h.mu.RUnlock()

	event := models.GroupEvent{Type: "delete_for_all", MessageID: messageID}
	payload, _ := json.Marshal(event)
	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
			log.Printf("websocket write error: %v", err)
			conn.Close()
			h.RemoveGroupClient(groupID, conn)
			h.publishWSError("group", groupID, conn, err)
		}
	}
}

func (h *Hub) publishWSError(kind string, resourceID int, conn *websocket.Conn, err error) {
	info, ok := h.getConnInfo(kind, resourceID, conn)
	if !ok {
		return
	}

	payload := map[string]interface{}{
		"ws": map[string]interface{}{
			"kind":        kind,
			"resource_id": resourceID,
			"event":       "ws_error",
			"conn_id":     info.ConnID,
			"duration_ms": time.Since(info.ConnectedAt).Milliseconds(),
			"reason":      err.Error(),
		},
		"identity": map[string]interface{}{
			"user_id":   info.UserID,
			"device_id": info.DeviceID,
			"ip":        info.IP,
		},
	}

	headers := observability.BuildHeaders(info.RequestID, info.TraceID)
	_ = observability.PublishEvent(context.Background(), wsRoutingKey(kind), observability.EventEnvelope{
		EventType: "ws_events",
		EventName: "ws_error",
		Payload:   payload,
	}, headers)
	observability.IncWSEvent(kind, "ws_error")
}

func (h *Hub) getConnInfo(kind string, resourceID int, conn *websocket.Conn) (ConnInfo, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if kind == "chat" {
		if infos, ok := h.chatConnInfo[resourceID]; ok {
			info, exists := infos[conn]
			return info, exists
		}
		return ConnInfo{}, false
	}
	if infos, ok := h.groupConnInfo[resourceID]; ok {
		info, exists := infos[conn]
		return info, exists
	}
	return ConnInfo{}, false
}

func wsRoutingKey(kind string) string {
	if kind == "group" {
		return "ws_events.groups"
	}
	return "ws_events.chats"
}
