package repository

import (
	"time"

	"dispatchlearn/internal/domain"

	"gorm.io/gorm"
)

type FinanceRepository struct {
	db *gorm.DB
}

func NewFinanceRepository(db *gorm.DB) *FinanceRepository {
	return &FinanceRepository{db: db}
}

// Invoices
func (r *FinanceRepository) CreateInvoice(invoice *domain.Invoice) error {
	return r.db.Create(invoice).Error
}

func (r *FinanceRepository) FindInvoiceByID(tenantID, id string) (*domain.Invoice, error) {
	var invoice domain.Invoice
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&invoice).Error
	if err != nil {
		return nil, err
	}
	return &invoice, nil
}

func (r *FinanceRepository) ListInvoices(tenantID string, page, perPage int) ([]domain.Invoice, int64, error) {
	var invoices []domain.Invoice
	var total int64

	r.db.Model(&domain.Invoice{}).Where("tenant_id = ?", tenantID).Count(&total)

	err := r.db.Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&invoices).Error
	return invoices, total, err
}

func (r *FinanceRepository) UpdateInvoiceStatus(tenantID, id string, status domain.InvoiceStatus) error {
	return r.db.Model(&domain.Invoice{}).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Update("status", status).Error
}

// Payments (append-only)
func (r *FinanceRepository) CreatePayment(payment *domain.Payment) error {
	return r.db.Create(payment).Error
}

func (r *FinanceRepository) FindPaymentByID(tenantID, id string) (*domain.Payment, error) {
	var payment domain.Payment
	err := r.db.Where("tenant_id = ? AND id = ?", tenantID, id).First(&payment).Error
	if err != nil {
		return nil, err
	}
	return &payment, nil
}

func (r *FinanceRepository) ListPaymentsByOrder(tenantID, orderID string) ([]domain.Payment, error) {
	var payments []domain.Payment
	err := r.db.Where("tenant_id = ? AND order_id = ?", tenantID, orderID).
		Order("created_at ASC").Find(&payments).Error
	return payments, err
}

func (r *FinanceRepository) ListPaymentsByInvoice(tenantID, invoiceID string) ([]domain.Payment, error) {
	var payments []domain.Payment
	err := r.db.Where("tenant_id = ? AND invoice_id = ?", tenantID, invoiceID).
		Order("created_at ASC").Find(&payments).Error
	return payments, err
}

func (r *FinanceRepository) SumPaymentsByInvoice(tenantID, invoiceID string) (float64, error) {
	var result struct{ Total float64 }
	err := r.db.Model(&domain.Payment{}).
		Select("COALESCE(SUM(amount), 0) as total").
		Where("tenant_id = ? AND invoice_id = ? AND status = ?", tenantID, invoiceID, "completed").
		Scan(&result).Error
	return result.Total, err
}

// Duplicate detection: same order_id + amount + method within ±5 minutes
func (r *FinanceRepository) CheckDuplicatePayment(tenantID, orderID string, amount float64, method string) (bool, error) {
	var count int64
	now := time.Now()
	fiveMinAgo := now.Add(-5 * time.Minute)
	fiveMinAhead := now.Add(5 * time.Minute)

	err := r.db.Model(&domain.Payment{}).
		Where("tenant_id = ? AND order_id = ? AND amount = ? AND method = ? AND created_at BETWEEN ? AND ?",
			tenantID, orderID, amount, method, fiveMinAgo, fiveMinAhead).
		Count(&count).Error
	return count > 0, err
}

// Ledger Entries
func (r *FinanceRepository) CreateLedgerEntry(entry *domain.LedgerEntry) error {
	return r.db.Create(entry).Error
}

func (r *FinanceRepository) ListLedgerEntries(tenantID string, page, perPage int) ([]domain.LedgerEntry, int64, error) {
	var entries []domain.LedgerEntry
	var total int64

	r.db.Model(&domain.LedgerEntry{}).Where("tenant_id = ?", tenantID).Count(&total)

	err := r.db.Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Offset((page - 1) * perPage).
		Limit(perPage).
		Find(&entries).Error
	return entries, total, err
}

func (r *FinanceRepository) ListLedgerEntriesByOrder(tenantID, orderID string) ([]domain.LedgerEntry, error) {
	var entries []domain.LedgerEntry
	err := r.db.Where("tenant_id = ? AND order_id = ?", tenantID, orderID).
		Order("created_at ASC").Find(&entries).Error
	return entries, err
}

// Ledger Links
func (r *FinanceRepository) CreateLedgerLink(link *domain.LedgerLink) error {
	return r.db.Create(link).Error
}
