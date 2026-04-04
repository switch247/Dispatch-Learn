package worker

import (
	"fmt"
	"time"

	"dispatchlearn/internal/audit"
	"dispatchlearn/internal/domain"
	"dispatchlearn/logging"

	"gorm.io/gorm"
)

// ExpiryWorker runs a background ticker that expires stale orders and cancels unstarted accepted orders.
type ExpiryWorker struct {
	db       *gorm.DB
	audit    *audit.Service
	interval time.Duration
	stopCh   chan struct{}
}

func NewExpiryWorker(db *gorm.DB, auditSvc *audit.Service, interval time.Duration) *ExpiryWorker {
	return &ExpiryWorker{
		db:       db,
		audit:    auditSvc,
		interval: interval,
		stopCh:   make(chan struct{}),
	}
}

func (w *ExpiryWorker) Start() {
	logging.Info("worker", "expiry", "Starting expiry worker with interval "+w.interval.String())
	go w.run()
}

func (w *ExpiryWorker) Stop() {
	close(w.stopCh)
	logging.Info("worker", "expiry", "Expiry worker stopped")
}

func (w *ExpiryWorker) run() {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	// Run once immediately on startup
	w.process()

	for {
		select {
		case <-ticker.C:
			w.process()
		case <-w.stopCh:
			return
		}
	}
}

func (w *ExpiryWorker) process() {
	expired := w.expireStaleOrders()
	cancelled := w.cancelStaleAccepted()

	if expired > 0 || cancelled > 0 {
		logging.Info("worker", "expiry",
			"Processed: expired="+itoa(expired)+", cancelled="+itoa(cancelled))
	}
}

// expireStaleOrders expires AVAILABLE orders older than 15 minutes
func (w *ExpiryWorker) expireStaleOrders() int64 {
	cutoff := time.Now().Add(-15 * time.Minute)

	// Find orders to expire for audit logging
	var orders []domain.Order
	w.db.Where("status = ? AND available_at < ?",
		domain.OrderAvailable, cutoff).Find(&orders)

	if len(orders) == 0 {
		return 0
	}

	result := w.db.Model(&domain.Order{}).
		Where("status = ? AND available_at < ?",
			domain.OrderAvailable, cutoff).
		Update("status", domain.OrderExpired)

	for _, order := range orders {
		w.audit.Log(audit.LogEntry{
			TenantID:    order.TenantID,
			ActorID:     "system",
			Action:      "order.auto_expired",
			EntityType:  "order",
			EntityID:    order.ID,
			BeforeState: map[string]string{"status": string(domain.OrderAvailable)},
			AfterState:  map[string]string{"status": string(domain.OrderExpired)},
		})
	}

	return result.RowsAffected
}

// cancelStaleAccepted cancels ACCEPTED orders where:
// - current_time > time_window_start + 2 hours (scheduled window exceeded)
// - OR if no time_window_start, falls back to accepted_at + 2 hours
// - AND the order has not been started (status still ACCEPTED, not IN_PROGRESS)
func (w *ExpiryWorker) cancelStaleAccepted() int64 {
	now := time.Now()
	cutoffAccepted := now.Add(-2 * time.Hour)

	var orders []domain.Order
	// Orders with time_window_start: cancel if time_window_start + 2h has passed
	// Orders without time_window_start: fall back to accepted_at + 2h
	w.db.Where(
		"status = ? AND ((time_window_start IS NOT NULL AND DATE_ADD(time_window_start, INTERVAL 2 HOUR) < ?) OR (time_window_start IS NULL AND accepted_at < ?))",
		domain.OrderAccepted, now, cutoffAccepted,
	).Find(&orders)

	if len(orders) == 0 {
		return 0
	}

	var ids []string
	for _, order := range orders {
		ids = append(ids, order.ID)
	}

	result := w.db.Model(&domain.Order{}).
		Where("id IN ?", ids).
		Update("status", domain.OrderCancelled)

	for _, order := range orders {
		reason := "scheduled window exceeded (time_window_start + 2h)"
		if order.TimeWindowStart == nil {
			reason = "not started within 2h of acceptance"
		}
		w.audit.Log(audit.LogEntry{
			TenantID:    order.TenantID,
			ActorID:     "system",
			Action:      "order.auto_cancelled",
			EntityType:  "order",
			EntityID:    order.ID,
			BeforeState: map[string]string{"status": string(domain.OrderAccepted)},
			AfterState:  map[string]string{"status": string(domain.OrderCancelled), "reason": reason},
		})
	}

	return result.RowsAffected
}

func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
