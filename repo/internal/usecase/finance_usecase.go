package usecase

import (
	"errors"
	"fmt"
	"time"

	"dispatchlearn/internal/audit"
	"dispatchlearn/internal/crypto"
	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/repository"

	"github.com/google/uuid"
)

type FinanceUseCase struct {
	repo      *repository.FinanceRepository
	audit     *audit.Service
	encryptor *crypto.Encryptor
}

func NewFinanceUseCase(repo *repository.FinanceRepository, audit *audit.Service, enc *crypto.Encryptor) *FinanceUseCase {
	return &FinanceUseCase{repo: repo, audit: audit, encryptor: enc}
}

// Invoices
func (uc *FinanceUseCase) CreateInvoice(tenantID, actorID string, req *domain.CreateInvoiceRequest) (*domain.Invoice, error) {
	taxAmount := req.Subtotal * req.TaxRate
	totalAmount := req.Subtotal + taxAmount

	encAddress, err := uc.encryptor.Encrypt(req.BillingAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt address: %w", err)
	}

	invoiceNo := fmt.Sprintf("INV-%s-%d", tenantID[:8], time.Now().UnixNano())

	invoice := &domain.Invoice{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		OrderID:        req.OrderID,
		InvoiceNo:      invoiceNo,
		Status:         domain.InvoiceDraft,
		Subtotal:       req.Subtotal,
		TaxRate:        req.TaxRate,
		TaxAmount:      taxAmount,
		TotalAmount:    totalAmount,
		BillingAddress: encAddress,
	}

	if err := uc.repo.CreateInvoice(invoice); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "invoice.created",
		EntityType: "invoice",
		EntityID:   invoice.ID,
		AfterState: map[string]interface{}{"invoice_no": invoiceNo, "total": totalAmount},
	})

	return invoice, nil
}

func (uc *FinanceUseCase) GetInvoice(tenantID, id string) (*domain.Invoice, error) {
	return uc.repo.FindInvoiceByID(tenantID, id)
}

func (uc *FinanceUseCase) ListInvoices(tenantID string, page, perPage int) ([]domain.Invoice, int64, error) {
	return uc.repo.ListInvoices(tenantID, page, perPage)
}

func (uc *FinanceUseCase) IssueInvoice(tenantID, actorID, invoiceID string) error {
	invoice, err := uc.repo.FindInvoiceByID(tenantID, invoiceID)
	if err != nil {
		return errors.New("invoice not found")
	}

	if invoice.Status != domain.InvoiceDraft {
		return errors.New("only draft invoices can be issued")
	}

	now := time.Now()
	invoice.IssuedAt = &now
	due := now.AddDate(0, 0, 30)
	invoice.DueAt = &due

	if err := uc.repo.UpdateInvoiceStatus(tenantID, invoiceID, domain.InvoiceIssued); err != nil {
		return err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:    tenantID,
		ActorID:     actorID,
		Action:      "invoice.issued",
		EntityType:  "invoice",
		EntityID:    invoiceID,
		BeforeState: map[string]string{"status": string(domain.InvoiceDraft)},
		AfterState:  map[string]string{"status": string(domain.InvoiceIssued)},
	})

	return nil
}

// Payments (append-only)
func (uc *FinanceUseCase) RecordPayment(tenantID, actorID string, req *domain.CreatePaymentRequest) (*domain.Payment, error) {
	// Validate method
	validMethods := map[string]bool{"cash": true, "check": true, "card_present": true, "house_account": true}
	if !validMethods[req.Method] {
		return nil, errors.New("invalid payment method")
	}

	// Check duplicate
	isDup, err := uc.repo.CheckDuplicatePayment(tenantID, req.OrderID, req.Amount, req.Method)
	if err != nil {
		return nil, err
	}
	if isDup {
		return nil, errors.New("duplicate payment detected within 5 minute window")
	}

	// Encrypt reference
	encRef, err := uc.encryptor.Encrypt(req.Reference)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt reference: %w", err)
	}

	payment := &domain.Payment{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		OrderID:        req.OrderID,
		InvoiceID:      req.InvoiceID,
		Amount:         req.Amount,
		Method:         domain.PaymentMethod(req.Method),
		Reference:      encRef,
		IdempotencyKey: req.IdempotencyKey,
		Status:         "completed",
		ProcessedAt:    time.Now(),
	}

	if err := uc.repo.CreatePayment(payment); err != nil {
		return nil, err
	}

	// Create ledger entry
	ledger := &domain.LedgerEntry{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		OrderID:     req.OrderID,
		InvoiceID:   req.InvoiceID,
		PaymentID:   payment.ID,
		EntryType:   "credit",
		Amount:      req.Amount,
		Description: fmt.Sprintf("Payment received via %s", req.Method),
	}
	uc.repo.CreateLedgerEntry(ledger)

	// Update invoice status based on total payments
	uc.reconcileInvoice(tenantID, req.InvoiceID)

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "payment.recorded",
		EntityType: "payment",
		EntityID:   payment.ID,
		AfterState: map[string]interface{}{"amount": req.Amount, "method": req.Method},
	})

	return payment, nil
}

func (uc *FinanceUseCase) reconcileInvoice(tenantID, invoiceID string) {
	invoice, err := uc.repo.FindInvoiceByID(tenantID, invoiceID)
	if err != nil {
		return
	}

	totalPaid, err := uc.repo.SumPaymentsByInvoice(tenantID, invoiceID)
	if err != nil {
		return
	}

	if totalPaid >= invoice.TotalAmount {
		uc.repo.UpdateInvoiceStatus(tenantID, invoiceID, domain.InvoicePaid)
	} else if totalPaid > 0 {
		uc.repo.UpdateInvoiceStatus(tenantID, invoiceID, domain.InvoicePartial)
	}
}

func (uc *FinanceUseCase) GetPayment(tenantID, id string) (*domain.Payment, error) {
	return uc.repo.FindPaymentByID(tenantID, id)
}

func (uc *FinanceUseCase) ListPaymentsByOrder(tenantID, orderID string) ([]domain.Payment, error) {
	return uc.repo.ListPaymentsByOrder(tenantID, orderID)
}

func (uc *FinanceUseCase) ListPaymentsByInvoice(tenantID, invoiceID string) ([]domain.Payment, error) {
	return uc.repo.ListPaymentsByInvoice(tenantID, invoiceID)
}

// Refunds as reversal entries
func (uc *FinanceUseCase) ProcessRefund(tenantID, actorID string, req *domain.CreateRefundRequest) (*domain.LedgerEntry, error) {
	payment, err := uc.repo.FindPaymentByID(tenantID, req.PaymentID)
	if err != nil {
		return nil, errors.New("original payment not found")
	}

	if req.Amount > payment.Amount {
		return nil, errors.New("refund amount exceeds original payment")
	}

	// Create reversal ledger entry
	reversalEntry := &domain.LedgerEntry{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		OrderID:     payment.OrderID,
		InvoiceID:   payment.InvoiceID,
		PaymentID:   payment.ID,
		EntryType:   "debit",
		Amount:      req.Amount,
		Description: fmt.Sprintf("Refund: %s", req.Reason),
	}

	if err := uc.repo.CreateLedgerEntry(reversalEntry); err != nil {
		return nil, err
	}

	// Find original credit entry and link
	// Create ledger link for traceability
	link := &domain.LedgerLink{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		OriginalEntryID: payment.ID,
		ReversalEntryID: reversalEntry.ID,
		Reason:          req.Reason,
	}
	uc.repo.CreateLedgerLink(link)

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "refund.processed",
		EntityType: "ledger_entry",
		EntityID:   reversalEntry.ID,
		AfterState: map[string]interface{}{"amount": req.Amount, "reason": req.Reason, "payment_id": req.PaymentID},
	})

	return reversalEntry, nil
}

// Ledger
func (uc *FinanceUseCase) ListLedgerEntries(tenantID string, page, perPage int) ([]domain.LedgerEntry, int64, error) {
	return uc.repo.ListLedgerEntries(tenantID, page, perPage)
}

func (uc *FinanceUseCase) ListLedgerEntriesByOrder(tenantID, orderID string) ([]domain.LedgerEntry, error) {
	return uc.repo.ListLedgerEntriesByOrder(tenantID, orderID)
}
