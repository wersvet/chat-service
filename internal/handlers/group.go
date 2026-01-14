package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"chat-service/internal/models"
	"chat-service/internal/repositories"
	"chat-service/internal/telemetry"
	"chat-service/internal/ws"
)

// GroupHandler manages group-related endpoints.
type GroupHandler struct {
	groupRepo   repositories.GroupRepository
	messageRepo repositories.GroupMessageRepository
	userClient  userClient
	hub         *ws.Hub
	audit       *telemetry.AuditEmitter
}

// NewGroupHandler constructs a GroupHandler.
func NewGroupHandler(groupRepo repositories.GroupRepository, messageRepo repositories.GroupMessageRepository, userClient userClient, hub *ws.Hub, audit *telemetry.AuditEmitter) *GroupHandler {
	return &GroupHandler{
		groupRepo:   groupRepo,
		messageRepo: messageRepo,
		userClient:  userClient,
		hub:         hub,
		audit:       audit,
	}
}

// CreateGroup handles POST /groups.
func (h *GroupHandler) CreateGroup(c *gin.Context) {
	userID := c.GetInt("userID")

	var req struct {
		Name      string `json:"name" binding:"required"`
		MemberIDs []int  `json:"member_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.emitAudit(c, "ERROR", "invalid request payload")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate members exist via user-service
	if len(req.MemberIDs) > 0 {
		if _, err := h.userClient.BulkUsers(c.Request.Context(), req.MemberIDs); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to validate members"})
			return
		}
	}

	group, err := h.groupRepo.CreateGroup(c.Request.Context(), userID, req.Name, req.MemberIDs)
	if err != nil {
		h.emitAudit(c, "ERROR", "internal error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create group"})
		return
	}

	h.emitAudit(c, "INFO", "Group created")
	c.JSON(http.StatusCreated, gin.H{"group_id": group.ID})
}

// ListGroups returns groups the caller belongs to.
func (h *GroupHandler) ListGroups(c *gin.Context) {
	userID := c.GetInt("userID")
	groups, err := h.groupRepo.ListGroupsForUser(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load groups"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"groups": groups})
}

// GetGroupMessages returns messages in the group.
func (h *GroupHandler) GetGroupMessages(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("group_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	userID := c.GetInt("userID")
	member, err := h.groupRepo.IsMember(c.Request.Context(), groupID, userID)
	if err != nil {
		h.emitAudit(c, "ERROR", "internal error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "membership check failed"})
		return
	}
	if !member {
		h.emitAudit(c, "ERROR", "not allowed")
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}

	msgs, err := h.messageRepo.ListGroupMessages(c.Request.Context(), groupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load messages"})
		return
	}

	senderIDs := make([]int, 0, len(msgs))
	seen := map[int]struct{}{}
	for _, m := range msgs {
		if _, ok := seen[m.SenderID]; !ok {
			seen[m.SenderID] = struct{}{}
			senderIDs = append(senderIDs, m.SenderID)
		}
	}

	usernameByID := map[int]string{}
	if len(senderIDs) > 0 {
		users, err := h.userClient.BulkUsers(c.Request.Context(), senderIDs)
		if err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to load senders"})
			return
		}
		for _, u := range users {
			usernameByID[int(u.Id)] = u.Username
		}
	}

	type messageResponse struct {
		models.GroupMessage
		SenderUsername string `json:"sender_username,omitempty"`
	}

	resp := make([]messageResponse, 0, len(msgs))
	for _, m := range msgs {
		resp = append(resp, messageResponse{GroupMessage: m, SenderUsername: usernameByID[m.SenderID]})
	}

	c.JSON(http.StatusOK, gin.H{"messages": resp})
}

// PostGroupMessage persists and broadcasts a group message.
func (h *GroupHandler) PostGroupMessage(c *gin.Context) {
	groupID, err := strconv.Atoi(c.Param("group_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return
	}

	userID := c.GetInt("userID")
	member, err := h.groupRepo.IsMember(c.Request.Context(), groupID, userID)
	if err != nil {
		h.emitAudit(c, "ERROR", "internal error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "membership check failed"})
		return
	}
	if !member {
		h.emitAudit(c, "ERROR", "not allowed")
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		h.emitAudit(c, "ERROR", "invalid request payload")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := h.messageRepo.CreateGroupMessage(c.Request.Context(), groupID, userID, req.Content)
	if err != nil {
		h.emitAudit(c, "ERROR", "internal error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store message"})
		return
	}

	h.hub.BroadcastGroupMessage(groupID, msg)
	h.emitAudit(c, "INFO", "Group message sent")
	c.JSON(http.StatusCreated, msg)
}

// DeleteGroupMessageForAll deletes a message for everyone when invoked by the sender.
func (h *GroupHandler) DeleteGroupMessageForAll(c *gin.Context) {
	groupID, messageID, ok := parseGroupIDs(c)
	if !ok {
		return
	}

	userID := c.GetInt("userID")
	member, err := h.groupRepo.IsMember(c.Request.Context(), groupID, userID)
	if err != nil {
		h.emitAudit(c, "ERROR", "internal error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "membership check failed"})
		return
	}
	if !member {
		h.emitAudit(c, "ERROR", "not allowed to delete for all")
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}

	msg, err := h.messageRepo.GetGroupMessage(c.Request.Context(), messageID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrMessageNotFound) {
			status = http.StatusNotFound
		}
		if status == http.StatusNotFound {
			h.emitAudit(c, "ERROR", "message not found")
		} else {
			h.emitAudit(c, "ERROR", "internal error")
		}
		c.JSON(status, gin.H{"error": "message not found"})
		return
	}
	if msg.GroupID != groupID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message does not belong to group"})
		return
	}
	if msg.SenderID != userID {
		h.emitAudit(c, "ERROR", "not allowed to delete for all")
		c.JSON(http.StatusForbidden, gin.H{"error": "only sender may delete"})
		return
	}

	if err := h.messageRepo.DeleteForAll(c.Request.Context(), messageID, userID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrMessageNotFound) {
			status = http.StatusNotFound
		}
		if status == http.StatusNotFound {
			h.emitAudit(c, "ERROR", "message not found")
		} else {
			h.emitAudit(c, "ERROR", "internal error")
		}
		c.JSON(status, gin.H{"error": "could not delete"})
		return
	}

	h.hub.BroadcastGroupDeletion(groupID, messageID)
	h.emitAudit(c, "INFO", "Group message deleted for all")
	c.Status(http.StatusNoContent)
}

func (h *GroupHandler) emitAudit(c *gin.Context, level, text string) {
	if h.audit == nil {
		return
	}
	h.audit.Emit(c.Request.Context(), level, text, requestIDFromContext(c), userIDFromContext(c))
}

func parseGroupIDs(c *gin.Context) (int, int, bool) {
	groupID, err := strconv.Atoi(c.Param("group_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid group id"})
		return 0, 0, false
	}
	msgID, err := strconv.Atoi(c.Param("message_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return 0, 0, false
	}
	return groupID, msgID, true
}
