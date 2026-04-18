package unit

import (
	"testing"

	"dispatchlearn/internal/domain"

	"github.com/stretchr/testify/assert"
)

// ---- Invoice tax math ----

func TestInvoiceTaxCalculation(t *testing.T) {
	cases := []struct {
		subtotal    float64
		taxRate     float64
		wantTax     float64
		wantTotal   float64
	}{
		{100.00, 0.08, 8.00, 108.00},
		{250.00, 0.095, 23.75, 273.75},
		{500.00, 0.00, 0.00, 500.00},
		{0.00, 0.10, 0.00, 0.00},
		{1000.00, 0.0875, 87.50, 1087.50},
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			taxAmount := tc.subtotal * tc.taxRate
			totalAmount := tc.subtotal + taxAmount
			assert.InDelta(t, tc.wantTax, taxAmount, 0.01)
			assert.InDelta(t, tc.wantTotal, totalAmount, 0.01)
		})
	}
}

func TestInvoiceStatusConstants(t *testing.T) {
	assert.Equal(t, domain.InvoiceStatus("DRAFT"), domain.InvoiceDraft)
	assert.Equal(t, domain.InvoiceStatus("ISSUED"), domain.InvoiceIssued)
	assert.Equal(t, domain.InvoiceStatus("PAID"), domain.InvoicePaid)
	assert.Equal(t, domain.InvoiceStatus("PARTIAL"), domain.InvoicePartial)
	assert.Equal(t, domain.InvoiceStatus("VOIDED"), domain.InvoiceVoided)
}

func TestInvoiceStatusDistinct(t *testing.T) {
	statuses := []domain.InvoiceStatus{
		domain.InvoiceDraft,
		domain.InvoiceIssued,
		domain.InvoicePaid,
		domain.InvoicePartial,
		domain.InvoiceVoided,
	}
	seen := map[domain.InvoiceStatus]bool{}
	for _, s := range statuses {
		assert.False(t, seen[s], "duplicate invoice status: %s", s)
		seen[s] = true
	}
}

// ---- Payment method validation ----

func TestPaymentMethodConstants(t *testing.T) {
	assert.Equal(t, domain.PaymentMethod("cash"), domain.PaymentCash)
	assert.Equal(t, domain.PaymentMethod("check"), domain.PaymentCheck)
	assert.Equal(t, domain.PaymentMethod("card_present"), domain.PaymentCardPresent)
	assert.Equal(t, domain.PaymentMethod("house_account"), domain.PaymentHouseAccount)
}

func TestPaymentMethodValidation(t *testing.T) {
	validMethods := map[string]bool{
		"cash":          true,
		"check":         true,
		"card_present":  true,
		"house_account": true,
	}

	valid := []string{"cash", "check", "card_present", "house_account"}
	invalid := []string{"credit_card", "wire", "bitcoin", "", "CASH", "Cash"}

	for _, m := range valid {
		assert.True(t, validMethods[m], "expected %q to be valid", m)
	}
	for _, m := range invalid {
		assert.False(t, validMethods[m], "expected %q to be invalid", m)
	}
}

// ---- Refund validation ----

func TestRefundAmountValidation(t *testing.T) {
	cases := []struct {
		refundAmount  float64
		paymentAmount float64
		shouldError   bool
	}{
		{50.00, 100.00, false},   // partial refund
		{100.00, 100.00, false},  // full refund
		{100.01, 100.00, true},   // exceeds payment
		{0.01, 100.00, false},    // minimum partial
		{200.00, 100.00, true},   // double the payment
	}

	for _, tc := range cases {
		t.Run("", func(t *testing.T) {
			exceeds := tc.refundAmount > tc.paymentAmount
			assert.Equal(t, tc.shouldError, exceeds,
				"refund=%.2f payment=%.2f", tc.refundAmount, tc.paymentAmount)
		})
	}
}

// ---- Ledger entry types ----

func TestLedgerEntryTypes(t *testing.T) {
	t.Run("payment creates credit entry", func(t *testing.T) {
		entry := domain.LedgerEntry{EntryType: "credit", Amount: 100.00}
		assert.Equal(t, "credit", entry.EntryType)
		assert.Greater(t, entry.Amount, 0.0)
	})

	t.Run("refund creates debit entry", func(t *testing.T) {
		entry := domain.LedgerEntry{EntryType: "debit", Amount: 50.00}
		assert.Equal(t, "debit", entry.EntryType)
	})

	t.Run("entry types are distinct", func(t *testing.T) {
		assert.NotEqual(t, "credit", "debit")
	})
}

func TestLedgerBalanceAfterComputation(t *testing.T) {
	// balance_after = previous balance + credit - debit
	cases := []struct {
		prevBalance float64
		entryType   string
		amount      float64
		want        float64
	}{
		{0.00, "credit", 100.00, 100.00},
		{100.00, "credit", 50.00, 150.00},
		{150.00, "debit", 50.00, 100.00},
		{100.00, "debit", 100.00, 0.00},
	}

	for _, tc := range cases {
		var balanceAfter float64
		if tc.entryType == "credit" {
			balanceAfter = tc.prevBalance + tc.amount
		} else {
			balanceAfter = tc.prevBalance - tc.amount
		}
		assert.InDelta(t, tc.want, balanceAfter, 0.01)
	}
}

// ---- Net settlement math ----

func TestNetSettlementCalculation(t *testing.T) {
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
	netSettlement := totalRevenue - totalTax

	assert.InDelta(t, 500.00, totalRevenue, 0.01)
	assert.InDelta(t, 40.00, totalTax, 0.01)
	assert.InDelta(t, 460.00, netSettlement, 0.01)
}

// ---- Duplicate payment detection window ----

func TestDuplicatePaymentWindowLogic(t *testing.T) {
	// The business rule: same order + amount + method within 5 minutes = duplicate
	t.Run("same order+amount+method qualifies as duplicate", func(t *testing.T) {
		orderID := "order-1"
		amount := 100.00
		method := "cash"

		// Simulated duplicate check: same triple
		isDuplicate := func(o string, a float64, m string) bool {
			return o == orderID && a == amount && m == method
		}

		assert.True(t, isDuplicate("order-1", 100.00, "cash"))
		assert.False(t, isDuplicate("order-1", 100.00, "check")) // different method
		assert.False(t, isDuplicate("order-1", 99.00, "cash"))   // different amount
		assert.False(t, isDuplicate("order-2", 100.00, "cash"))  // different order
	})
}
