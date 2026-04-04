package middleware

import (
	"fmt"
	"time"

	"dispatchlearn/logging"

	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		tenantID := GetTenantID(c)
		userID := GetUserID(c)

		msg := fmt.Sprintf("%s %s -> %d (%v) tenant=%s user=%s",
			method, path, status, latency, tenantID, userID)

		if status >= 500 {
			logging.Error("http", "request", msg)
		} else if status >= 400 {
			logging.Warn("http", "request", msg)
		} else {
			logging.Info("http", "request", msg)
		}
	}
}
