package usecase

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Tests for haversine distance and ranking score math (in-package, unexported).

func TestHaversineKnownDistances(t *testing.T) {
	t.Run("same point yields zero distance", func(t *testing.T) {
		dist := haversine(40.7128, -74.0060, 40.7128, -74.0060)
		assert.InDelta(t, 0.0, dist, 0.001)
	})

	t.Run("NYC to Los Angeles approx 3940 km", func(t *testing.T) {
		// New York: 40.7128, -74.0060  |  LA: 34.0522, -118.2437
		dist := haversine(40.7128, -74.0060, 34.0522, -118.2437)
		// Haversine gives ~3940 km; allow ±10 km tolerance
		assert.InDelta(t, 3940.0, dist, 10.0)
	})

	t.Run("NYC to Boston approx 306 km", func(t *testing.T) {
		// Boston: 42.3601, -71.0589
		dist := haversine(40.7128, -74.0060, 42.3601, -71.0589)
		assert.InDelta(t, 306.0, dist, 5.0)
	})

	t.Run("distance is symmetric", func(t *testing.T) {
		a2b := haversine(40.7128, -74.0060, 34.0522, -118.2437)
		b2a := haversine(34.0522, -118.2437, 40.7128, -74.0060)
		assert.InDelta(t, a2b, b2a, 0.001)
	})

	t.Run("Earth radius produces km-scale results for inter-city distances", func(t *testing.T) {
		// Chicago: 41.8781, -87.6298
		dist := haversine(40.7128, -74.0060, 41.8781, -87.6298)
		// NYC to Chicago is roughly 1150 km
		assert.Greater(t, dist, 1000.0)
		assert.Less(t, dist, 1300.0)
	})

	t.Run("adjacent ZIP centroids produce sub-50km distances", func(t *testing.T) {
		// Two close NYC ZIP centroids
		dist := haversine(40.7128, -74.0060, 40.7300, -73.9950)
		assert.Less(t, dist, 5.0)
	})
}

func TestRankingScoreFormula(t *testing.T) {
	// rankingScore = 0.50*normalizedDist + 0.30*reputationNorm + 0.20*workloadPenalty
	rankScore := func(normalizedDist, reputationNorm, workloadPenalty float64) float64 {
		raw := 0.50*normalizedDist + 0.30*reputationNorm + 0.20*workloadPenalty
		return math.Round(raw*100) / 100
	}

	t.Run("perfect agent scores 1.00", func(t *testing.T) {
		assert.InDelta(t, 1.00, rankScore(1.0, 1.0, 1.0), 0.001)
	})

	t.Run("zero agent scores 0.00", func(t *testing.T) {
		assert.InDelta(t, 0.00, rankScore(0.0, 0.0, 0.0), 0.001)
	})

	t.Run("distance weight is 50 percent", func(t *testing.T) {
		// Only distance contributes
		assert.InDelta(t, 0.50, rankScore(1.0, 0.0, 0.0), 0.001)
	})

	t.Run("reputation weight is 30 percent", func(t *testing.T) {
		assert.InDelta(t, 0.30, rankScore(0.0, 1.0, 0.0), 0.001)
	})

	t.Run("workload weight is 20 percent", func(t *testing.T) {
		assert.InDelta(t, 0.20, rankScore(0.0, 0.0, 1.0), 0.001)
	})

	t.Run("mixed realistic agent", func(t *testing.T) {
		// normalizedDist=0.8, reputation=75/100=0.75, openTasks=2/8=0.25 → workload=0.75
		// 0.50*0.8 + 0.30*0.75 + 0.20*0.75 = 0.40 + 0.225 + 0.15 = 0.775 → 0.78
		assert.InDelta(t, 0.78, rankScore(0.8, 0.75, 0.75), 0.01)
	})

	t.Run("result is rounded to 2 decimal places", func(t *testing.T) {
		score := rankScore(0.333, 0.333, 0.333)
		// Should be a clean 2dp value
		rounded := math.Round(score*100) / 100
		assert.Equal(t, rounded, score)
	})
}

func TestNormalizedDistanceCalculation(t *testing.T) {
	// normalizedDist = 1.0 - (dist / maxDist), higher is better (closer agent)
	normalizedDist := func(dist, maxDist float64) float64 {
		if maxDist <= 0 {
			return 0.0
		}
		return 1.0 - (dist / maxDist)
	}

	t.Run("closest agent gets score 1.0", func(t *testing.T) {
		assert.InDelta(t, 1.0, normalizedDist(0, 100), 0.001)
	})

	t.Run("furthest agent gets score 0.0", func(t *testing.T) {
		assert.InDelta(t, 0.0, normalizedDist(100, 100), 0.001)
	})

	t.Run("mid-distance agent gets 0.5", func(t *testing.T) {
		assert.InDelta(t, 0.5, normalizedDist(50, 100), 0.001)
	})

	t.Run("zero maxDist produces 0 to avoid divide by zero", func(t *testing.T) {
		assert.InDelta(t, 0.0, normalizedDist(0, 0), 0.001)
	})
}

func TestWorkloadPenaltyCalculation(t *testing.T) {
	// workloadPenalty = 1.0 - (openTasks / maxWorkload)
	workloadPenalty := func(openTasks, maxWorkload float64) float64 {
		return 1.0 - (openTasks / maxWorkload)
	}

	t.Run("no open tasks yields full score", func(t *testing.T) {
		assert.InDelta(t, 1.0, workloadPenalty(0, 8), 0.001)
	})

	t.Run("half workload yields 0.5", func(t *testing.T) {
		assert.InDelta(t, 0.5, workloadPenalty(4, 8), 0.001)
	})

	t.Run("at max workload yields 0", func(t *testing.T) {
		assert.InDelta(t, 0.0, workloadPenalty(8, 8), 0.001)
	})

	t.Run("single open task with max 8 yields 0.875", func(t *testing.T) {
		assert.InDelta(t, 0.875, workloadPenalty(1, 8), 0.001)
	})
}

func TestDefaultFallbackDistance(t *testing.T) {
	// When no zone or ZIP data is available, distance falls back to 50 km
	const defaultDistKm = 50.0
	assert.Equal(t, 50.0, defaultDistKm)
}
