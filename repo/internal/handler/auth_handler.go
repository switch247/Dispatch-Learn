package handler

import (
	"crypto/rand"
	"encoding/hex"
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

func (h *AuthHandler) OAuth2Login(c *gin.Context) {
	if !h.cfg.OAuth2.Enabled {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "OAuth2 not enabled")
		return
	}

	provider := auth.NewMockOAuth2Provider()

	// Generate a random state parameter
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", "failed to generate state")
		return
	}
	state := hex.EncodeToString(stateBytes)

	respondOK(c, gin.H{"authorize_url": provider.AuthorizeURL(state)})
}

func (h *AuthHandler) OAuth2Callback(c *gin.Context) {
	if !h.cfg.OAuth2.Enabled {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "OAuth2 not enabled")
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	provider := auth.NewMockOAuth2Provider()

	// Exchange authorization code for token
	tokenResp, err := provider.ExchangeCode(req.Code)
	if err != nil {
		respondError(c, http.StatusBadRequest, "OAUTH2_EXCHANGE_FAILED", err.Error())
		return
	}

	// Get user info from provider
	userInfo, err := provider.GetUserInfo(tokenResp.AccessToken)
	if err != nil {
		respondError(c, http.StatusBadRequest, "OAUTH2_USERINFO_FAILED", err.Error())
		return
	}

	// Use the default tenant for OAuth2 users
	tenantID := "00000000-0000-0000-0000-000000000001"

	// Try to find existing user or register a new one
	loginReq := &domain.LoginRequest{
		Username: userInfo.Email,
		Password: "oauth2-" + userInfo.Subject, // deterministic password for mock OAuth2 users
		TenantID: tenantID,
	}

	// Attempt to register (will fail if user already exists, which is fine)
	_, _ = h.uc.Register(tenantID, loginReq)

	// Now login to get JWT
	resp, err := h.uc.Login(loginReq, c.ClientIP(), c.GetHeader("User-Agent"))
	if err != nil {
		respondError(c, http.StatusInternalServerError, "OAUTH2_LOGIN_FAILED", err.Error())
		return
	}

	respondOK(c, resp)
}
