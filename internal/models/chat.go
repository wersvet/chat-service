package models

import "time"

// Chat represents a private chat between exactly two users.
type Chat struct {
	ID        int       `db:"id" json:"id"`
	User1ID   int       `db:"user1_id" json:"user1_id"`
	User2ID   int       `db:"user2_id" json:"user2_id"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ChatSummary provides API-friendly view of a chat for a user.
type ChatSummary struct {
	ChatID   int       `db:"id" json:"chat_id"`
	FriendID int       `json:"friend_id"`
	Created  time.Time `db:"created_at" json:"created_at"`
}

// ChatVisibility models per-user chat visibility state.
type ChatVisibility struct {
	ChatID int  `db:"chat_id" json:"chat_id"`
	UserID int  `db:"user_id" json:"user_id"`
	Hidden bool `db:"hidden" json:"hidden"`
}
