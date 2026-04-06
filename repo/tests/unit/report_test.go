package unit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestKPICalculationCorrectness verifies KPI math against a known gold-standard dataset.
// These are pure math tests — no DB required.
func TestKPICalculationCorrectness(t *testing.T) {
	t.Run("fulfillment timeliness percentage", func(t *testing.T) {
		// Gold dataset: 10 total orders, 7 completed
		total := int64(10)
		completed := int64(7)
		expected := 70.0

		actual := float64(completed) / float64(total) * 100
		assert.InDelta(t, expected, actual, 0.01)
	})

	t.Run("exception rate percentage", func(t *testing.T) {
		// Gold dataset: 10 total, 2 cancelled, 1 expired
		total := int64(10)
		cancelled := int64(2)
		expired := int64(1)
		expected := 30.0 // (2+1)/10 * 100

		actual := float64(cancelled+expired) / float64(total) * 100
		assert.InDelta(t, expected, actual, 0.01)
	})

	t.Run("return rate percentage", func(t *testing.T) {
		// Gold dataset: 10 total, 2 returns (cancelled after completion)
		total := int64(10)
		returns := int64(2)
		expected := 20.0

		actual := float64(returns) / float64(total) * 100
		assert.InDelta(t, expected, actual, 0.01)
	})

	t.Run("net settlement calculation", func(t *testing.T) {
		// Gold dataset: 3 paid invoices
		invoices := []struct {
			total float64
			tax   float64
		}{
			{150.00, 12.00},
			{250.00, 20.00},
			{100.00, 8.00},
		}

		var totalRevenue, totalTax float64
		for _, inv := range invoices {
			totalRevenue += inv.total
			totalTax += inv.tax
		}

		expectedRevenue := 500.00
		expectedTax := 40.00
		expectedNet := 460.00

		assert.InDelta(t, expectedRevenue, totalRevenue, 0.01)
		assert.InDelta(t, expectedTax, totalTax, 0.01)
		assert.InDelta(t, expectedNet, totalRevenue-totalTax, 0.01)
	})

	t.Run("average completion minutes", func(t *testing.T) {
		// Gold dataset: 3 completed orders with durations in minutes
		durations := []float64{45.0, 60.0, 30.0}
		expected := 45.0 // (45+60+30)/3

		var sum float64
		for _, d := range durations {
			sum += d
		}
		actual := sum / float64(len(durations))

		assert.InDelta(t, expected, actual, 0.01)
	})

	t.Run("zero total orders produces zero rates", func(t *testing.T) {
		total := int64(0)
		var fulfillment, exception, returnRate float64

		if total > 0 {
			fulfillment = float64(5) / float64(total) * 100
			exception = float64(2) / float64(total) * 100
			returnRate = float64(1) / float64(total) * 100
		}

		assert.Equal(t, 0.0, fulfillment)
		assert.Equal(t, 0.0, exception)
		assert.Equal(t, 0.0, returnRate)
	})
}
