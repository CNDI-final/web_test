package WebUI

import (
	"time"

	"github.com/gin-gonic/gin"

	"web_test/internal/logger"
)

// GinLogger returns a gin middleware that logs requests via backend/logger.MainLog
func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		status := c.Writer.Status()

		logger.WebLog.WithFields(map[string]interface{}{
			"method":  c.Request.Method,
			"path":    c.Request.URL.Path,
			"status":  status,
			"latency": latency.String(),
			"client":  c.ClientIP(),
		}).Info("http")
	}
}
