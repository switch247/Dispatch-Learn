package domain

import "time"

type User struct {
	BaseModel
	Username           string    `gorm:"type:varchar(255);uniqueIndex:idx_user_tenant,priority:2;not null" json:"username"`
	PasswordHash       string    `gorm:"type:varchar(255);not null" json:"-"`
	Email              string    `gorm:"type:varchar(255)" json:"email"`
	FullName           string    `gorm:"type:varchar(255)" json:"full_name"`
	IsActive           bool      `gorm:"default:true" json:"is_active"`
	LastLoginAt        *time.Time `json:"last_login_at"`
	FailedAttemptCount int       `gorm:"default:0" json:"-"`
	LockedUntil        *time.Time `json:"-"`
	Roles              []Role    `gorm:"many2many:user_roles;" json:"roles,omitempty"`
}

func (User) TableName() string { return "users" }

type Role struct {
	BaseModel
	Name        string       `gorm:"type:varchar(100);uniqueIndex:idx_role_tenant,priority:2;not null" json:"name"`
	Description string       `gorm:"type:varchar(500)" json:"description"`
	Permissions []Permission `gorm:"many2many:role_permissions;" json:"permissions,omitempty"`
}

func (Role) TableName() string { return "roles" }

type Permission struct {
	BaseModel
	Name     string `gorm:"type:varchar(100);uniqueIndex:idx_perm_tenant,priority:2;not null" json:"name"`
	Resource string `gorm:"type:varchar(100);not null" json:"resource"`
	Action   string `gorm:"type:varchar(50);not null" json:"action"` // create, read, update, delete
}

func (Permission) TableName() string { return "permissions" }

type UserRole struct {
	UserID   string `gorm:"type:char(36);primaryKey"`
	RoleID   string `gorm:"type:char(36);primaryKey"`
	TenantID string `gorm:"type:char(36);not null;index"`
}

func (UserRole) TableName() string { return "user_roles" }

type RolePermission struct {
	RoleID       string `gorm:"type:char(36);primaryKey"`
	PermissionID string `gorm:"type:char(36);primaryKey"`
	TenantID     string `gorm:"type:char(36);not null;index"`
}

func (RolePermission) TableName() string { return "role_permissions" }

type UserSession struct {
	BaseModel
	UserID       string    `gorm:"type:char(36);not null;index" json:"user_id"`
	RefreshToken string    `gorm:"type:varchar(500);uniqueIndex" json:"-"`
	UserAgent    string    `gorm:"type:varchar(500)" json:"user_agent"`
	IPAddress    string    `gorm:"type:varchar(45)" json:"ip_address"`
	ExpiresAt    time.Time `json:"expires_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
	IsActive     bool      `gorm:"default:true" json:"is_active"`
}

func (UserSession) TableName() string { return "user_sessions" }

// Auth request/response types
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	TenantID string `json:"tenant_id" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type TokenClaims struct {
	UserID   string   `json:"user_id"`
	TenantID string   `json:"tenant_id"`
	Username string   `json:"username"`
	Roles    []string `json:"roles"`
}
