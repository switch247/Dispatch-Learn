package usecase

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"dispatchlearn/internal/audit"
	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/repository"
	"dispatchlearn/logging"

	"github.com/google/uuid"
)

type ReportUseCase struct {
	sysRepo      *repository.SystemRepository
	finRepo      *repository.FinanceRepository
	dispatchRepo *repository.DispatchRepository
	audit        *audit.Service
	exportDir    string
}

func NewReportUseCase(
	sysRepo *repository.SystemRepository,
	finRepo *repository.FinanceRepository,
	dispatchRepo *repository.DispatchRepository,
	audit *audit.Service,
	exportDir string,
) *ReportUseCase {
	os.MkdirAll(exportDir, 0750)
	return &ReportUseCase{
		sysRepo:      sysRepo,
		finRepo:      finRepo,
		dispatchRepo: dispatchRepo,
		audit:        audit,
		exportDir:    exportDir,
	}
}

type KPIReport struct {
	TenantID              string  `json:"tenant_id"`
	Period                string  `json:"period"`
	TotalOrders           int64   `json:"total_orders"`
	CompletedOrders       int64   `json:"completed_orders"`
	CancelledOrders       int64   `json:"cancelled_orders"`
	ExpiredOrders         int64   `json:"expired_orders"`
	AverageOrderValue     float64 `json:"average_order_value"`
	FulfillmentTimeliness float64 `json:"fulfillment_timeliness_pct"`
	ExceptionRate         float64 `json:"exception_rate_pct"`
	ReturnRate            float64 `json:"return_rate_pct"`
	TotalRevenue          float64 `json:"total_revenue"`
	TotalTaxCollected     float64 `json:"total_tax_collected"`
	NetSettlement         float64 `json:"net_settlement"`
	AvgCompletionMinutes  float64 `json:"avg_completion_minutes"`
	GeneratedAt           string  `json:"generated_at"`
	FilterRegion          string  `json:"filter_region,omitempty"`
	FilterChannel         string  `json:"filter_channel,omitempty"`
}

func (uc *ReportUseCase) GenerateKPIReport(tenantID, actorID, reportType string, params map[string]string) (*domain.Report, error) {
	paramsJSON, _ := json.Marshal(params)

	report := &domain.Report{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		Name:        fmt.Sprintf("KPI Report - %s", time.Now().Format("2006-01-02")),
		ReportType:  reportType,
		Parameters:  string(paramsJSON),
		GeneratedBy: actorID,
		GeneratedAt: time.Now(),
		Status:      "generating",
	}

	if err := uc.sysRepo.CreateReport(report); err != nil {
		return nil, err
	}

	// Generate report data
	kpi, err := uc.calculateKPIs(tenantID, params)
	if err != nil {
		report.Status = "failed"
		uc.sysRepo.UpdateReport(report)
		return nil, err
	}

	// Export to CSV
	filename := fmt.Sprintf("kpi_%s_%s.csv", tenantID[:8], time.Now().Format("20060102_150405"))
	filePath := filepath.Join(uc.exportDir, filename)

	if err := uc.exportToCSV(filePath, kpi); err != nil {
		report.Status = "failed"
		uc.sysRepo.UpdateReport(report)
		return nil, err
	}

	// Generate checksum
	checksum, err := uc.fileChecksum(filePath)
	if err != nil {
		return nil, err
	}

	// Write checksum file
	checksumPath := filePath + ".sha256"
	os.WriteFile(checksumPath, []byte(checksum+"  "+filename), 0640)

	report.FilePath = filePath
	report.FileChecksum = checksum
	report.Status = "completed"
	uc.sysRepo.UpdateReport(report)

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "report.generated",
		EntityType: "report",
		EntityID:   report.ID,
		AfterState: map[string]string{"file": filename, "checksum": checksum},
	})

	logging.Info("report", "generate", fmt.Sprintf("Report generated: %s (checksum: %s)", filename, checksum[:16]))

	return report, nil
}

func (uc *ReportUseCase) calculateKPIs(tenantID string, params map[string]string) (*KPIReport, error) {
	kpi := &KPIReport{
		TenantID:    tenantID,
		Period:      time.Now().Format("2006-01"),
		GeneratedAt: time.Now().Format(time.RFC3339),
	}

	// Apply filter dimensions from params
	if region, ok := params["region"]; ok && region != "" {
		kpi.FilterRegion = region
	}
	if channel, ok := params["channel"]; ok && channel != "" {
		kpi.FilterChannel = channel
	}

	// Count orders by status
	allOrders, totalCount, _ := uc.dispatchRepo.ListOrders(tenantID, nil, 1, 1)
	_ = allOrders
	kpi.TotalOrders = totalCount

	completed := domain.OrderCompleted
	_, completedCount, _ := uc.dispatchRepo.ListOrders(tenantID, &completed, 1, 1)
	kpi.CompletedOrders = completedCount

	cancelled := domain.OrderCancelled
	_, cancelledCount, _ := uc.dispatchRepo.ListOrders(tenantID, &cancelled, 1, 1)
	kpi.CancelledOrders = cancelledCount

	expired := domain.OrderExpired
	_, expiredCount, _ := uc.dispatchRepo.ListOrders(tenantID, &expired, 1, 1)
	kpi.ExpiredOrders = expiredCount

	if totalCount > 0 {
		kpi.ExceptionRate = float64(cancelledCount+expiredCount) / float64(totalCount) * 100
		kpi.FulfillmentTimeliness = float64(completedCount) / float64(totalCount) * 100
	}

	// Financial KPIs - aggregate from invoices
	invoices, _, _ := uc.finRepo.ListInvoices(tenantID, 1, 10000)
	var totalRevenue, totalTax float64
	for _, inv := range invoices {
		if inv.Status == domain.InvoicePaid || inv.Status == domain.InvoicePartial {
			totalRevenue += inv.TotalAmount
			totalTax += inv.TaxAmount
		}
	}
	kpi.TotalRevenue = totalRevenue
	kpi.TotalTaxCollected = totalTax
	kpi.NetSettlement = totalRevenue - totalTax

	// Efficiency KPIs - average delivery duration (window_start to completed_at)
	if completedCount > 0 {
		avgMinutes, err := uc.dispatchRepo.AvgCompletionMinutes(tenantID)
		if err == nil {
			kpi.AvgCompletionMinutes = avgMinutes
		}
	}

	// Returns rate (return-tagged cancelled orders as proportion of total)
	returnsCount, _ := uc.dispatchRepo.CountReturnedOrders(tenantID)
	if totalCount > 0 {
		kpi.ReturnRate = float64(returnsCount) / float64(totalCount) * 100
	}

	return kpi, nil
}

func (uc *ReportUseCase) exportToCSV(filePath string, kpi *KPIReport) error {
	f, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	defer w.Flush()

	// Header
	w.Write([]string{"metric", "value"})
	w.Write([]string{"tenant_id", kpi.TenantID})
	w.Write([]string{"period", kpi.Period})
	// Filter dimensions
	if kpi.FilterRegion != "" {
		w.Write([]string{"filter_region", kpi.FilterRegion})
	}
	if kpi.FilterChannel != "" {
		w.Write([]string{"filter_channel", kpi.FilterChannel})
	}
	w.Write([]string{"total_orders", fmt.Sprintf("%d", kpi.TotalOrders)})
	w.Write([]string{"completed_orders", fmt.Sprintf("%d", kpi.CompletedOrders)})
	w.Write([]string{"cancelled_orders", fmt.Sprintf("%d", kpi.CancelledOrders)})
	w.Write([]string{"expired_orders", fmt.Sprintf("%d", kpi.ExpiredOrders)})
	w.Write([]string{"fulfillment_timeliness_pct", fmt.Sprintf("%.2f", kpi.FulfillmentTimeliness)})
	w.Write([]string{"exception_rate_pct", fmt.Sprintf("%.2f", kpi.ExceptionRate)})
	w.Write([]string{"return_rate_pct", fmt.Sprintf("%.2f", kpi.ReturnRate)})
	w.Write([]string{"total_revenue", fmt.Sprintf("%.2f", kpi.TotalRevenue)})
	w.Write([]string{"total_tax_collected", fmt.Sprintf("%.2f", kpi.TotalTaxCollected)})
	w.Write([]string{"net_settlement", fmt.Sprintf("%.2f", kpi.NetSettlement)})
	w.Write([]string{"avg_completion_minutes", fmt.Sprintf("%.2f", kpi.AvgCompletionMinutes)})
	w.Write([]string{"generated_at", kpi.GeneratedAt})

	return nil
}

func (uc *ReportUseCase) fileChecksum(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash), nil
}

func (uc *ReportUseCase) GetReport(tenantID, id string) (*domain.Report, error) {
	return uc.sysRepo.FindReportByID(tenantID, id)
}

func (uc *ReportUseCase) ListReports(tenantID string, page, perPage int) ([]domain.Report, int64, error) {
	return uc.sysRepo.ListReports(tenantID, page, perPage)
}

// Audit log access
func (uc *ReportUseCase) ListAuditLogs(tenantID, entityType string, page, perPage int) ([]domain.AuditLog, int64, error) {
	return uc.sysRepo.ListAuditLogs(tenantID, entityType, page, perPage)
}

func (uc *ReportUseCase) VerifyAuditChain(tenantID string) (bool, error) {
	return uc.audit.VerifyChain(tenantID)
}

// Config changes
func (uc *ReportUseCase) ListConfigChanges(tenantID string, page, perPage int) ([]domain.ConfigChange, int64, error) {
	return uc.sysRepo.ListConfigChanges(tenantID, page, perPage)
}

// GenerateScheduledReport can be called by an external cron-like trigger
// to generate a KPI report with default parameters.
func (uc *ReportUseCase) GenerateScheduledReport(tenantID, actorID string) (*domain.Report, error) {
	defaultParams := map[string]string{
		"scheduled": "true",
	}
	return uc.GenerateKPIReport(tenantID, actorID, "scheduled_kpi", defaultParams)
}

// Quota overrides
func (uc *ReportUseCase) GetQuotaOverride(tenantID string) (*domain.QuotaOverride, error) {
	return uc.sysRepo.FindQuotaOverride(tenantID)
}

func (uc *ReportUseCase) SetQuotaOverride(tenantID, actorID string, override *domain.QuotaOverride) error {
	// Fetch existing override to track the config change
	existing, _ := uc.sysRepo.FindQuotaOverride(tenantID)

	oldValue := "none"
	if existing != nil {
		oldValue = fmt.Sprintf("rpm=%d,burst=%d,webhook_daily_limit=%d", existing.RPM, existing.Burst, existing.WebhookDailyLimit)
	}
	newValue := fmt.Sprintf("rpm=%d,burst=%d,webhook_daily_limit=%d", override.RPM, override.Burst, override.WebhookDailyLimit)

	override.BaseModel = domain.BaseModel{
		ID:       uuid.New().String(),
		TenantID: tenantID,
	}

	if err := uc.sysRepo.UpsertQuotaOverride(override); err != nil {
		return err
	}

	// Record config change with old and new values
	uc.sysRepo.CreateConfigChange(&domain.ConfigChange{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		ChangedBy: actorID,
		ConfigKey: "quota_override",
		OldValue:  oldValue,
		NewValue:  newValue,
		Reason:    "quota override updated via API",
	})

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "quota.override.set",
		EntityType: "quota_override",
		EntityID:   override.ID,
		AfterState: override,
	})

	return nil
}
