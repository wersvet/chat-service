package repositories

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	"github.com/jmoiron/sqlx"

	"chat-service/internal/models"
)

var ErrChatNotFound = errors.New("chat not found")

// ChatRepository abstracts chat persistence.
type ChatRepository interface {
	CreateOrGetChat(ctx context.Context, userID int, friendID int) (models.Chat, error)
	IsParticipant(ctx context.Context, chatID int, userID int) (bool, error)
	GetChat(ctx context.Context, chatID int) (models.Chat, error)
	ListChats(ctx context.Context, userID int) ([]models.ChatSummary, error)
	HideChatForUser(ctx context.Context, chatID int, userID int) error
	UnhideChatForUser(ctx context.Context, chatID int, userID int) error
}

// ChatRepo is a sqlx implementation of ChatRepository.
type ChatRepo struct {
	db *sqlx.DB
}

// NewChatRepo constructs a ChatRepo.
func NewChatRepo(db *sqlx.DB) *ChatRepo {
	return &ChatRepo{db: db}
}

// CreateOrGetChat creates a chat between two users if it does not already exist.
func (r *ChatRepo) CreateOrGetChat(ctx context.Context, userID int, friendID int) (models.Chat, error) {
	if userID == friendID {
		return models.Chat{}, errors.New("cannot create chat with self")
	}
	participants := []int{userID, friendID}
	sort.Ints(participants)
	user1, user2 := participants[0], participants[1]

	var chat models.Chat
	query := `SELECT id, user1_id, user2_id, created_at FROM chats WHERE user1_id=$1 AND user2_id=$2`
	if err := r.db.GetContext(ctx, &chat, query, user1, user2); err != nil {
		if err != sql.ErrNoRows {
			return models.Chat{}, err
		}
		if err := r.db.QueryRowxContext(ctx, `INSERT INTO chats (user1_id, user2_id) VALUES ($1, $2) RETURNING id, user1_id, user2_id, created_at`, user1, user2).
			Scan(&chat.ID, &chat.User1ID, &chat.User2ID, &chat.CreatedAt); err != nil {
			return models.Chat{}, err
		}
	} else {
		// ensure participants column order matches request
	}

	if err := r.UnhideChatForUser(ctx, chat.ID, userID); err != nil {
		return models.Chat{}, err
	}
	if err := r.UnhideChatForUser(ctx, chat.ID, friendID); err != nil {
		return models.Chat{}, err
	}
	return chat, nil
}

// IsParticipant checks whether a user belongs to the chat.
func (r *ChatRepo) IsParticipant(ctx context.Context, chatID int, userID int) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM chats WHERE id=$1 AND (user1_id=$2 OR user2_id=$2))`, chatID, userID)
	return exists, err
}

// GetChat fetches a chat by id.
func (r *ChatRepo) GetChat(ctx context.Context, chatID int) (models.Chat, error) {
	var chat models.Chat
	err := r.db.GetContext(ctx, &chat, `SELECT id, user1_id, user2_id, created_at FROM chats WHERE id=$1`, chatID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Chat{}, ErrChatNotFound
	}
	return chat, err
}

// ListChats returns chats visible to the user.
func (r *ChatRepo) ListChats(ctx context.Context, userID int) ([]models.ChatSummary, error) {
	query := `SELECT c.id, c.user1_id, c.user2_id, c.created_at FROM chats c
        LEFT JOIN chat_visibility cv ON cv.chat_id = c.id AND cv.user_id=$1
        WHERE (c.user1_id=$1 OR c.user2_id=$1) AND (cv.hidden IS NULL OR cv.hidden = FALSE)
        ORDER BY created_at DESC`
	rows, err := r.db.QueryxContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.ChatSummary
	for rows.Next() {
		var chat models.Chat
		if err := rows.StructScan(&chat); err != nil {
			return nil, err
		}
		friendID := chat.User1ID
		if friendID == userID {
			friendID = chat.User2ID
		}
		result = append(result, models.ChatSummary{ChatID: chat.ID, FriendID: friendID, Created: chat.CreatedAt})
	}
	return result, rows.Err()
}

// HideChatForUser marks a chat hidden for the user.
func (r *ChatRepo) HideChatForUser(ctx context.Context, chatID int, userID int) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO chat_visibility (chat_id, user_id, hidden) VALUES ($1, $2, TRUE)
        ON CONFLICT (chat_id, user_id) DO UPDATE SET hidden = EXCLUDED.hidden`, chatID, userID)
	return err
}

// UnhideChatForUser removes the hidden flag for the user.
func (r *ChatRepo) UnhideChatForUser(ctx context.Context, chatID int, userID int) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO chat_visibility (chat_id, user_id, hidden) VALUES ($1, $2, FALSE)
        ON CONFLICT (chat_id, user_id) DO UPDATE SET hidden = FALSE`, chatID, userID)
	return err
}
