package repositories

import (
	"context"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"

	"chat-service/internal/models"
)

// GroupMessageRepository defines interactions for group messages.
type GroupMessageRepository interface {
	CreateGroupMessage(ctx context.Context, groupID int, senderID int, content string) (models.GroupMessage, error)
	ListGroupMessages(ctx context.Context, groupID int) ([]models.GroupMessage, error)
	GetGroupMessage(ctx context.Context, messageID int) (models.GroupMessage, error)
	DeleteForAll(ctx context.Context, messageID int, senderID int) error
}

// GroupMessageRepo is a sqlx-backed implementation.
type GroupMessageRepo struct {
	db *sqlx.DB
}

// NewGroupMessageRepo constructs a GroupMessageRepo.
func NewGroupMessageRepo(db *sqlx.DB) *GroupMessageRepo {
	return &GroupMessageRepo{db: db}
}

// CreateGroupMessage persists a group message.
func (r *GroupMessageRepo) CreateGroupMessage(ctx context.Context, groupID int, senderID int, content string) (models.GroupMessage, error) {
	var msg models.GroupMessage
	err := r.db.QueryRowxContext(ctx, `INSERT INTO group_messages (group_id, sender_id, content) VALUES ($1, $2, $3) RETURNING id, group_id, sender_id, content, deleted_for_all, created_at`, groupID, senderID, content).
		Scan(&msg.ID, &msg.GroupID, &msg.SenderID, &msg.Content, &msg.DeletedForAll, &msg.CreatedAt)
	return msg, err
}

// ListGroupMessages returns messages ordered by creation, excluding deleted_for_all.
func (r *GroupMessageRepo) ListGroupMessages(ctx context.Context, groupID int) ([]models.GroupMessage, error) {
	var msgs []models.GroupMessage
	err := r.db.SelectContext(ctx, &msgs, `SELECT id, group_id, sender_id, content, deleted_for_all, created_at FROM group_messages WHERE group_id=$1 AND deleted_for_all = FALSE ORDER BY created_at ASC`, groupID)
	return msgs, err
}

// GetGroupMessage fetches a single message.
func (r *GroupMessageRepo) GetGroupMessage(ctx context.Context, messageID int) (models.GroupMessage, error) {
	var msg models.GroupMessage
	err := r.db.GetContext(ctx, &msg, `SELECT id, group_id, sender_id, content, deleted_for_all, created_at FROM group_messages WHERE id=$1`, messageID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.GroupMessage{}, ErrMessageNotFound
	}
	return msg, err
}

// DeleteForAll marks a message deleted for everyone (sender only).
func (r *GroupMessageRepo) DeleteForAll(ctx context.Context, messageID int, senderID int) error {
	res, err := r.db.ExecContext(ctx, `UPDATE group_messages SET deleted_for_all = TRUE WHERE id=$1 AND sender_id=$2`, messageID, senderID)
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
