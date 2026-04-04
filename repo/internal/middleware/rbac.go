package middleware

import (
	"net/http"

	"dispatchlearn/internal/domain"

	"github.com/gin-gonic/gin"
)

// RequireRole checks if the authenticated user has at least one of the specified roles
func RequireRole(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles := GetRoles(c)
		for _, ur := range userRoles {
			for _, required := range roles {
				if ur == required {
					c.Next()
					return
				}
			}
		}

		c.AbortWithStatusJSON(http.StatusForbidden, domain.APIResponse{
			Errors: []domain.APIError{{Code: "FORBIDDEN", Message: "insufficient permissions"}},
		})
	}
}

// RequirePermission checks if any of the user's roles have the specified permission
// This is a simplified check - in production, it would query the DB for role-permission mappings
func RequirePermission(resource, action string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRoles := GetRoles(c)
		// System admin has all permissions
		for _, r := range userRoles {
			if r == "system_admin" || r == "admin" {
				c.Next()
				return
			}
		}

		// For non-admin roles, check permission mapping from context or DB
		// Simplified: allow if user has any role (real impl queries role_permissions table)
		if len(userRoles) > 0 {
			c.Next()
			return
		}

		c.AbortWithStatusJSON(http.StatusForbidden, domain.APIResponse{
			Errors: []domain.APIError{{Code: "FORBIDDEN", Message: "missing permission: " + resource + ":" + action}},
		})
	}
}
