// Package auth provides OAuth2/OIDC integration for Identity Providers.
//
// OIDCProvider performs real HTTP calls to the provider's token and userinfo endpoints
// implementing the full Authorization Code Flow with state/nonce CSRF protection.
// MockOAuth2Provider is available for testing when USE_OAUTH2_MOCK=true.
package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// UserInfo represents OIDC standard claims from the userinfo endpoint.
type UserInfo struct {
	Subject       string   `json:"sub"`
	Email         string   `json:"email"`
	EmailVerified bool     `json:"email_verified"`
	Name          string   `json:"name"`
	GivenName     string   `json:"given_name"`
	FamilyName    string   `json:"family_name"`
	Groups        []string `json:"groups,omitempty"`
}

// TokenResponse holds the tokens received from the token endpoint.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	IDToken      string `json:"id_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

// OAuth2Provider defines the interface for OIDC identity providers.
type OAuth2Provider interface {
	AuthorizeURL(state, nonce string) string
	ExchangeCode(code string) (*TokenResponse, error)
	GetUserInfo(accessToken string) (*UserInfo, error)
}

// ---------------------------------------------------------------------------
// OIDCProvider — production implementation using real HTTP calls
// ---------------------------------------------------------------------------

// OIDCConfig holds the configuration for an OIDC provider (Keycloak, Okta, etc.).
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// OIDCProvider performs real Authorization Code Flow over HTTP.
type OIDCProvider struct {
	cfg        OIDCConfig
	httpClient *http.Client
}

func NewOIDCProvider(cfg OIDCConfig) *OIDCProvider {
	return &OIDCProvider{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// AuthorizeURL builds the OIDC authorization endpoint URL with state and nonce.
func (p *OIDCProvider) AuthorizeURL(state, nonce string) string {
	params := url.Values{
		"client_id":     {p.cfg.ClientID},
		"redirect_uri":  {p.cfg.RedirectURL},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {state},
		"nonce":         {nonce},
	}
	return fmt.Sprintf("%s/protocol/openid-connect/auth?%s", p.cfg.IssuerURL, params.Encode())
}

// ExchangeCode exchanges an authorization code for tokens via HTTP POST to the token endpoint.
func (p *OIDCProvider) ExchangeCode(code string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("%s/protocol/openid-connect/token", p.cfg.IssuerURL)

	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {p.cfg.RedirectURL},
		"client_id":     {p.cfg.ClientID},
		"client_secret": {p.cfg.ClientSecret},
	}

	resp, err := p.httpClient.Post(tokenURL, "application/x-www-form-urlencoded", strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("token exchange HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("failed to parse token response JSON: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, errors.New("token response missing access_token")
	}

	return &tokenResp, nil
}

// GetUserInfo fetches user claims from the OIDC userinfo endpoint.
func (p *OIDCProvider) GetUserInfo(accessToken string) (*UserInfo, error) {
	userInfoURL := fmt.Sprintf("%s/protocol/openid-connect/userinfo", p.cfg.IssuerURL)

	req, err := http.NewRequest("GET", userInfoURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("userinfo HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read userinfo response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("userinfo endpoint returned HTTP %d: %s", resp.StatusCode, string(body))
	}

	var userInfo UserInfo
	if err := json.Unmarshal(body, &userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse userinfo JSON: %w", err)
	}

	if userInfo.Email == "" && userInfo.Subject == "" {
		return nil, errors.New("userinfo response missing both email and sub claims")
	}

	return &userInfo, nil
}

// ---------------------------------------------------------------------------
// MockOAuth2Provider — for testing and local development only
// ---------------------------------------------------------------------------

// MockOAuth2Provider returns deterministic responses without making HTTP calls.
// Controlled by USE_OAUTH2_MOCK env var. Never use in production.
type MockOAuth2Provider struct {
	IssuerURL         string
	ClientID          string
	RedirectURL       string
	StubUser          UserInfo
	StubTokenResponse *TokenResponse
	ExchangeCodeErr   error
	GetUserInfoErr    error
}

func NewMockOAuth2Provider() *MockOAuth2Provider {
	return &MockOAuth2Provider{
		IssuerURL:   "https://idp.internal.example.com",
		ClientID:    "mock-client-id",
		RedirectURL: "http://localhost:8080/api/v1/auth/oauth2/callback",
		StubUser: UserInfo{
			Subject:       "mock-user-001",
			Email:         "testuser@internal.example.com",
			EmailVerified: true,
			Name:          "Test User",
			GivenName:     "Test",
			FamilyName:    "User",
			Groups:        []string{"engineers"},
		},
	}
}

func (m *MockOAuth2Provider) AuthorizeURL(state, nonce string) string {
	return fmt.Sprintf("%s/authorize?client_id=%s&redirect_uri=%s&response_type=code&scope=openid+profile+email&state=%s&nonce=%s",
		m.IssuerURL, m.ClientID, m.RedirectURL, state, nonce)
}

func (m *MockOAuth2Provider) ExchangeCode(code string) (*TokenResponse, error) {
	if m.ExchangeCodeErr != nil {
		return nil, m.ExchangeCodeErr
	}
	if m.StubTokenResponse != nil {
		return m.StubTokenResponse, nil
	}
	return &TokenResponse{
		AccessToken:  "mock-access-token-" + code,
		IDToken:      "mock-id-token",
		RefreshToken: "mock-refresh-token",
		TokenType:    "Bearer",
		ExpiresIn:    3600,
	}, nil
}

func (m *MockOAuth2Provider) GetUserInfo(accessToken string) (*UserInfo, error) {
	if m.GetUserInfoErr != nil {
		return nil, m.GetUserInfoErr
	}
	u := m.StubUser
	return &u, nil
}
