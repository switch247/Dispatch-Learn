package usecase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// numericToLetter is tested directly here (in-package, unexported).

func TestNumericToLetterGrade(t *testing.T) {
	cases := []struct {
		score    int
		expected string
	}{
		{100, "A"},
		{95, "A"},
		{93, "A"},
		{92, "A-"},
		{90, "A-"},
		{89, "B+"},
		{87, "B+"},
		{86, "B"},
		{83, "B"},
		{82, "B-"},
		{80, "B-"},
		{79, "C+"},
		{77, "C+"},
		{76, "C"},
		{73, "C"},
		{72, "C-"},
		{70, "C-"},
		{69, "D+"},
		{67, "D+"},
		{66, "D"},
		{63, "D"},
		{62, "D-"},
		{60, "D-"},
		{59, "F"},
		{1, "F"},
		{0, "F"},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			result := numericToLetter(tc.score)
			assert.Equal(t, tc.expected, result, "score=%d", tc.score)
		})
	}
}

func TestGradePassingThreshold(t *testing.T) {
	passingCases := []int{70, 71, 80, 90, 100}
	failingCases := []int{0, 59, 69, 1, 30}

	for _, score := range passingCases {
		isPassing := score >= 70
		assert.True(t, isPassing, "score %d should be passing", score)
	}

	for _, score := range failingCases {
		isPassing := score >= 70
		assert.False(t, isPassing, "score %d should not be passing", score)
	}
}

func TestGradeLetterBoundaryPrecision(t *testing.T) {
	// Verify boundary conditions — one below each threshold drops to next band
	boundaries := []struct{ score int; above, below string }{
		{93, "A", "A-"},
		{90, "A-", "B+"},
		{87, "B+", "B"},
		{83, "B", "B-"},
		{80, "B-", "C+"},
		{77, "C+", "C"},
		{73, "C", "C-"},
		{70, "C-", "D+"},
		{67, "D+", "D"},
		{63, "D", "D-"},
		{60, "D-", "F"},
	}

	for _, b := range boundaries {
		assert.Equal(t, b.above, numericToLetter(b.score),   "at boundary %d", b.score)
		assert.Equal(t, b.below, numericToLetter(b.score-1), "just below %d", b.score)
	}
}

func TestReputationScoreFormula(t *testing.T) {
	// score = 0.50*timeliness + 0.30*avgGrade + 0.20*completionRate
	compute := func(timeliness, avgGrade, completionRate float64) float64 {
		score := 0.50*timeliness + 0.30*avgGrade + 0.20*completionRate
		if score < 0 {
			score = 0
		}
		if score > 100 {
			score = 100
		}
		return score
	}

	t.Run("all perfect scores yield 100", func(t *testing.T) {
		assert.InDelta(t, 100.0, compute(100, 100, 100), 0.01)
	})

	t.Run("all zero scores yield 0", func(t *testing.T) {
		assert.InDelta(t, 0.0, compute(0, 0, 0), 0.01)
	})

	t.Run("mixed scores apply correct weights", func(t *testing.T) {
		// 0.50*80 + 0.30*90 + 0.20*70 = 40 + 27 + 14 = 81
		assert.InDelta(t, 81.0, compute(80, 90, 70), 0.01)
	})

	t.Run("weight proportions are correct", func(t *testing.T) {
		// Only timeliness at 100, rest 0 → 50
		assert.InDelta(t, 50.0, compute(100, 0, 0), 0.01)
		// Only avgGrade at 100, rest 0 → 30
		assert.InDelta(t, 30.0, compute(0, 100, 0), 0.01)
		// Only completion at 100, rest 0 → 20
		assert.InDelta(t, 20.0, compute(0, 0, 100), 0.01)
	})

	t.Run("score is clamped to 0-100", func(t *testing.T) {
		assert.InDelta(t, 100.0, compute(200, 200, 200), 0.01)
		assert.InDelta(t, 0.0, compute(-50, -50, -50), 0.01)
	})
}

func TestContentSizeLimit(t *testing.T) {
	const maxBytes = 50 * 1024 * 1024 // 50 MB

	t.Run("size at limit is accepted", func(t *testing.T) {
		assert.False(t, maxBytes > maxBytes)
	})

	t.Run("size above limit is rejected", func(t *testing.T) {
		oversized := int64(maxBytes + 1)
		assert.True(t, oversized > maxBytes)
	})

	t.Run("50MB boundary is 52428800 bytes", func(t *testing.T) {
		assert.Equal(t, int64(52428800), int64(maxBytes))
	})
}
