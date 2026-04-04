package middleware

import (
	"net/http"

	"dispatchlearn/internal/domain"
	"dispatchlearn/logging"

	"github.com/gin-gonic/gin"
)

// TenantIsolation enforces that all requests are scoped to the authenticated user's tenant
func TenantIsolation() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := GetTenantID(c)
		if tenantID == "" {
			logging.Warn("middleware", "tenant", "request without tenant_id")
			c.AbortWithStatusJSON(http.StatusForbidden, domain.APIResponse{
				Errors: []domain.APIError{{Code: "FORBIDDEN", Message: "tenant context required"}},
			})
			return
		}
		c.Next()
	}
}
