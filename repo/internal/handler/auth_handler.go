package handler

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"dispatchlearn/config"
	"dispatchlearn/internal/auth"
	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/middleware"
	"dispatchlearn/internal/usecase"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	uc  *usecase.AuthUseCase
	cfg *config.Config
}

func NewAuthHandler(uc *usecase.AuthUseCase, cfg *config.Config) *AuthHandler {
	return &AuthHandler{uc: uc, cfg: cfg}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	user, err := h.uc.Register(req.TenantID, &req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "REGISTER_FAILED", err.Error())
		return
	}

	respondCreated(c, user)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	resp, err := h.uc.Login(&req, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		respondError(c, http.StatusUnauthorized, "LOGIN_FAILED", err.Error())
		return
	}

	respondOK(c, resp)
}

func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req domain.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	resp, err := h.uc.RefreshToken(&req, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		respondError(c, http.StatusUnauthorized, "REFRESH_FAILED", err.Error())
		return
	}

	respondOK(c, resp)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := middleware.GetUserID(c)
	sessionID := c.Param("session_id")

	if err := h.uc.Logout(tenantID, userID, sessionID); err != nil {
		if strings.Contains(err.Error(), "FORBIDDEN") {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "session not found or not owned by user")
			return
		}
		respondError(c, http.StatusBadRequest, "LOGOUT_FAILED", err.Error())
		return
	}

	respondOK(c, gin.H{"message": "session revoked"})
}

func (h *AuthHandler) GetMe(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := middleware.GetUserID(c)

	user, err := h.uc.GetUser(tenantID, userID)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	respondOK(c, user)
}

func (h *AuthHandler) ListUsers(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	page, perPage := getPagination(c)

	users, total, err := h.uc.ListUsers(tenantID, page, perPage)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondList(c, users, page, perPage, total)
}

func (h *AuthHandler) GetUser(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := c.Param("id")

	user, err := h.uc.GetUser(tenantID, userID)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "user not found")
		return
	}

	respondOK(c, user)
}

func (h *AuthHandler) ListSessions(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := middleware.GetUserID(c)

	sessions, err := h.uc.ListSessions(tenantID, userID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondOK(c, sessions)
}

func (h *AuthHandler) RevokeSession(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := middleware.GetUserID(c)
	sessionID := c.Param("session_id")

	if err := h.uc.RevokeSession(tenantID, userID, sessionID); err != nil {
		if strings.Contains(err.Error(), "FORBIDDEN") {
			respondError(c, http.StatusForbidden, "FORBIDDEN", "session not found or not owned by user")
			return
		}
		respondError(c, http.StatusBadRequest, "REVOKE_FAILED", err.Error())
		return
	}

	respondOK(c, gin.H{"message": "session revoked"})
}

func (h *AuthHandler) AssignRole(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)
	userID := c.Param("id")

	var req struct {
		Role string `json:"role" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	if err := h.uc.AssignRole(tenantID, actorID, userID, req.Role); err != nil {
		if strings.Contains(err.Error(), "FORBIDDEN") {
			respondError(c, http.StatusForbidden, "ESCALATION_DENIED", err.Error())
			return
		}
		respondError(c, http.StatusBadRequest, "ASSIGN_FAILED", err.Error())
		return
	}

	respondOK(c, gin.H{"message": "role assigned"})
}

func (h *AuthHandler) ListRoles(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	roles, err := h.uc.ListRoles(tenantID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondOK(c, roles)
}

// getOAuth2Provider returns the appropriate OIDC provider based on config.
// Returns error if OAuth2 is enabled but misconfigured (no auto-fallback to mock).
func (h *AuthHandler) getOAuth2Provider() (auth.OAuth2Provider, error) {
	// Explicit mock mode: USE_OAUTH2_MOCK=true (for testing/dev only)
	if h.cfg.OAuth2.MockMode {
		return auth.NewMockOAuth2Provider(), nil
	}

	// Production: require real IssuerURL
	if h.cfg.OAuth2.IssuerURL == "" {
		return nil, fmt.Errorf("OAuth2 is enabled but OAUTH2_ISSUER_URL is not configured. Set USE_OAUTH2_MOCK=true for testing")
	}

	return auth.NewOIDCProvider(auth.OIDCConfig{
		IssuerURL:    h.cfg.OAuth2.IssuerURL,
		ClientID:     h.cfg.OAuth2.ClientID,
		ClientSecret: h.cfg.OAuth2.ClientSecret,
		RedirectURL:  h.cfg.OAuth2.RedirectURL,
	}), nil
}

func (h *AuthHandler) OAuth2Login(c *gin.Context) {
	if !h.cfg.OAuth2.Enabled {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "OAuth2 not enabled")
		return
	}

	provider, err := h.getOAuth2Provider()
	if err != nil {
		respondError(c, http.StatusInternalServerError, "OAUTH2_CONFIG_ERROR", err.Error())
		return
	}

	// Generate cryptographic state (CSRF protection) and nonce (replay protection)
	stateBytes := make([]byte, 16)
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", "failed to generate state")
		return
	}
	if _, err := rand.Read(nonceBytes); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", "failed to generate nonce")
		return
	}
	state := hex.EncodeToString(stateBytes)
	nonce := hex.EncodeToString(nonceBytes)

	// Set state in a secure, HttpOnly cookie for CSRF validation
	secure := h.cfg.TLS.Enabled
	c.SetCookie("oauth2_state", state, 600, "/", "", secure, true)
	c.SetCookie("oauth2_nonce", nonce, 600, "/", "", secure, true)

	respondOK(c, gin.H{
		"authorize_url": provider.AuthorizeURL(state, nonce),
		"state":         state,
	})
}

func (h *AuthHandler) OAuth2Callback(c *gin.Context) {
	if !h.cfg.OAuth2.Enabled {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "OAuth2 not enabled")
		return
	}

	var req struct {
		Code  string `json:"code" binding:"required"`
		State string `json:"state" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	// Validate state parameter against cookie to prevent CSRF
	savedState, err := c.Cookie("oauth2_state")
	if err != nil || savedState != req.State {
		respondError(c, http.StatusBadRequest, "OAUTH2_CSRF", "state parameter mismatch — possible CSRF attack")
		return
	}
	// Clear the state cookies
	secure := h.cfg.TLS.Enabled
	c.SetCookie("oauth2_state", "", -1, "/", "", secure, true)
	c.SetCookie("oauth2_nonce", "", -1, "/", "", secure, true)

	provider, provErr := h.getOAuth2Provider()
	if provErr != nil {
		respondError(c, http.StatusInternalServerError, "OAUTH2_CONFIG_ERROR", provErr.Error())
		return
	}

	// Exchange authorization code for tokens (real HTTP call to provider)
	tokenResp, err := provider.ExchangeCode(req.Code)
	if err != nil {
		respondError(c, http.StatusBadRequest, "OAUTH2_EXCHANGE_FAILED", err.Error())
		return
	}

	// Fetch user info from provider (real HTTP call to userinfo endpoint)
	userInfo, err := provider.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		respondError(c, http.StatusBadRequest, "OAUTH2_USERINFO_FAILED", err.Error())
		return
	}

	// Map OIDC user to local system
	tenantID := "00000000-0000-0000-0000-000000000001"
	loginReq := &domain.LoginRequest{
		Username: userInfo.Email,
		Password: "oidc-" + userInfo.Subject,
		TenantID: tenantID,
	}

	// Auto-provision: register if user doesn't exist (idempotent)
	_, _ = h.uc.Register(tenantID, loginReq)

	// Issue local JWT session
	resp, err := h.uc.Login(loginReq, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "OAUTH2_LOGIN_FAILED", err.Error())
		return
	}

	respondOK(c, resp)
}
