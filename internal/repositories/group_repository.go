package repositories

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	"github.com/jmoiron/sqlx"

	"chat-service/internal/models"
)

var ErrGroupNotFound = errors.New("group not found")

// GroupRepository abstracts group persistence.
type GroupRepository interface {
	CreateGroup(ctx context.Context, ownerID int, name string, memberIDs []int) (models.Group, error)
	ListGroupsForUser(ctx context.Context, userID int) ([]models.Group, error)
	IsMember(ctx context.Context, groupID int, userID int) (bool, error)
	GetGroup(ctx context.Context, groupID int) (models.Group, error)
}

// GroupRepo is a sqlx implementation of GroupRepository.
type GroupRepo struct {
	db *sqlx.DB
}

// NewGroupRepo constructs a GroupRepo.
func NewGroupRepo(db *sqlx.DB) *GroupRepo {
	return &GroupRepo{db: db}
}

// CreateGroup creates a group and its members atomically.
func (r *GroupRepo) CreateGroup(ctx context.Context, ownerID int, name string, memberIDs []int) (models.Group, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return models.Group{}, err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	var group models.Group
	if err = tx.QueryRowxContext(ctx, `INSERT INTO groups (name, owner_id) VALUES ($1, $2) RETURNING id, name, owner_id, created_at`, name, ownerID).
		Scan(&group.ID, &group.Name, &group.OwnerID, &group.CreatedAt); err != nil {
		return models.Group{}, err
	}

	// ensure owner present and dedupe members
	memberSet := map[int]struct{}{ownerID: {}}
	for _, id := range memberIDs {
		memberSet[id] = struct{}{}
	}
	ids := make([]int, 0, len(memberSet))
	for id := range memberSet {
		ids = append(ids, id)
	}
	sort.Ints(ids)

	for _, id := range ids {
		if _, err = tx.ExecContext(ctx, `INSERT INTO group_members (group_id, user_id) VALUES ($1, $2)`, group.ID, id); err != nil {
			return models.Group{}, err
		}
	}

	if err = tx.Commit(); err != nil {
		return models.Group{}, err
	}
	return group, nil
}

// ListGroupsForUser returns groups that include the user.
func (r *GroupRepo) ListGroupsForUser(ctx context.Context, userID int) ([]models.Group, error) {
	var groups []models.Group
	err := r.db.SelectContext(ctx, &groups, `SELECT g.id, g.name, g.owner_id, g.created_at FROM groups g INNER JOIN group_members gm ON gm.group_id = g.id WHERE gm.user_id=$1 ORDER BY g.created_at DESC`, userID)
	return groups, err
}

// IsMember checks membership.
func (r *GroupRepo) IsMember(ctx context.Context, groupID int, userID int) (bool, error) {
	var exists bool
	err := r.db.GetContext(ctx, &exists, `SELECT EXISTS(SELECT 1 FROM group_members WHERE group_id=$1 AND user_id=$2)`, groupID, userID)
	return exists, err
}

// GetGroup fetches a single group.
func (r *GroupRepo) GetGroup(ctx context.Context, groupID int) (models.Group, error) {
	var group models.Group
	err := r.db.GetContext(ctx, &group, `SELECT id, name, owner_id, created_at FROM groups WHERE id=$1`, groupID)
	if errors.Is(err, sql.ErrNoRows) {
		return models.Group{}, ErrGroupNotFound
	}
	return group, err
}
