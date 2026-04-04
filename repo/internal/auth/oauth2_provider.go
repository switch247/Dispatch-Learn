// Package auth provides OAuth2/OIDC integration for on-premises Identity Providers.
//
// This module is designed exclusively for on-prem IdP deployments (e.g., Keycloak,
// Okta on-prem, AD FS) and is gated behind the USE_OAUTH2 configuration flag.
// It must NOT be used with public cloud identity services in production without
// additional security review.
package auth

import (
	"fmt"
	"time"
)

// UserInfo represents the claims returned by the IdP's userinfo endpoint.
type UserInfo struct {
	Subject       string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Groups        []string `json:"groups,omitempty"`
}

// TokenResponse holds the tokens received after a successful code exchange.
type TokenResponse struct {
	AccessToken  string    `json:"access_token"`
	IDToken      string    `json:"id_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// OAuth2Provider defines the interface for interacting with an on-prem OIDC
// identity provider. Implementations must handle the full authorization-code
// flow: redirect URL generation, code-for-token exchange, and user profile
// retrieval.
type OAuth2Provider interface {
	// AuthorizeURL returns the URL the user should be redirected to in order
	// to begin the OIDC authorization-code flow. The state parameter must be
	// a cryptographically random value bound to the user's session.
	AuthorizeURL(state string) string

	// ExchangeCode trades an authorization code for an access/ID token pair.
	// It should validate the code and return the resulting tokens.
	ExchangeCode(code string) (*TokenResponse, error)

	// GetUserInfo uses an access token to retrieve the authenticated user's
	// profile claims from the IdP's userinfo endpoint.
	GetUserInfo(accessToken string) (*UserInfo, error)
}

// ---------------------------------------------------------------------------
// MockOAuth2Provider -- stub implementation for testing and local development
// ---------------------------------------------------------------------------

// MockOAuth2Provider is a fake OIDC provider that returns configurable,
// deterministic responses. It is intended for unit/integration tests and local
// development only. It must NEVER be enabled in production.
type MockOAuth2Provider struct {
	// IssuerURL is the pretend issuer. It is used when building the
	// authorize URL so callers can verify redirect logic.
	IssuerURL string

	// ClientID is echoed in the authorize URL query string.
	ClientID string

	// RedirectURL is the callback the authorize URL points back to.
	RedirectURL string

	// StubUser is the profile returned by GetUserInfo. Callers may
	// override individual fields before invoking the provider.
	StubUser UserInfo

	// StubTokenResponse is returned by ExchangeCode. If nil, a
	// reasonable default is generated.
	StubTokenResponse *TokenResponse

	// ExchangeCodeErr, if non-nil, is returned by ExchangeCode to
	// simulate IdP errors during the code exchange step.
	ExchangeCodeErr error

	// GetUserInfoErr, if non-nil, is returned by GetUserInfo to
	// simulate userinfo-endpoint failures.
	GetUserInfoErr error
}

// NewMockOAuth2Provider returns a MockOAuth2Provider pre-loaded with
// sensible default stub values.
func NewMockOAuth2Provider() *MockOAuth2Provider {
	return &MockOAuth2Provider{
		IssuerURL:   "https://idp.internal.example.com",
		ClientID:    "mock-client-id",
		RedirectURL: "http://localhost:8080/auth/callback",
		StubUser: UserInfo{
			Subject:       "mock-user-001",
			Email:         "testuser@internal.example.com",
			EmailVerified: true,
			Name:          "Test User",
			GivenName:     "Test",
			FamilyName:    "User",
			Groups:        []string{"engineers", "admins"},
		},
	}
}

// AuthorizeURL builds a fake authorization URL using the configured issuer,
// client ID, and redirect URL.
func (m *MockOAuth2Provider) AuthorizeURL(state string) string {
	return fmt.Sprintf(
		"%s/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=openid+profile+email&state=%s",
		m.IssuerURL, m.ClientID, m.RedirectURL, state,
	)
}

// ExchangeCode returns either a pre-configured error or a stubbed token
// response. The provided code value is ignored.
func (m *MockOAuth2Provider) ExchangeCode(code string) (*TokenResponse, error) {
	if m.ExchangeCodeErr != nil {
		return nil, m.ExchangeCodeErr
	}
	if m.StubTokenResponse != nil {
		return m.StubTokenResponse, nil
	}
	return &TokenResponse{
		AccessToken:  "mock-access-token",
		IDToken:      "mock-id-token",
		RefreshToken: "mock-refresh-token",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
	}, nil
}

// GetUserInfo returns either a pre-configured error or the stubbed user
// profile. The provided accessToken is ignored.
func (m *MockOAuth2Provider) GetUserInfo(accessToken string) (*UserInfo, error) {
	if m.GetUserInfoErr != nil {
		return nil, m.GetUserInfoErr
	}
	u := m.StubUser // copy so callers cannot mutate the original
	return &u, nil
}
