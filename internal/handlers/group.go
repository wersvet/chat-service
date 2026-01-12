package handlers

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel/trace"

	"chat-service/internal/models"
	"chat-service/internal/observability"
	"chat-service/internal/repositories"
	"chat-service/internal/ws"
)

// GroupHandler manages group-related endpoints.
type GroupHandler struct {
	groupRepo   repositories.GroupRepository
	messageRepo repositories.GroupMessageRepository
	userClient  userClient
	hub         *ws.Hub
}

// NewGroupHandler constructs a GroupHandler.
func NewGroupHandler(groupRepo repositories.GroupRepository, messageRepo repositories.GroupMessageRepository, userClient userClient, hub *ws.Hub) *GroupHandler {
	return &GroupHandler{
		groupRepo:   groupRepo,
		messageRepo: messageRepo,
		userClient:  userClient,
		hub:         hub,
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
		h.publishGroupCreateAudit(c, 0, nil, http.StatusBadRequest, false, err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate members exist via user-service
	if len(req.MemberIDs) > 0 {
		if _, err := h.userClient.BulkUsers(c.Request.Context(), req.MemberIDs); err != nil {
			h.publishGroupCreateAudit(c, 0, req.MemberIDs, http.StatusBadGateway, false, "failed to validate members")
			c.JSON(http.StatusBadGateway, gin.H{"error": "failed to validate members"})
			return
		}
	}

	group, err := h.groupRepo.CreateGroup(c.Request.Context(), userID, req.Name, req.MemberIDs)
	if err != nil {
		h.publishGroupCreateAudit(c, 0, req.MemberIDs, http.StatusInternalServerError, false, "could not create group")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create group"})
		return
	}

	h.publishGroupCreateAudit(c, group.ID, req.MemberIDs, http.StatusCreated, true, "")
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "membership check failed"})
		return
	}
	if !member {
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "membership check failed"})
		return
	}
	if !member {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}

	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := h.messageRepo.CreateGroupMessage(c.Request.Context(), groupID, userID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to store message"})
		return
	}

	h.hub.BroadcastGroupMessage(groupID, msg)
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
		c.JSON(http.StatusInternalServerError, gin.H{"error": "membership check failed"})
		return
	}
	if !member {
		c.JSON(http.StatusForbidden, gin.H{"error": "not a member"})
		return
	}

	msg, err := h.messageRepo.GetGroupMessage(c.Request.Context(), messageID)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrMessageNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "message not found"})
		return
	}
	if msg.GroupID != groupID {
		c.JSON(http.StatusBadRequest, gin.H{"error": "message does not belong to group"})
		return
	}
	if msg.SenderID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "only sender may delete"})
		return
	}

	if err := h.messageRepo.DeleteForAll(c.Request.Context(), messageID, userID); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, repositories.ErrMessageNotFound) {
			status = http.StatusNotFound
		}
		c.JSON(status, gin.H{"error": "could not delete"})
		return
	}

	h.hub.BroadcastGroupDeletion(groupID, messageID)
	c.Status(http.StatusNoContent)
}

func (h *GroupHandler) publishGroupCreateAudit(c *gin.Context, groupID int, memberIDs []int, status int, ok bool, errMsg string) {
	deviceID := observability.DeviceIDFromRequest(c.Request)
	requestID := observability.RequestIDFromRequest(c.Request)
	span := trace.SpanFromContext(c.Request.Context())
	traceID := span.SpanContext().TraceID().String()

	payload := map[string]interface{}{
		"actor": map[string]interface{}{
			"user_id":   c.GetInt("userID"),
			"device_id": deviceID,
		},
		"action": "groups.create",
		"target": map[string]interface{}{
			"group_id":   groupID,
			"member_ids": memberIDs,
		},
		"http": map[string]interface{}{
			"method": c.Request.Method,
			"path":   c.FullPath(),
			"status": status,
		},
		"result": map[string]interface{}{
			"ok":    ok,
			"error": errMsg,
		},
	}

	headers := observability.BuildHeaders(requestID, traceID)
	_ = observability.PublishEvent(c.Request.Context(), "audit_events.groups", observability.EventEnvelope{
		EventType: "audit_events",
		EventName: "groups.create",
		Payload:   payload,
	}, headers)
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
