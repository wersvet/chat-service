package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

const requestIDContextKey = "request_id"

func requestIDFromContext(c *gin.Context) string {
	if val, ok := c.Get(requestIDContextKey); ok {
		if id, ok := val.(string); ok && id != "" {
			return id
		}
	}

	requestID := c.GetHeader("X-Request-ID")
	if requestID == "" {
		requestID = uuid.NewString()
	}
	c.Set(requestIDContextKey, requestID)
	return requestID
}

func userIDFromContext(c *gin.Context) *string {
	if val, ok := c.Get("userID"); ok {
		if userID, ok := val.(int); ok && userID != 0 {
			text := strconv.Itoa(userID)
			return &text
		}
	}

	if header := c.GetHeader("X-User-ID"); header != "" {
		return &header
	}

	return nil
}
