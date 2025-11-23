package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"

	"chat-service/internal/models"
)

// Hub maintains active websocket rooms.
type Hub struct {
	chatRooms map[int]map[*websocket.Conn]bool
	mu        sync.RWMutex
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{
		chatRooms: make(map[int]map[*websocket.Conn]bool),
	}
}

// AddChatClient registers a websocket connection to a chat room.
func (h *Hub) AddChatClient(chatID int, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.chatRooms[chatID]; !ok {
		h.chatRooms[chatID] = make(map[*websocket.Conn]bool)
	}
	h.chatRooms[chatID][conn] = true
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
		}
	}
}
