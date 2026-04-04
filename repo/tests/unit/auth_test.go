package unit

import (
	"testing"
	"time"

	"dispatchlearn/config"
	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestJWTGeneration(t *testing.T) {
	cfg := &config.Config{
		JWT: config.JWTConfig{
			Secret:            "test-secret-key-for-unit-testing!",
			AccessExpiry:      30 * time.Minute,
			MaxActiveSessions: 10,
		},
	}

	t.Run("generate valid access token", func(t *testing.T) {
		claims := &domain.TokenClaims{
			UserID:   "user-123",
			TenantID: "tenant-456",
			Username: "testuser",
			Roles:    []string{"admin", "agent"},
		}

		token, err := middleware.GenerateAccessToken(cfg, claims)
		require.NoError(t, err)
		assert.NotEmpty(t, token)

		// Parse and validate
		parsed, err := jwt.ParseWithClaims(token, &middleware.Claims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWT.Secret), nil
		})
		require.NoError(t, err)
		assert.True(t, parsed.Valid)

		parsedClaims := parsed.Claims.(*middleware.Claims)
		assert.Equal(t, "user-123", parsedClaims.UserID)
		assert.Equal(t, "tenant-456", parsedClaims.TenantID)
		assert.Equal(t, "testuser", parsedClaims.Username)
		assert.Equal(t, []string{"admin", "agent"}, parsedClaims.Roles)
		assert.Equal(t, "dispatchlearn", parsedClaims.Issuer)
	})

	t.Run("token expires after configured duration", func(t *testing.T) {
		claims := &domain.TokenClaims{
			UserID:   "user-123",
			TenantID: "tenant-456",
			Username: "testuser",
			Roles:    []string{"agent"},
		}

		token, err := middleware.GenerateAccessToken(cfg, claims)
		require.NoError(t, err)

		parsed, err := jwt.ParseWithClaims(token, &middleware.Claims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte(cfg.JWT.Secret), nil
		})
		require.NoError(t, err)

		parsedClaims := parsed.Claims.(*middleware.Claims)
		expiresAt := parsedClaims.ExpiresAt.Time
		issuedAt := parsedClaims.IssuedAt.Time

		// Should expire ~30 minutes after issue
		diff := expiresAt.Sub(issuedAt)
		assert.InDelta(t, 30*60, diff.Seconds(), 5) // 5 second tolerance
	})

	t.Run("invalid secret rejects token", func(t *testing.T) {
		claims := &domain.TokenClaims{
			UserID:   "user-123",
			TenantID: "tenant-456",
			Username: "testuser",
			Roles:    []string{},
		}

		token, _ := middleware.GenerateAccessToken(cfg, claims)

		_, err := jwt.ParseWithClaims(token, &middleware.Claims{}, func(t *jwt.Token) (interface{}, error) {
			return []byte("wrong-secret"), nil
		})
		assert.Error(t, err)
	})
}
