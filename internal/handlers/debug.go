package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"chat-service/internal/telemetry"
)

// RegisterDebugRoutes wires debug-only endpoints.
func RegisterDebugRoutes(router *gin.Engine, emitter *telemetry.AuditEmitter, enabled bool) {
	if !enabled {
		return
	}

	router.GET("/debug/audit-test", func(c *gin.Context) {
		if emitter == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "audit emitter not configured"})
			return
		}
		emitter.Emit(c.Request.Context(), "INFO", "audit test", requestIDFromContext(c), userIDFromContext(c))
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})
}
