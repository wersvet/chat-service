package models

import "time"

// Group represents a chat group.
type Group struct {
	ID        int       `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	OwnerID   int       `db:"owner_id" json:"owner_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// GroupMessage represents a message sent in a group.
type GroupMessage struct {
	ID            int       `db:"id" json:"id"`
	GroupID       int       `db:"group_id" json:"group_id"`
	SenderID      int       `db:"sender_id" json:"sender_id"`
	Content       string    `db:"content" json:"content"`
	DeletedForAll bool      `db:"deleted_for_all" json:"deleted_for_all"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

// GroupEvent is emitted over WebSocket connections for groups.
type GroupEvent struct {
	Type      string        `json:"type"`
	Message   *GroupMessage `json:"message,omitempty"`
	MessageID int           `json:"message_id,omitempty"`
}
