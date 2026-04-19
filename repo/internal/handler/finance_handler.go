package handler

import (
	"net/http"
	"strings"

	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/middleware"
	"dispatchlearn/internal/usecase"

	"github.com/gin-gonic/gin"
)

type FinanceHandler struct {
	uc *usecase.FinanceUseCase
}

func NewFinanceHandler(uc *usecase.FinanceUseCase) *FinanceHandler {
	return &FinanceHandler{uc: uc}
}

// Invoices
func (h *FinanceHandler) CreateInvoice(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var req domain.CreateInvoiceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	invoice, err := h.uc.CreateInvoice(tenantID, actorID, &req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}

	respondCreated(c, maskInvoice(invoice, canViewSensitiveFinance(c)))
}

func (h *FinanceHandler) GetInvoice(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	id := c.Param("id")

	invoice, err := h.uc.GetInvoice(tenantID, id)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "invoice not found")
		return
	}

	respondOK(c, maskInvoice(invoice, canViewSensitiveFinance(c)))
}

func (h *FinanceHandler) ListInvoices(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	page, perPage := getPagination(c)

	invoices, total, err := h.uc.ListInvoices(tenantID, page, perPage)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondList(c, maskInvoices(invoices, canViewSensitiveFinance(c)), page, perPage, total)
}

func (h *FinanceHandler) IssueInvoice(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)
	id := c.Param("id")

	invoice, err := h.uc.IssueInvoice(tenantID, actorID, id)
	if err != nil {
		respondError(c, http.StatusBadRequest, "ISSUE_FAILED", err.Error())
		return
	}

	respondOK(c, maskInvoice(invoice, canViewSensitiveFinance(c)))
}

// Payments
func (h *FinanceHandler) RecordPayment(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var req domain.CreatePaymentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	payment, err := h.uc.RecordPayment(tenantID, actorID, &req)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") {
			respondConflict(c, err.Error())
			return
		}
		respondError(c, http.StatusBadRequest, "PAYMENT_FAILED", err.Error())
		return
	}

	respondCreated(c, maskPayment(payment, canViewSensitiveFinance(c)))
}

func (h *FinanceHandler) GetPayment(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	id := c.Param("id")

	payment, err := h.uc.GetPayment(tenantID, id)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "payment not found")
		return
	}

	respondOK(c, maskPayment(payment, canViewSensitiveFinance(c)))
}

func (h *FinanceHandler) ListPaymentsByOrder(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	orderID := c.Param("id")

	payments, err := h.uc.ListPaymentsByOrder(tenantID, orderID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondOK(c, maskPayments(payments, canViewSensitiveFinance(c)))
}

func (h *FinanceHandler) ListPaymentsByInvoice(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	invoiceID := c.Param("id")

	payments, err := h.uc.ListPaymentsByInvoice(tenantID, invoiceID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondOK(c, maskPayments(payments, canViewSensitiveFinance(c)))
}

// Refunds
func (h *FinanceHandler) ProcessRefund(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var req domain.CreateRefundRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	entry, err := h.uc.ProcessRefund(tenantID, actorID, &req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "REFUND_FAILED", err.Error())
		return
	}

	respondCreated(c, entry)
}

// Ledger
func (h *FinanceHandler) ListLedgerEntries(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	page, perPage := getPagination(c)

	entries, total, err := h.uc.ListLedgerEntries(tenantID, page, perPage)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondList(c, entries, page, perPage, total)
}

func (h *FinanceHandler) ListLedgerEntriesByOrder(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	orderID := c.Param("id")

	entries, err := h.uc.ListLedgerEntriesByOrder(tenantID, orderID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondOK(c, entries)
}
