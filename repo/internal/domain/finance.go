package domain

import "time"

type PaymentMethod string

const (
	PaymentCash         PaymentMethod = "cash"
	PaymentCheck        PaymentMethod = "check"
	PaymentCardPresent  PaymentMethod = "card_present"
	PaymentHouseAccount PaymentMethod = "house_account"
)

type InvoiceStatus string

const (
	InvoiceDraft     InvoiceStatus = "DRAFT"
	InvoiceIssued    InvoiceStatus = "ISSUED"
	InvoicePaid      InvoiceStatus = "PAID"
	InvoicePartial   InvoiceStatus = "PARTIAL"
	InvoiceVoided    InvoiceStatus = "VOIDED"
)

type Invoice struct {
	BaseModel
	OrderID        string        `gorm:"type:char(36);not null;index" json:"order_id"`
	InvoiceNo      string        `gorm:"type:varchar(50);uniqueIndex;not null" json:"invoice_no"`
	Status         InvoiceStatus `gorm:"type:varchar(20);not null;default:DRAFT" json:"status"`
	Subtotal       float64       `gorm:"type:decimal(12,2);not null" json:"subtotal"`
	TaxRate        float64       `gorm:"type:decimal(5,4);default:0" json:"tax_rate"`
	TaxAmount      float64       `gorm:"type:decimal(12,2);default:0" json:"tax_amount"`
	TotalAmount    float64       `gorm:"type:decimal(12,2);not null" json:"total_amount"`
	BillingAddress string        `gorm:"type:varchar(500)" json:"billing_address"` // encrypted
	IssuedAt       *time.Time    `json:"issued_at,omitempty"`
	DueAt          *time.Time    `json:"due_at,omitempty"`
}

func (Invoice) TableName() string { return "invoices" }

type Payment struct {
	BaseModel
	OrderID        string        `gorm:"type:char(36);not null;index" json:"order_id"`
	InvoiceID      string        `gorm:"type:char(36);not null;index" json:"invoice_id"`
	Amount         float64       `gorm:"type:decimal(12,2);not null" json:"amount"`
	Method         PaymentMethod `gorm:"type:varchar(20);not null" json:"method"`
	Reference      string        `gorm:"type:varchar(255)" json:"reference"` // encrypted
	IdempotencyKey string        `gorm:"type:varchar(255);uniqueIndex" json:"idempotency_key"`
	Status         string        `gorm:"type:varchar(20);default:'completed'" json:"status"`
	ProcessedAt    time.Time     `json:"processed_at"`
}

func (Payment) TableName() string { return "payments" }

type LedgerEntry struct {
	BaseModel
	OrderID     string  `gorm:"type:char(36);index" json:"order_id"`
	InvoiceID   string  `gorm:"type:char(36);index" json:"invoice_id"`
	PaymentID   string  `gorm:"type:char(36);index" json:"payment_id"`
	EntryType   string  `gorm:"type:enum('debit','credit');not null" json:"entry_type"`
	Amount      float64 `gorm:"type:decimal(12,2);not null" json:"amount"`
	Description string  `gorm:"type:varchar(500)" json:"description"`
	BalanceAfter float64 `gorm:"type:decimal(12,2)" json:"balance_after"`
}

func (LedgerEntry) TableName() string { return "ledger_entries" }

type LedgerLink struct {
	BaseModel
	OriginalEntryID string `gorm:"type:char(36);not null;index" json:"original_entry_id"`
	ReversalEntryID string `gorm:"type:char(36);not null;index" json:"reversal_entry_id"`
	Reason          string `gorm:"type:varchar(500)" json:"reason"`
}

func (LedgerLink) TableName() string { return "ledger_links" }

// Finance request types
type CreateInvoiceRequest struct {
	OrderID        string  `json:"order_id" binding:"required"`
	Subtotal       float64 `json:"subtotal" binding:"required"`
	TaxRate        float64 `json:"tax_rate"`
	BillingAddress string  `json:"billing_address"`
}

type CreatePaymentRequest struct {
	OrderID        string  `json:"order_id" binding:"required"`
	InvoiceID      string  `json:"invoice_id" binding:"required"`
	Amount         float64 `json:"amount" binding:"required"`
	Method         string  `json:"method" binding:"required"`
	Reference      string  `json:"reference"`
	IdempotencyKey string  `json:"idempotency_key" binding:"required"`
}

type CreateRefundRequest struct {
	PaymentID string  `json:"payment_id" binding:"required"`
	Amount    float64 `json:"amount" binding:"required"`
	Reason    string  `json:"reason" binding:"required"`
}
