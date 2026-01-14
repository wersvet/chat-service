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

func userIDFromContext(c *gin.Context) *int64 {
	if val, ok := c.Get("userID"); ok {
		switch userID := val.(type) {
		case int:
			if userID != 0 {
				value := int64(userID)
				return &value
			}
		case int64:
			if userID != 0 {
				value := userID
				return &value
			}
		}
	}

	if header := c.GetHeader("X-User-ID"); header != "" {
		if parsed, err := strconv.ParseInt(header, 10, 64); err == nil {
			return &parsed
		}
	}

	return nil
}
