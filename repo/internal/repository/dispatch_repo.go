package repository

import (
	"time"

	"dispatchlearn/internal/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type DispatchRepository struct {
	db *gorm.DB
}

func NewDispatchRepository(db *gorm.DB) *DispatchRepository {
	return &DispatchRepository{db: db}
}

// Orders
func (r *DispatchRepository) CreateOrder(order *domain.Order) error {
	return r.db.Create(order).Error
}

func (r *DispatchRepository) FindOrderByID(tenantID, id string) (*domain.Order, error) {
	var order domain.Order
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *DispatchRepository) FindOrderByOrderNo(tenantID, orderNo string) (*domain.Order, error) {
	var order domain.Order
	err := r.db.Where("tenant_id = ? AND order_no = ?", tenantID, orderNo).First(&order).Error
	if err != nil {
		return nil, err
	}
	return &order, nil
}

func (r *DispatchRepository) ListOrders(tenantID string, status *domain.OrderStatus, page, perPage int) ([]domain.Order, int64, error) {
	var orders []domain.Order
	var total int64

	q := r.db.Model(&domain.Order{}).Where("tenant_id = ?", tenantID)
	if status != nil {
		q = q.Where("status = ?", *status)
	}
	q.Count(&total)

	err := r.db.Where("tenant_id = ?", tenantID).
		Scopes(func(db *gorm.DB) *gorm.DB {
			if status != nil {
				return db.Where("status = ?", *status)
			}
			return db
		}).
		Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&orders).Error
	return orders, total, err
}

func (r *DispatchRepository) UpdateOrderStatus(tenantID, orderID string, status domain.OrderStatus) error {
	updates := map[string]interface{}{"status": status}
	switch status {
	case domain.OrderAvailable:
		now := time.Now()
		updates["available_at"] = &now
	case domain.OrderAccepted:
		now := time.Now()
		updates["accepted_at"] = &now
	case domain.OrderCompleted:
		now := time.Now()
		updates["completed_at"] = &now
	}
	return r.db.Model(&domain.Order{}).
		Where("tenant_id = ? AND id = ?", tenantID, orderID).
		Updates(updates).Error
}

func (r *DispatchRepository) AssignAgent(tenantID, orderID, agentID string) error {
	return r.db.Model(&domain.Order{}).
		Where("tenant_id = ? AND id = ?", tenantID, orderID).
		Update("assigned_agent_id", agentID).Error
}

// Acceptance - uses DB locking for single winner
func (r *DispatchRepository) AcceptOrder(tenantID string, acceptance *domain.DispatchAcceptance) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Lock the order row
		var order domain.Order
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("tenant_id = ? AND id = ?", tenantID, acceptance.OrderID).
			First(&order).Error; err != nil {
			return err
		}

		// Check order is AVAILABLE
		if order.Status != domain.OrderAvailable {
			return gorm.ErrInvalidData
		}

		// Check no existing acceptance
		var existingCount int64
		tx.Model(&domain.DispatchAcceptance{}).
			Where("tenant_id = ? AND order_id = ?", tenantID, acceptance.OrderID).
			Count(&existingCount)
		if existingCount > 0 {
			return gorm.ErrDuplicatedKey
		}

		// Create acceptance
		if err := tx.Create(acceptance).Error; err != nil {
			return err
		}

		// Update order status
		now := time.Now()
		return tx.Model(&domain.Order{}).
			Where("id = ?", order.ID).
			Updates(map[string]interface{}{
				"status":            domain.OrderAccepted,
				"assigned_agent_id": acceptance.AgentID,
				"accepted_at":       &now,
			}).Error
	})
}

// Expire orders that have been AVAILABLE for > 15 minutes
func (r *DispatchRepository) ExpireStaleOrders(tenantID string) (int64, error) {
	cutoff := time.Now().Add(-15 * time.Minute)
	result := r.db.Model(&domain.Order{}).
		Where("tenant_id = ? AND status = ? AND available_at < ?",
			tenantID, domain.OrderAvailable, cutoff).
		Update("status", domain.OrderExpired)
	return result.RowsAffected, result.Error
}

// Cancel orders accepted but not started within 2 hours
func (r *DispatchRepository) CancelStaleAccepted(tenantID string) (int64, error) {
	cutoff := time.Now().Add(-2 * time.Hour)
	result := r.db.Model(&domain.Order{}).
		Where("tenant_id = ? AND status = ? AND accepted_at < ?",
			tenantID, domain.OrderAccepted, cutoff).
		Update("status", domain.OrderCancelled)
	return result.RowsAffected, result.Error
}

// FindAndExpireAvailable finds and expires AVAILABLE orders older than 15 minutes for a specific tenant.
func (r *DispatchRepository) FindAndExpireAvailable(tenantID string) ([]domain.Order, error) {
	cutoff := time.Now().Add(-15 * time.Minute)
	var orders []domain.Order
	r.db.Where("tenant_id = ? AND status = ? AND available_at < ?", tenantID, domain.OrderAvailable, cutoff).Find(&orders)
	if len(orders) == 0 {
		return nil, nil
	}
	var ids []string
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	r.db.Model(&domain.Order{}).Where("tenant_id = ? AND id IN ?", tenantID, ids).Update("status", domain.OrderExpired)
	return orders, nil
}

// FindAndCancelExpiredAccepted finds and cancels ACCEPTED orders whose time window has expired for a specific tenant.
func (r *DispatchRepository) FindAndCancelExpiredAccepted(tenantID string) ([]domain.Order, error) {
	now := time.Now()
	cutoff := now.Add(-2 * time.Hour)
	var orders []domain.Order
	r.db.Where(
		"tenant_id = ? AND status = ? AND ((time_window_end IS NOT NULL AND time_window_end < ?) OR (time_window_end IS NULL AND time_window_start IS NOT NULL AND DATE_ADD(time_window_start, INTERVAL 2 HOUR) < ?) OR (time_window_end IS NULL AND time_window_start IS NULL AND accepted_at < ?))",
		tenantID, domain.OrderAccepted, now, now, cutoff,
	).Find(&orders)
	if len(orders) == 0 {
		return nil, nil
	}
	var ids []string
	for _, o := range orders {
		ids = append(ids, o.ID)
	}
	r.db.Model(&domain.Order{}).Where("tenant_id = ? AND id IN ?", tenantID, ids).Update("status", domain.OrderCancelled)
	return orders, nil
}

// ListDistinctTenantIDs returns all distinct tenant IDs from the orders table.
func (r *DispatchRepository) ListDistinctTenantIDs() ([]string, error) {
	var tenantIDs []string
	err := r.db.Model(&domain.Order{}).Distinct("tenant_id").Pluck("tenant_id", &tenantIDs).Error
	return tenantIDs, err
}

// AvgCompletionMinutes calculates the average duration from time_window_start (or created_at) to completed_at for completed orders.
func (r *DispatchRepository) AvgCompletionMinutes(tenantID string) (float64, error) {
	var result struct{ Avg float64 }
	err := r.db.Model(&domain.Order{}).
		Select("COALESCE(AVG(TIMESTAMPDIFF(MINUTE, COALESCE(time_window_start, created_at), completed_at)), 0) as avg").
		Where("tenant_id = ? AND status = ? AND completed_at IS NOT NULL", tenantID, domain.OrderCompleted).
		Scan(&result).Error
	return result.Avg, err
}

// CountReturnedOrders counts cancelled orders explicitly tagged as return-related.
func (r *DispatchRepository) CountReturnedOrders(tenantID string) (int64, error) {
	var count int64
	err := r.db.Model(&domain.Order{}).
		Where(`tenant_id = ? AND status = ?
			AND (
				LOWER(category) LIKE '%return%'
				OR LOWER(override_reason) LIKE '%return%'
			)`, tenantID, domain.OrderCancelled).
		Count(&count).Error
	return count, err
}

// Service Zones
func (r *DispatchRepository) CreateServiceZone(zone *domain.ServiceZone) error {
	return r.db.Create(zone).Error
}

func (r *DispatchRepository) FindServiceZoneByID(tenantID, id string) (*domain.ServiceZone, error) {
	var zone domain.ServiceZone
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&zone).Error
	if err != nil {
		return nil, err
	}
	return &zone, nil
}

func (r *DispatchRepository) ListServiceZones(tenantID string) ([]domain.ServiceZone, error) {
	var zones []domain.ServiceZone
	err := r.db.Where("tenant_id = ?", tenantID).Find(&zones).Error
	return zones, err
}

// Distance Matrix
func (r *DispatchRepository) FindDistance(tenantID, fromZoneID, toZoneID string) (*domain.DistanceMatrix, error) {
	var dm domain.DistanceMatrix
	err := r.db.Where("tenant_id = ? AND from_zone_id = ? AND to_zone_id = ?",
		tenantID, fromZoneID, toZoneID).First(&dm).Error
	if err != nil {
		return nil, err
	}
	return &dm, nil
}

func (r *DispatchRepository) UpsertDistance(dm *domain.DistanceMatrix) error {
	return r.db.Save(dm).Error
}

// Agent Profiles
func (r *DispatchRepository) FindAgentProfile(tenantID, userID string) (*domain.AgentProfile, error) {
	var profile domain.AgentProfile
	err := r.db.Where("tenant_id = ? AND user_id = ?", tenantID, userID).First(&profile).Error
	if err != nil {
		return nil, err
	}
	return &profile, nil
}

func (r *DispatchRepository) CreateAgentProfile(profile *domain.AgentProfile) error {
	return r.db.Create(profile).Error
}

func (r *DispatchRepository) UpdateAgentProfile(profile *domain.AgentProfile) error {
	return r.db.Save(profile).Error
}

func (r *DispatchRepository) ListAvailableAgents(tenantID string) ([]domain.AgentProfile, error) {
	var profiles []domain.AgentProfile
	err := r.db.Where("tenant_id = ? AND is_available = ?", tenantID, true).
		Find(&profiles).Error
	return profiles, err
}

// Agent Metrics
func (r *DispatchRepository) FindAgentMetrics(tenantID, agentID string) (*domain.AgentMetrics, error) {
	var metrics domain.AgentMetrics
	err := r.db.Where("tenant_id = ? AND agent_id = ?", tenantID, agentID).First(&metrics).Error
	if err != nil {
		return nil, err
	}
	return &metrics, nil
}

func (r *DispatchRepository) UpsertAgentMetrics(metrics *domain.AgentMetrics) error {
	return r.db.Save(metrics).Error
}

func (r *DispatchRepository) CountOpenTasks(tenantID, agentID string) (int64, error) {
	var count int64
	err := r.db.Model(&domain.Order{}).
		Where("tenant_id = ? AND assigned_agent_id = ? AND status IN ?",
			tenantID, agentID,
			[]domain.OrderStatus{domain.OrderAvailable, domain.OrderAccepted, domain.OrderInProgress}).
		Count(&count).Error
	return count, err
}

// Zip4 Centroids
func (r *DispatchRepository) FindZip4Centroid(tenantID, zipCode string) (*domain.Zip4Centroid, error) {
	var centroid domain.Zip4Centroid
	err := r.db.Where("tenant_id = ? AND zip_code = ?", tenantID, zipCode).First(&centroid).Error
	if err != nil {
		return nil, err
	}
	return &centroid, nil
}

func (r *DispatchRepository) UpsertZip4Centroid(centroid *domain.Zip4Centroid) error {
	return r.db.Save(centroid).Error
}
