package usecase

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"dispatchlearn/internal/audit"
	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/middleware"
	"dispatchlearn/internal/repository"
	"dispatchlearn/logging"

	"github.com/google/uuid"
)

type WebhookUseCase struct {
	repo         *repository.SystemRepository
	audit        *audit.Service
	quotaTracker *middleware.WebhookQuotaTracker
}

func NewWebhookUseCase(repo *repository.SystemRepository, audit *audit.Service, quotaTracker *middleware.WebhookQuotaTracker) *WebhookUseCase {
	return &WebhookUseCase{repo: repo, audit: audit, quotaTracker: quotaTracker}
}

func (uc *WebhookUseCase) CreateSubscription(tenantID, actorID string, req *domain.CreateWebhookRequest) (*domain.WebhookSubscription, error) {
	var dateStart, dateEnd *time.Time
	if req.DateRangeStart != "" {
		t, err := time.Parse(time.RFC3339, req.DateRangeStart)
		if err == nil {
			dateStart = &t
		}
	}
	if req.DateRangeEnd != "" {
		t, err := time.Parse(time.RFC3339, req.DateRangeEnd)
		if err == nil {
			dateEnd = &t
		}
	}

	sub := &domain.WebhookSubscription{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		URL:            req.URL,
		EventTypes:     req.EventTypes,
		Secret:         req.Secret,
		IsActive:       true,
		DateRangeStart: dateStart,
		DateRangeEnd:   dateEnd,
	}

	if err := uc.repo.CreateWebhookSubscription(sub); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "webhook.subscription.created",
		EntityType: "webhook_subscription",
		EntityID:   sub.ID,
		AfterState: map[string]string{"url": req.URL, "events": req.EventTypes},
	})

	return sub, nil
}

func (uc *WebhookUseCase) ListSubscriptions(tenantID string) ([]domain.WebhookSubscription, error) {
	return uc.repo.ListWebhookSubscriptions(tenantID)
}

func (uc *WebhookUseCase) GetSubscription(tenantID, id string) (*domain.WebhookSubscription, error) {
	return uc.repo.FindWebhookSubscription(tenantID, id)
}

// DispatchEvent sends an event to all matching subscribers
func (uc *WebhookUseCase) DispatchEvent(tenantID, eventType string, payload interface{}) error {
	if !uc.quotaTracker.Increment(tenantID) {
		return errors.New("webhook daily quota exceeded")
	}

	subs, err := uc.repo.FindSubscriptionsByEvent(tenantID, eventType)
	if err != nil {
		return err
	}

	payloadJSON, _ := json.Marshal(payload)

	for _, sub := range subs {
		// Check date range filter
		now := time.Now()
		if sub.DateRangeStart != nil && now.Before(*sub.DateRangeStart) {
			continue
		}
		if sub.DateRangeEnd != nil && now.After(*sub.DateRangeEnd) {
			continue
		}

		deliveryID := uuid.New().String()
		nonce := uuid.New().String()

		delivery := &domain.WebhookDelivery{
			BaseModel: domain.BaseModel{
				ID:       uuid.New().String(),
				TenantID: tenantID,
			},
			SubscriptionID: sub.ID,
			DeliveryID:     deliveryID,
			EventType:      eventType,
			Payload:        string(payloadJSON),
			AttemptCount:   0,
			MaxAttempts:    5,
			Status:         "pending",
			Nonce:          nonce,
		}

		uc.repo.CreateWebhookDelivery(delivery)

		// Attempt delivery (mocked for LAN-only)
		go uc.attemptDelivery(sub, delivery)
	}

	return nil
}

func (uc *WebhookUseCase) attemptDelivery(sub domain.WebhookSubscription, delivery *domain.WebhookDelivery) {
	// Mocking webhook delivery for LAN-only environment
	// In production, this would make HTTP POST to sub.URL with HMAC signature
	logging.Info("webhook", "delivery", fmt.Sprintf("Delivering event %s to %s (delivery_id: %s)",
		delivery.EventType, sub.URL, delivery.DeliveryID))

	// Compute HMAC-SHA256 signature
	signature := computeHMAC(delivery.Payload, sub.Secret, delivery.Nonce)

	for attempt := 1; attempt <= delivery.MaxAttempts; attempt++ {
		delivery.AttemptCount = attempt

		// Mocking Payment Gateway response for audit stability
		// In production: POST to sub.URL with headers X-Signature, X-Delivery-ID, X-Nonce
		success := uc.mockDeliverWebhook(sub.URL, delivery.Payload, signature, delivery.DeliveryID, delivery.Nonce)

		if success {
			now := time.Now()
			delivery.Status = "delivered"
			delivery.DeliveredAt = &now
			delivery.ResponseCode = http.StatusOK
			uc.repo.UpdateWebhookDelivery(delivery)
			return
		}

		// Exponential backoff
		if attempt < delivery.MaxAttempts {
			backoff := time.Duration(1<<uint(attempt)) * time.Second
			nextRetry := time.Now().Add(backoff)
			delivery.NextRetryAt = &nextRetry
			delivery.Status = "failed"
			uc.repo.UpdateWebhookDelivery(delivery)
			time.Sleep(backoff)
		}
	}

	// Dead letter
	delivery.Status = "dead_letter"
	uc.repo.UpdateWebhookDelivery(delivery)
	logging.Warn("webhook", "delivery", fmt.Sprintf("Dead-lettered delivery %s after %d attempts",
		delivery.DeliveryID, delivery.MaxAttempts))
}

// mockDeliverWebhook simulates LAN webhook delivery
// Mocking external webhook endpoint for audit stability
func (uc *WebhookUseCase) mockDeliverWebhook(url, payload, signature, deliveryID, nonce string) bool {
	// Stub: In production, this would be an HTTP POST with:
	// Headers: Content-Type: application/json
	//          X-Webhook-Signature: <HMAC-SHA256>
	//          X-Delivery-ID: <unique delivery ID>
	//          X-Nonce: <anti-replay nonce>
	// Body: payload

	// Simulate successful delivery for LAN-only testing
	if strings.HasPrefix(url, "http") {
		logging.Info("webhook", "mock", fmt.Sprintf(
			"[STUB] POST %s | delivery_id=%s | nonce=%s | sig=%s",
			url, deliveryID, nonce, signature[:16]+"..."))
		return true
	}
	return false
}

func computeHMAC(payload, secret, nonce string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(nonce + "." + payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func (uc *WebhookUseCase) ListDeadLetters(tenantID string) ([]domain.WebhookDelivery, error) {
	return uc.repo.FindDeadLetterDeliveries(tenantID)
}
