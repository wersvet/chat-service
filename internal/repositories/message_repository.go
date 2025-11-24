package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"chat-service/internal/models"
)

var ErrMessageNotFound = errors.New("message not found")

// MessageRepository defines interactions for chat messages.
type MessageRepository interface {
	CreateChatMessage(ctx context.Context, chatID int, senderID int, content string) (models.Message, error)
	GetChatMessagesForUser(ctx context.Context, chatID int, userID int) ([]models.Message, error)
	GetMessage(ctx context.Context, messageID int) (models.Message, error)
	SoftDeleteMessageForUser(ctx context.Context, messageID int, isSender bool) error
	DeleteMessageForAll(ctx context.Context, messageID int, userID int) error
}

// MessageRepo is a sqlx-backed repository.
type MessageRepo struct {
	db *sqlx.DB
}

// NewMessageRepo constructs MessageRepo.
func NewMessageRepo(db *sqlx.DB) *MessageRepo {
	return &MessageRepo{db: db}
}

// CreateChatMessage stores a message in a private chat.
func (r *MessageRepo) CreateChatMessage(ctx context.Context, chatID int, senderID int, content string) (models.Message, error) {
	var msg models.Message
	err := r.db.QueryRowxContext(ctx, `INSERT INTO messages (chat_id, sender_id, content) VALUES ($1, $2, $3) RETURNING id, chat_id, sender_id, content, deleted_by_sender, deleted_by_receiver, deleted_for_all, created_at`, chatID, senderID, content).
		Scan(&msg.ID, &msg.ChatID, &msg.SenderID, &msg.Content, &msg.DeletedBySender, &msg.DeletedByReceiver, &msg.DeletedForAll, &msg.CreatedAt)
	return msg, err
}

// GetChatMessagesForUser returns ordered chat messages filtered per user visibility rules.
func (r *MessageRepo) GetChatMessagesForUser(ctx context.Context, chatID int, userID int) ([]models.Message, error) {
	query := `SELECT id, chat_id, sender_id, content, deleted_by_sender, deleted_by_receiver, deleted_for_all, created_at
        FROM messages
        WHERE chat_id=$1
        AND deleted_for_all = FALSE
        AND NOT (sender_id=$2 AND deleted_by_sender = TRUE)
        AND NOT (sender_id<>$2 AND deleted_by_receiver = TRUE)
        ORDER BY created_at ASC`
	var msgs []models.Message
	err := r.db.SelectContext(ctx, &msgs, query, chatID, userID)
	return msgs, err
}

// GetMessage retrieves a single message.
func (r *MessageRepo) GetMessage(ctx context.Context, messageID int) (models.Message, error) {
	var msg models.Message
	err := r.db.GetContext(ctx, &msg, `SELECT id, chat_id, sender_id, content, deleted_by_sender, deleted_by_receiver, deleted_for_all, created_at FROM messages WHERE id=$1`, messageID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Message{}, ErrMessageNotFound
	}
	return msg, err
}

// SoftDeleteMessageForUser marks a message as deleted for either sender or receiver.
func (r *MessageRepo) SoftDeleteMessageForUser(ctx context.Context, messageID int, isSender bool) error {
	if isSender {
		_, err := r.db.ExecContext(ctx, `UPDATE messages SET deleted_by_sender = TRUE WHERE id=$1`, messageID)
		return err
	}
	_, err := r.db.ExecContext(ctx, `UPDATE messages SET deleted_by_receiver = TRUE WHERE id=$1`, messageID)
	return err
}

// DeleteMessageForAll marks a message as deleted for everyone.
func (r *MessageRepo) DeleteMessageForAll(ctx context.Context, messageID int, userID int) error {
	res, err := r.db.ExecContext(ctx, `UPDATE messages SET deleted_for_all = TRUE WHERE id=$1 AND sender_id=$2`, messageID, userID)
	if err != nil {
		return err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return ErrMessageNotFound
	}
	return nil
}
