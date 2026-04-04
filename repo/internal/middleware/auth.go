package middleware

import (
	"net/http"
	"strings"
	"time"

	"dispatchlearn/config"
	"dispatchlearn/internal/domain"
	"dispatchlearn/logging"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   string   `json:"user_id"`
	TenantID string   `json:"tenant_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
	jwt.RegisteredClaims
}

func GenerateAccessToken(cfg *config.Config, claims *domain.TokenClaims) (string, error) {
	now := time.Now()
	tokenClaims := Claims{
		UserID:   claims.UserID,
		TenantID: claims.TenantID,
		Username: claims.Username,
		Roles:    claims.Roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.JWT.AccessExpiry)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "dispatchlearn",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, tokenClaims)
	return token.SignedString([]byte(cfg.JWT.Secret))
}

func AuthMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.APIResponse{
				Errors: []domain.APIError{{Code: "UNAUTHORIZED", Message: "missing authorization header"}},
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.APIResponse{
				Errors: []domain.APIError{{Code: "UNAUTHORIZED", Message: "invalid authorization format"}},
			})
			return
		}

		claims := &Claims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWT.Secret), nil
		})

		if err != nil || !token.Valid {
			logging.Warn("auth", "jwt", "invalid token presented")
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.APIResponse{
				Errors: []domain.APIError{{Code: "UNAUTHORIZED", Message: "invalid or expired token"}},
			})
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("tenant_id", claims.TenantID)
		c.Set("username", claims.Username)
		c.Set("roles", claims.Roles)

		c.Next()
	}
}

func GetUserID(c *gin.Context) string {
	if v, exists := c.Get("user_id"); exists {
		return v.(string)
	}
	return ""
}

func GetTenantID(c *gin.Context) string {
	if v, exists := c.Get("tenant_id"); exists {
		return v.(string)
	}
	return ""
}

func GetRoles(c *gin.Context) []string {
	if v, exists := c.Get("roles"); exists {
		return v.([]string)
	}
	return nil
}
