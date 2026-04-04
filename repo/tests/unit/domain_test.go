package unit

import (
	"testing"

	"dispatchlearn/internal/domain"

	"github.com/stretchr/testify/assert"
)

func TestOrderStatusTransitions(t *testing.T) {
	t.Run("CREATED can transition to AVAILABLE", func(t *testing.T) {
		assert.True(t, domain.OrderCreated.CanTransitionTo(domain.OrderAvailable))
	})

	t.Run("AVAILABLE can transition to ACCEPTED", func(t *testing.T) {
		assert.True(t, domain.OrderAvailable.CanTransitionTo(domain.OrderAccepted))
	})

	t.Run("AVAILABLE can transition to EXPIRED", func(t *testing.T) {
		assert.True(t, domain.OrderAvailable.CanTransitionTo(domain.OrderExpired))
	})

	t.Run("ACCEPTED can transition to IN_PROGRESS", func(t *testing.T) {
		assert.True(t, domain.OrderAccepted.CanTransitionTo(domain.OrderInProgress))
	})

	t.Run("ACCEPTED can transition to CANCELLED", func(t *testing.T) {
		assert.True(t, domain.OrderAccepted.CanTransitionTo(domain.OrderCancelled))
	})

	t.Run("IN_PROGRESS can transition to COMPLETED", func(t *testing.T) {
		assert.True(t, domain.OrderInProgress.CanTransitionTo(domain.OrderCompleted))
	})

	// Invalid transitions
	t.Run("CREATED cannot transition to ACCEPTED", func(t *testing.T) {
		assert.False(t, domain.OrderCreated.CanTransitionTo(domain.OrderAccepted))
	})

	t.Run("COMPLETED cannot transition to anything", func(t *testing.T) {
		assert.False(t, domain.OrderCompleted.CanTransitionTo(domain.OrderAvailable))
		assert.False(t, domain.OrderCompleted.CanTransitionTo(domain.OrderCancelled))
	})

	t.Run("AVAILABLE cannot transition to COMPLETED directly", func(t *testing.T) {
		assert.False(t, domain.OrderAvailable.CanTransitionTo(domain.OrderCompleted))
	})

	t.Run("IN_PROGRESS cannot go back to AVAILABLE", func(t *testing.T) {
		assert.False(t, domain.OrderInProgress.CanTransitionTo(domain.OrderAvailable))
	})
}

func TestAPIResponse(t *testing.T) {
	t.Run("API response structure", func(t *testing.T) {
		resp := domain.APIResponse{
			Data: map[string]string{"key": "value"},
			Meta: &domain.Meta{Page: 1, PerPage: 20, Total: 100, TotalPages: 5},
		}
		assert.NotNil(t, resp.Data)
		assert.NotNil(t, resp.Meta)
		assert.Equal(t, 1, resp.Meta.Page)
		assert.Equal(t, int64(100), resp.Meta.Total)
	})

	t.Run("API error response", func(t *testing.T) {
		resp := domain.APIResponse{
			Errors: []domain.APIError{
				{Code: "VALIDATION_ERROR", Message: "field required", Field: "name"},
			},
		}
		assert.Len(t, resp.Errors, 1)
		assert.Equal(t, "VALIDATION_ERROR", resp.Errors[0].Code)
	})
}
