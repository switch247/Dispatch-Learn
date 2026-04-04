package repository

import (
	"dispatchlearn/internal/domain"

	"gorm.io/gorm"
)

type SystemRepository struct {
	db *gorm.DB
}

func NewSystemRepository(db *gorm.DB) *SystemRepository {
	return &SystemRepository{db: db}
}

// Audit Logs
func (r *SystemRepository) ListAuditLogs(tenantID string, entityType string, page, perPage int) ([]domain.AuditLog, int64, error) {
	var logs []domain.AuditLog
	var total int64

	q := r.db.Model(&domain.AuditLog{}).Where("tenant_id = ?", tenantID)
	if entityType != "" {
		q = q.Where("entity_type = ?", entityType)
	}
	q.Count(&total)

	err := r.db.Where("tenant_id = ?", tenantID).
		Scopes(func(db *gorm.DB) *gorm.DB {
			if entityType != "" {
				return db.Where("entity_type = ?", entityType)
			}
			return db
		}).
		Order("timestamp DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&logs).Error
	return logs, total, err
}

// Config Changes
func (r *SystemRepository) CreateConfigChange(change *domain.ConfigChange) error {
	return r.db.Create(change).Error
}

func (r *SystemRepository) ListConfigChanges(tenantID string, page, perPage int) ([]domain.ConfigChange, int64, error) {
	var changes []domain.ConfigChange
	var total int64

	r.db.Model(&domain.ConfigChange{}).Where("tenant_id = ?", tenantID).Count(&total)

	err := r.db.Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&changes).Error
	return changes, total, err
}

// Reports
func (r *SystemRepository) CreateReport(report *domain.Report) error {
	return r.db.Create(report).Error
}

func (r *SystemRepository) FindReportByID(tenantID, id string) (*domain.Report, error) {
	var report domain.Report
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&report).Error
	if err != nil {
		return nil, err
	}
	return &report, nil
}

func (r *SystemRepository) UpdateReport(report *domain.Report) error {
	return r.db.Save(report).Error
}

func (r *SystemRepository) ListReports(tenantID string, page, perPage int) ([]domain.Report, int64, error) {
	var reports []domain.Report
	var total int64

	r.db.Model(&domain.Report{}).Where("tenant_id = ?", tenantID).Count(&total)

	err := r.db.Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&reports).Error
	return reports, total, err
}

// Webhooks
func (r *SystemRepository) CreateWebhookSubscription(sub *domain.WebhookSubscription) error {
	return r.db.Create(sub).Error
}

func (r *SystemRepository) FindWebhookSubscription(tenantID, id string) (*domain.WebhookSubscription, error) {
	var sub domain.WebhookSubscription
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&sub).Error
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SystemRepository) ListWebhookSubscriptions(tenantID string) ([]domain.WebhookSubscription, error) {
	var subs []domain.WebhookSubscription
	err := r.db.Where("tenant_id = ? AND is_active = ?", tenantID, true).Find(&subs).Error
	return subs, err
}

func (r *SystemRepository) FindSubscriptionsByEvent(tenantID, eventType string) ([]domain.WebhookSubscription, error) {
	var subs []domain.WebhookSubscription
	err := r.db.Where("tenant_id = ? AND is_active = ? AND FIND_IN_SET(?, event_types) > 0",
		tenantID, true, eventType).Find(&subs).Error
	return subs, err
}

func (r *SystemRepository) CreateWebhookDelivery(delivery *domain.WebhookDelivery) error {
	return r.db.Create(delivery).Error
}

func (r *SystemRepository) UpdateWebhookDelivery(delivery *domain.WebhookDelivery) error {
	return r.db.Save(delivery).Error
}

func (r *SystemRepository) FindPendingDeliveries(tenantID string) ([]domain.WebhookDelivery, error) {
	var deliveries []domain.WebhookDelivery
	err := r.db.Where("tenant_id = ? AND status IN ? AND attempt_count < max_attempts",
		tenantID, []string{"pending", "failed"}).
		Order("created_at ASC").
		Limit(100).
		Find(&deliveries).Error
	return deliveries, err
}

func (r *SystemRepository) FindDeadLetterDeliveries(tenantID string) ([]domain.WebhookDelivery, error) {
	var deliveries []domain.WebhookDelivery
	err := r.db.Where("tenant_id = ? AND status = ?", tenantID, "dead_letter").
		Order("created_at DESC").
		Find(&deliveries).Error
	return deliveries, err
}

// Quota Overrides
func (r *SystemRepository) FindQuotaOverride(tenantID string) (*domain.QuotaOverride, error) {
	var override domain.QuotaOverride
	err := r.db.Where("tenant_id = ?", tenantID).First(&override).Error
	if err != nil {
		return nil, err
	}
	return &override, nil
}

func (r *SystemRepository) UpsertQuotaOverride(override *domain.QuotaOverride) error {
	return r.db.Save(override).Error
}
