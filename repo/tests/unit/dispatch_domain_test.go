package unit

import (
	"testing"

	"dispatchlearn/internal/domain"

	"github.com/stretchr/testify/assert"
)

// ---- OrderStatus string constants ----

func TestOrderStatusStringValues(t *testing.T) {
	assert.Equal(t, domain.OrderStatus("CREATED"), domain.OrderCreated)
	assert.Equal(t, domain.OrderStatus("AVAILABLE"), domain.OrderAvailable)
	assert.Equal(t, domain.OrderStatus("ACCEPTED"), domain.OrderAccepted)
	assert.Equal(t, domain.OrderStatus("IN_PROGRESS"), domain.OrderInProgress)
	assert.Equal(t, domain.OrderStatus("COMPLETED"), domain.OrderCompleted)
	assert.Equal(t, domain.OrderStatus("EXPIRED"), domain.OrderExpired)
	assert.Equal(t, domain.OrderStatus("CANCELLED"), domain.OrderCancelled)
}

func TestOrderStatusDistinct(t *testing.T) {
	statuses := []domain.OrderStatus{
		domain.OrderCreated,
		domain.OrderAvailable,
		domain.OrderAccepted,
		domain.OrderInProgress,
		domain.OrderCompleted,
		domain.OrderExpired,
		domain.OrderCancelled,
	}

	seen := map[domain.OrderStatus]bool{}
	for _, s := range statuses {
		assert.False(t, seen[s], "duplicate status: %s", s)
		seen[s] = true
	}
}

// ---- ValidTransitions map completeness ----

func TestValidTransitionsMapEntries(t *testing.T) {
	// States with defined outgoing transitions
	assert.Contains(t, domain.ValidTransitions, domain.OrderCreated)
	assert.Contains(t, domain.ValidTransitions, domain.OrderAvailable)
	assert.Contains(t, domain.ValidTransitions, domain.OrderAccepted)
	assert.Contains(t, domain.ValidTransitions, domain.OrderInProgress)
}

func TestValidTransitionsTerminalStates(t *testing.T) {
	// Terminal states must not appear as keys in ValidTransitions
	terminal := []domain.OrderStatus{
		domain.OrderCompleted,
		domain.OrderExpired,
		domain.OrderCancelled,
	}
	for _, s := range terminal {
		_, exists := domain.ValidTransitions[s]
		assert.False(t, exists, "terminal status %s should have no outgoing transitions", s)
	}
}

func TestValidTransitionsCorrectTargets(t *testing.T) {
	transitions := domain.ValidTransitions

	createdTargets := transitions[domain.OrderCreated]
	assert.Contains(t, createdTargets, domain.OrderAvailable)
	assert.Len(t, createdTargets, 1)

	availableTargets := transitions[domain.OrderAvailable]
	assert.Contains(t, availableTargets, domain.OrderAccepted)
	assert.Contains(t, availableTargets, domain.OrderExpired)
	assert.Len(t, availableTargets, 2)

	acceptedTargets := transitions[domain.OrderAccepted]
	assert.Contains(t, acceptedTargets, domain.OrderInProgress)
	assert.Contains(t, acceptedTargets, domain.OrderCancelled)
	assert.Len(t, acceptedTargets, 2)

	inProgressTargets := transitions[domain.OrderInProgress]
	assert.Contains(t, inProgressTargets, domain.OrderCompleted)
	assert.Len(t, inProgressTargets, 1)
}

// ---- CanTransitionTo method ----

func TestCanTransitionToValidPaths(t *testing.T) {
	cases := []struct {
		from   domain.OrderStatus
		to     domain.OrderStatus
		expect bool
	}{
		{domain.OrderCreated, domain.OrderAvailable, true},
		{domain.OrderAvailable, domain.OrderAccepted, true},
		{domain.OrderAvailable, domain.OrderExpired, true},
		{domain.OrderAccepted, domain.OrderInProgress, true},
		{domain.OrderAccepted, domain.OrderCancelled, true},
		{domain.OrderInProgress, domain.OrderCompleted, true},
		// Invalid hops
		{domain.OrderCreated, domain.OrderCompleted, false},
		{domain.OrderCreated, domain.OrderAccepted, false},
		{domain.OrderCompleted, domain.OrderCreated, false},
		{domain.OrderExpired, domain.OrderCreated, false},
		{domain.OrderCancelled, domain.OrderAvailable, false},
	}

	for _, tc := range cases {
		result := tc.from.CanTransitionTo(tc.to)
		assert.Equal(t, tc.expect, result, "%s → %s", tc.from, tc.to)
	}
}

func TestCannotTransitionFromTerminalStates(t *testing.T) {
	terminals := []domain.OrderStatus{
		domain.OrderCompleted,
		domain.OrderExpired,
		domain.OrderCancelled,
	}
	allStatuses := []domain.OrderStatus{
		domain.OrderCreated, domain.OrderAvailable, domain.OrderAccepted,
		domain.OrderInProgress, domain.OrderCompleted, domain.OrderExpired, domain.OrderCancelled,
	}

	for _, terminal := range terminals {
		for _, target := range allStatuses {
			assert.False(t, terminal.CanTransitionTo(target),
				"terminal state %s should not transition to %s", terminal, target)
		}
	}
}

// ---- Assignment mode values ----

func TestAssignmentModeValues(t *testing.T) {
	validModes := map[string]bool{"grab": true, "assigned": true}

	valid := []string{"grab", "assigned"}
	invalid := []string{"manual", "auto", "", "GRAB", "Assigned"}

	for _, m := range valid {
		assert.True(t, validModes[m], "expected %q to be a valid assignment mode", m)
	}
	for _, m := range invalid {
		assert.False(t, validModes[m], "expected %q to be an invalid assignment mode", m)
	}
}

// ---- AgentProfile defaults ----

func TestAgentProfileDefaults(t *testing.T) {
	ap := domain.AgentProfile{
		IsAvailable:     true,
		MaxWorkload:     8,
		ReputationScore: 50.00,
	}

	assert.True(t, ap.IsAvailable)
	assert.Equal(t, 8, ap.MaxWorkload)
	assert.InDelta(t, 50.00, ap.ReputationScore, 0.01)
}

// ---- RecommendationResponse fields ----

func TestRecommendationResponseFields(t *testing.T) {
	rr := domain.RecommendationResponse{
		AgentID:         "agent-1",
		Distance:        12.5,
		ReputationScore: 75.0,
		OpenTasks:       2,
		RankingScore:    0.82,
		IsQualified:     true,
	}

	assert.Equal(t, "agent-1", rr.AgentID)
	assert.InDelta(t, 12.5, rr.Distance, 0.001)
	assert.True(t, rr.IsQualified)
	assert.InDelta(t, 0.82, rr.RankingScore, 0.001)
}

// ---- DistanceMatrix source enum ----

func TestDistanceMatrixSourceValues(t *testing.T) {
	validSources := map[string]bool{
		"zip4_centroid": true,
		"precomputed":   true,
		"manual":        true,
	}

	valid := []string{"zip4_centroid", "precomputed", "manual"}
	invalid := []string{"gps", "estimated", "", "ZIP4"}

	for _, s := range valid {
		assert.True(t, validSources[s], "expected %q to be a valid source", s)
	}
	for _, s := range invalid {
		assert.False(t, validSources[s], "expected %q to be an invalid source", s)
	}
}
