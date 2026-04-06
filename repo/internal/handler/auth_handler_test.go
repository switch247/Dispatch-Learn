package handler

import (
	"testing"

	"dispatchlearn/config"
	"dispatchlearn/internal/auth"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetOAuth2Provider(t *testing.T) {
	t.Run("uses mock provider when mock mode is enabled", func(t *testing.T) {
		h := NewAuthHandler(nil, &config.Config{
			OAuth2: config.OAuth2Config{MockMode: true},
		})

		provider, err := h.getOAuth2Provider()
		require.NoError(t, err)
		require.NotNil(t, provider)

		_, ok := provider.(*auth.MockOAuth2Provider)
		assert.True(t, ok)
	})

	t.Run("fails closed when issuer URL is missing in non-mock mode", func(t *testing.T) {
		h := NewAuthHandler(nil, &config.Config{
			OAuth2: config.OAuth2Config{MockMode: false, IssuerURL: ""},
		})

		provider, err := h.getOAuth2Provider()
		require.Error(t, err)
		assert.Nil(t, provider)
		assert.Contains(t, err.Error(), "OAUTH2_ISSUER_URL")
	})

	t.Run("uses real OIDC provider when issuer URL is configured", func(t *testing.T) {
		h := NewAuthHandler(nil, &config.Config{
			OAuth2: config.OAuth2Config{
				MockMode:     false,
				IssuerURL:    "https://idp.example.com",
				ClientID:     "client-123",
				ClientSecret: "secret-123",
				RedirectURL:  "https://app.example.com/oauth2/callback",
			},
		})

		provider, err := h.getOAuth2Provider()
		require.NoError(t, err)
		require.NotNil(t, provider)

		oidc, ok := provider.(*auth.OIDCProvider)
		require.True(t, ok)
		authorizeURL := oidc.AuthorizeURL("state-1", "nonce-1")
		assert.Contains(t, authorizeURL, "https://idp.example.com/protocol/openid-connect/auth?")
		assert.Contains(t, authorizeURL, "client_id=client-123")
		assert.Contains(t, authorizeURL, "state=state-1")
		assert.Contains(t, authorizeURL, "nonce=nonce-1")
	})
}
