package domain

import "time"

type AuditLog struct {
	ID           string    `gorm:"type:char(36);primaryKey" json:"id"`
	TenantID     string    `gorm:"type:char(36);not null;index" json:"tenant_id"`
	ActorID      string    `gorm:"type:char(36);not null;index" json:"actor_id"`
	Action       string    `gorm:"type:varchar(100);not null;index" json:"action"`
	EntityType   string    `gorm:"type:varchar(100);not null;index" json:"entity_type"`
	EntityID     string    `gorm:"type:char(36);not null" json:"entity_id"`
	BeforeState  string    `gorm:"type:text" json:"before_state,omitempty"`
	AfterState   string    `gorm:"type:text" json:"after_state,omitempty"`
	Timestamp    time.Time `gorm:"not null;index" json:"timestamp"`
	TimestampMs  int64     `gorm:"type:bigint;not null" json:"timestamp_ms"`
	PreviousHash string    `gorm:"type:varchar(64)" json:"previous_hash"`
	CurrentHash  string    `gorm:"type:varchar(64);not null" json:"current_hash"`
	IPAddress    string    `gorm:"type:varchar(45)" json:"ip_address"`
}

func (AuditLog) TableName() string { return "audit_logs" }

type ConfigChange struct {
	BaseModel
	ChangedBy   string `gorm:"type:char(36);not null" json:"changed_by"`
	ConfigKey   string `gorm:"type:varchar(255);not null;index" json:"config_key"`
	OldValue    string `gorm:"type:text" json:"old_value"`
	NewValue    string `gorm:"type:text" json:"new_value"`
	Reason      string `gorm:"type:text" json:"reason"`
}

func (ConfigChange) TableName() string { return "config_changes" }

type Report struct {
	BaseModel
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	ReportType  string    `gorm:"type:varchar(100);not null" json:"report_type"`
	Parameters  string    `gorm:"type:text" json:"parameters"`
	FilePath    string    `gorm:"type:varchar(500)" json:"file_path"`
	FileChecksum string   `gorm:"type:varchar(64)" json:"file_checksum"`
	GeneratedAt time.Time `json:"generated_at"`
	GeneratedBy string    `gorm:"type:char(36);not null" json:"generated_by"`
	Status      string    `gorm:"type:varchar(20);default:'pending'" json:"status"`
}

func (Report) TableName() string { return "reports" }

type WebhookSubscription struct {
	BaseModel
	URL          string     `gorm:"type:varchar(500);not null" json:"url"`
	EventTypes   string     `gorm:"type:text;not null" json:"event_types"` // comma-separated
	Secret       string     `gorm:"type:varchar(255);not null" json:"-"`
	IsActive     bool       `gorm:"default:true" json:"is_active"`
	DateRangeStart *time.Time `json:"date_range_start,omitempty"`
	DateRangeEnd   *time.Time `json:"date_range_end,omitempty"`
}

func (WebhookSubscription) TableName() string { return "webhook_subscriptions" }

type WebhookDelivery struct {
	BaseModel
	SubscriptionID string    `gorm:"type:char(36);not null;index" json:"subscription_id"`
	DeliveryID     string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"delivery_id"`
	EventType      string    `gorm:"type:varchar(100);not null" json:"event_type"`
	Payload        string    `gorm:"type:text;not null" json:"payload"`
	ResponseCode   int       `json:"response_code"`
	ResponseBody   string    `gorm:"type:text" json:"response_body"`
	AttemptCount   int       `gorm:"default:0" json:"attempt_count"`
	MaxAttempts    int       `gorm:"default:5" json:"max_attempts"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	Status         string    `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending, delivered, failed, dead_letter
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
	Nonce          string    `gorm:"type:varchar(255);not null" json:"nonce"`
}

func (WebhookDelivery) TableName() string { return "webhook_deliveries" }

type Tenant struct {
	ID        string    `gorm:"type:char(36);primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"name"`
	IsActive  bool      `gorm:"default:true" json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Tenant) TableName() string { return "tenants" }

type QuotaOverride struct {
	BaseModel
	RPM              int `gorm:"default:600" json:"rpm"`
	Burst            int `gorm:"default:120" json:"burst"`
	WebhookDailyLimit int `gorm:"default:10000" json:"webhook_daily_limit"`
}

func (QuotaOverride) TableName() string { return "quota_overrides" }

// Webhook request types
type CreateWebhookRequest struct {
	URL            string `json:"url" binding:"required"`
	EventTypes     string `json:"event_types" binding:"required"`
	Secret         string `json:"secret" binding:"required"`
	DateRangeStart string `json:"date_range_start,omitempty"`
	DateRangeEnd   string `json:"date_range_end,omitempty"`
}
