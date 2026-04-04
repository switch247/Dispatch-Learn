package domain

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type BaseModel struct {
	ID        string         `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID  string         `gorm:"type:char(36);not null;index" json:"tenant_id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

func (b *BaseModel) BeforeCreate(tx *gorm.DB) error {
	if b.ID == "" {
		b.ID = uuid.New().String()
	}
	return nil
}

// Standard API response format
type APIResponse struct {
	Data   interface{}   `json:"data"`
	Meta   *Meta         `json:"meta,omitempty"`
	Errors []APIError    `json:"errors,omitempty"`
}

type Meta struct {
	Page       int   `json:"page,omitempty"`
	PerPage    int   `json:"per_page,omitempty"`
	Total      int64 `json:"total,omitempty"`
	TotalPages int   `json:"total_pages,omitempty"`
}

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}
