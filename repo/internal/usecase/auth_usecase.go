package usecase

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"dispatchlearn/config"
	"dispatchlearn/internal/audit"
	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/middleware"
	"dispatchlearn/internal/repository"
	"dispatchlearn/logging"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthUseCase struct {
	repo     *repository.AuthRepository
	audit    *audit.Service
	cfg      *config.Config
}

func NewAuthUseCase(repo *repository.AuthRepository, audit *audit.Service, cfg *config.Config) *AuthUseCase {
	return &AuthUseCase{repo: repo, audit: audit, cfg: cfg}
}

func (uc *AuthUseCase) Register(tenantID string, req *domain.LoginRequest) (*domain.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &domain.User{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		Username:     req.Username,
		PasswordHash: string(hash),
		IsActive:     true,
	}

	if err := uc.repo.CreateUser(user); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    user.ID,
		Action:     "user.created",
		EntityType: "user",
		EntityID:   user.ID,
		AfterState: map[string]string{"username": user.Username},
	})

	return user, nil
}

func (uc *AuthUseCase) Login(req *domain.LoginRequest, ipAddress, userAgent string) (*domain.LoginResponse, error) {
	user, err := uc.repo.FindByUsername(req.TenantID, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("invalid credentials")
		}
		return nil, err
	}

	// Check lockout
	if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
		logging.Warn("auth", "login", "account locked: "+user.Username)
		return nil, errors.New("account locked, try again later")
	}

	if !user.IsActive {
		return nil, errors.New("account disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		uc.repo.IncrementFailedAttempts(user.ID)
		if user.FailedAttemptCount+1 >= 5 {
			lockUntil := time.Now().Add(30 * time.Minute)
			uc.repo.LockUser(user.ID, lockUntil)
			logging.Warn("auth", "login", "account locked after 5 failed attempts: "+user.Username)
		}
		return nil, errors.New("invalid credentials")
	}

	// Reset failed attempts
	uc.repo.ResetFailedAttempts(user.ID)

	// Check session cap
	count, _ := uc.repo.CountActiveSessions(user.TenantID, user.ID)
	if count >= int64(uc.cfg.JWT.MaxActiveSessions) {
		uc.repo.RevokeOldestSession(user.TenantID, user.ID)
	}

	// Generate tokens
	roleNames := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roleNames[i] = r.Name
	}

	claims := &domain.TokenClaims{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Username: user.Username,
		Roles:    roleNames,
	}

	accessToken, err := middleware.GenerateAccessToken(uc.cfg, claims)
	if err != nil {
		return nil, err
	}

	refreshToken := generateRefreshToken()

	session := &domain.UserSession{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: user.TenantID,
		},
		UserID:       user.ID,
		RefreshToken: refreshToken,
		UserAgent:    userAgent,
		IPAddress:    ipAddress,
		ExpiresAt:    time.Now().Add(uc.cfg.JWT.RefreshExpiry),
		IsActive:     true,
	}
	uc.repo.CreateSession(session)

	uc.audit.Log(audit.LogEntry{
		TenantID:   user.TenantID,
		ActorID:    user.ID,
		Action:     "user.login",
		EntityType: "user",
		EntityID:   user.ID,
		IPAddress:  ipAddress,
	})

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int(uc.cfg.JWT.AccessExpiry.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

func (uc *AuthUseCase) RefreshToken(req *domain.RefreshRequest, ipAddress, userAgent string) (*domain.LoginResponse, error) {
	session, err := uc.repo.FindSessionByRefreshToken(req.RefreshToken)
	if err != nil {
		return nil, errors.New("invalid refresh token")
	}

	// Revoke old session (rotating refresh)
	uc.repo.RevokeSession(session.ID)

	user, err := uc.repo.FindUserByID(session.TenantID, session.UserID)
	if err != nil {
		return nil, err
	}

	roleNames := make([]string, len(user.Roles))
	for i, r := range user.Roles {
		roleNames[i] = r.Name
	}

	claims := &domain.TokenClaims{
		UserID:   user.ID,
		TenantID: user.TenantID,
		Username: user.Username,
		Roles:    roleNames,
	}

	accessToken, err := middleware.GenerateAccessToken(uc.cfg, claims)
	if err != nil {
		return nil, err
	}

	newRefreshToken := generateRefreshToken()
	newSession := &domain.UserSession{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: session.TenantID,
		},
		UserID:       user.ID,
		RefreshToken: newRefreshToken,
		UserAgent:    userAgent,
		IPAddress:    ipAddress,
		ExpiresAt:    time.Now().Add(uc.cfg.JWT.RefreshExpiry),
		IsActive:     true,
	}
	uc.repo.CreateSession(newSession)

	return &domain.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int(uc.cfg.JWT.AccessExpiry.Seconds()),
		TokenType:    "Bearer",
	}, nil
}

func (uc *AuthUseCase) Logout(tenantID, userID, sessionID string) error {
	rows, err := uc.repo.RevokeSessionOwned(tenantID, userID, sessionID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("FORBIDDEN: session not found or not owned by user")
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    userID,
		Action:     "user.logout",
		EntityType: "session",
		EntityID:   sessionID,
	})

	return nil
}

func (uc *AuthUseCase) ListSessions(tenantID, userID string) ([]domain.UserSession, error) {
	return uc.repo.ListUserSessions(tenantID, userID)
}

func (uc *AuthUseCase) RevokeSession(tenantID, userID, sessionID string) error {
	rows, err := uc.repo.RevokeSessionOwned(tenantID, userID, sessionID)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("FORBIDDEN: session not found or not owned by user")
	}
	return nil
}

func (uc *AuthUseCase) GetUser(tenantID, userID string) (*domain.User, error) {
	return uc.repo.FindUserByID(tenantID, userID)
}

func (uc *AuthUseCase) ListUsers(tenantID string, page, perPage int) ([]domain.User, int64, error) {
	return uc.repo.ListUsers(tenantID, page, perPage)
}

// Role hierarchy for escalation prevention:
//   system_admin > admin > dispatcher, instructor, finance, agent
// Rules:
//   - system_admin can assign any role
//   - admin can assign: dispatcher, agent, instructor, finance (NOT admin or system_admin)
//   - no other role can assign roles
var roleAssignmentMatrix = map[string]map[string]bool{
	"system_admin": {"system_admin": true, "admin": true, "dispatcher": true, "agent": true, "instructor": true, "finance": true},
	"admin":        {"dispatcher": true, "agent": true, "instructor": true, "finance": true},
}

func (uc *AuthUseCase) AssignRole(tenantID, actorID, userID, roleName string) error {
	// Prevent self-role modification
	if actorID == userID {
		return errors.New("FORBIDDEN: users cannot modify their own roles")
	}

	// Fetch actor to check their roles for escalation prevention
	actor, err := uc.repo.FindUserByID(tenantID, actorID)
	if err != nil {
		return errors.New("actor not found")
	}

	// Check escalation: actor must have permission to assign the target role
	allowed := false
	for _, actorRole := range actor.Roles {
		if perms, ok := roleAssignmentMatrix[actorRole.Name]; ok {
			if perms[roleName] {
				allowed = true
				break
			}
		}
	}
	if !allowed {
		logging.Warn("auth", "escalation", "role escalation denied: actor="+actorID+" tried to assign role="+roleName)
		return errors.New("FORBIDDEN: insufficient privileges to assign role '" + roleName + "' — escalation denied")
	}

	role, err := uc.repo.FindRoleByName(tenantID, roleName)
	if err != nil {
		return errors.New("role not found")
	}

	if err := uc.repo.AssignRole(tenantID, userID, role.ID); err != nil {
		return err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "role.assigned",
		EntityType: "user",
		EntityID:   userID,
		AfterState: map[string]string{"role": roleName, "assigned_by": actorID},
	})

	return nil
}

func (uc *AuthUseCase) ListRoles(tenantID string) ([]domain.Role, error) {
	return uc.repo.ListRoles(tenantID)
}

func generateRefreshToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}
