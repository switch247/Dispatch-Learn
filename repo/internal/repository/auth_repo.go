package repository

import (
	"errors"
	"time"

	"dispatchlearn/internal/domain"

	"gorm.io/gorm"
)

type AuthRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) *AuthRepository {
	return &AuthRepository{db: db}
}

func (r *AuthRepository) CreateUser(user *domain.User) error {
	return r.db.Create(user).Error
}

func (r *AuthRepository) FindByUsername(tenantID, username string) (*domain.User, error) {
	var user domain.User
	err := r.db.Preload("Roles").
		Where("tenant_id = ? AND username = ?", tenantID, username).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) FindUserByID(tenantID, id string) (*domain.User, error) {
	var user domain.User
	err := r.db.Preload("Roles").
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *AuthRepository) UpdateUser(user *domain.User) error {
	return r.db.Save(user).Error
}

func (r *AuthRepository) ListUsers(tenantID string, page, perPage int) ([]domain.User, int64, error) {
	var users []domain.User
	var total int64

	r.db.Model(&domain.User{}).Where("tenant_id = ?", tenantID).Count(&total)

	err := r.db.Preload("Roles").
		Where("tenant_id = ?", tenantID).
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&users).Error
	return users, total, err
}

func (r *AuthRepository) IncrementFailedAttempts(userID string) error {
	return r.db.Model(&domain.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{
			"failed_attempt_count": gorm.Expr("failed_attempt_count + 1"),
		}).Error
}

func (r *AuthRepository) LockUser(userID string, until time.Time) error {
	return r.db.Model(&domain.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{
			"locked_until": until,
		}).Error
}

func (r *AuthRepository) ResetFailedAttempts(userID string) error {
	now := time.Now()
	return r.db.Model(&domain.User{}).Where("id = ?", userID).
		Updates(map[string]interface{}{
			"failed_attempt_count": 0,
			"locked_until":         nil,
			"last_login_at":        &now,
		}).Error
}

// Sessions
func (r *AuthRepository) CreateSession(session *domain.UserSession) error {
	return r.db.Create(session).Error
}

func (r *AuthRepository) CountActiveSessions(tenantID, userID string) (int64, error) {
	var count int64
	err := r.db.Model(&domain.UserSession{}).
		Where("tenant_id = ? AND user_id = ? AND is_active = ? AND expires_at > ?",
			tenantID, userID, true, time.Now()).
		Count(&count).Error
	return count, err
}

func (r *AuthRepository) FindSessionByRefreshToken(token string) (*domain.UserSession, error) {
	var session domain.UserSession
	err := r.db.Where("refresh_token = ? AND is_active = ? AND expires_at > ?",
		token, true, time.Now()).
		First(&session).Error
	if err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *AuthRepository) RevokeSession(sessionID string) error {
	now := time.Now()
	return r.db.Model(&domain.UserSession{}).Where("id = ?", sessionID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"revoked_at": &now,
		}).Error
}

// RevokeSessionOwned enforces object-level authorization: only the session owner can revoke.
// Returns number of rows affected so caller can detect unauthorized attempts.
func (r *AuthRepository) RevokeSessionOwned(tenantID, userID, sessionID string) (int64, error) {
	now := time.Now()
	result := r.db.Model(&domain.UserSession{}).
		Where("id = ? AND user_id = ? AND tenant_id = ?", sessionID, userID, tenantID).
		Updates(map[string]interface{}{
			"is_active":  false,
			"revoked_at": &now,
		})
	return result.RowsAffected, result.Error
}

func (r *AuthRepository) RevokeOldestSession(tenantID, userID string) error {
	var oldest domain.UserSession
	err := r.db.Where("tenant_id = ? AND user_id = ? AND is_active = ?", tenantID, userID, true).
		Order("created_at ASC").First(&oldest).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return err
	}
	return r.RevokeSession(oldest.ID)
}

func (r *AuthRepository) ListUserSessions(tenantID, userID string) ([]domain.UserSession, error) {
	var sessions []domain.UserSession
	err := r.db.Where("tenant_id = ? AND user_id = ? AND is_active = ?",
		tenantID, userID, true).
		Order("created_at DESC").
		Find(&sessions).Error
	return sessions, err
}

// Roles & Permissions
func (r *AuthRepository) CreateRole(role *domain.Role) error {
	return r.db.Create(role).Error
}

func (r *AuthRepository) FindRoleByName(tenantID, name string) (*domain.Role, error) {
	var role domain.Role
	err := r.db.Preload("Permissions").
		Where("tenant_id = ? AND name = ?", tenantID, name).
		First(&role).Error
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *AuthRepository) ListRoles(tenantID string) ([]domain.Role, error) {
	var roles []domain.Role
	err := r.db.Preload("Permissions").
		Where("tenant_id = ?", tenantID).
		Find(&roles).Error
	return roles, err
}

func (r *AuthRepository) AssignRole(tenantID, userID, roleID string) error {
	ur := domain.UserRole{UserID: userID, RoleID: roleID, TenantID: tenantID}
	return r.db.Create(&ur).Error
}

func (r *AuthRepository) RemoveRole(tenantID, userID, roleID string) error {
	return r.db.Where("tenant_id = ? AND user_id = ? AND role_id = ?",
		tenantID, userID, roleID).Delete(&domain.UserRole{}).Error
}

func (r *AuthRepository) CreatePermission(perm *domain.Permission) error {
	return r.db.Create(perm).Error
}

// Tenants
func (r *AuthRepository) CreateTenant(tenant *domain.Tenant) error {
	return r.db.Create(tenant).Error
}

func (r *AuthRepository) FindTenantByID(id string) (*domain.Tenant, error) {
	var tenant domain.Tenant
	err := r.db.Where("id = ?", id).First(&tenant).Error
	if err != nil {
		return nil, err
	}
	return &tenant, nil
}
