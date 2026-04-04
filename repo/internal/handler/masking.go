package handler

import (
	"dispatchlearn/internal/crypto"
	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/middleware"

	"github.com/gin-gonic/gin"
)

// canViewSensitiveFinance returns true if the user has admin or finance role
func canViewSensitiveFinance(c *gin.Context) bool {
	roles := middleware.GetRoles(c)
	for _, r := range roles {
		if r == "admin" || r == "system_admin" || r == "finance" {
			return true
		}
	}
	return false
}

// canViewSensitiveGrades returns true if the user has admin or instructor role
func canViewSensitiveGrades(c *gin.Context) bool {
	roles := middleware.GetRoles(c)
	for _, r := range roles {
		if r == "admin" || r == "system_admin" || r == "instructor" {
			return true
		}
	}
	return false
}

// MaskedInvoice is a DTO that masks billing_address for non-privileged roles
type MaskedInvoice struct {
	domain.Invoice
	BillingAddress string `json:"billing_address"`
}

func maskInvoice(inv *domain.Invoice, canView bool) MaskedInvoice {
	masked := MaskedInvoice{Invoice: *inv}
	if !canView && inv.BillingAddress != "" {
		masked.BillingAddress = crypto.MaskString(inv.BillingAddress, 3)
	} else {
		masked.BillingAddress = inv.BillingAddress
	}
	// Zero out the embedded field to avoid duplication
	masked.Invoice.BillingAddress = ""
	return masked
}

func maskInvoices(invoices []domain.Invoice, canView bool) []MaskedInvoice {
	result := make([]MaskedInvoice, len(invoices))
	for i := range invoices {
		result[i] = maskInvoice(&invoices[i], canView)
	}
	return result
}

// MaskedPayment is a DTO that masks reference for non-privileged roles
type MaskedPayment struct {
	domain.Payment
	Reference string `json:"reference"`
}

func maskPayment(p *domain.Payment, canView bool) MaskedPayment {
	masked := MaskedPayment{Payment: *p}
	if !canView && p.Reference != "" {
		masked.Reference = crypto.MaskString(p.Reference, 3)
	} else {
		masked.Reference = p.Reference
	}
	masked.Payment.Reference = ""
	return masked
}

func maskPayments(payments []domain.Payment, canView bool) []MaskedPayment {
	result := make([]MaskedPayment, len(payments))
	for i := range payments {
		result[i] = maskPayment(&payments[i], canView)
	}
	return result
}

// MaskedGrade is a DTO that masks numeric_score for non-privileged roles
type MaskedGrade struct {
	domain.Grade
	NumericScore string `json:"numeric_score"`
}

func maskGrade(g *domain.Grade, canView bool) MaskedGrade {
	masked := MaskedGrade{Grade: *g}
	if !canView {
		masked.NumericScore = "****"
	} else {
		masked.NumericScore = g.NumericScore
	}
	masked.Grade.NumericScore = ""
	return masked
}
