package models

import "time"

// Message represents a chat message.
type Message struct {
	ID                int       `db:"id" json:"id"`
	ChatID            int       `db:"chat_id" json:"chat_id"`
	SenderID          int       `db:"sender_id" json:"sender_id"`
	Content           string    `db:"content" json:"content"`
	DeletedBySender   bool      `db:"deleted_by_sender" json:"deleted_by_sender"`
	DeletedByReceiver bool      `db:"deleted_by_receiver" json:"deleted_by_receiver"`
	DeletedForAll     bool      `db:"deleted_for_all" json:"deleted_for_all"`
	CreatedAt         time.Time `db:"created_at" json:"created_at"`
}

// ChatEvent is broadcasted through websockets.
type ChatEvent struct {
	Type      string   `json:"type"`
	Message   *Message `json:"message,omitempty"`
	MessageID int      `json:"message_id,omitempty"`
}
