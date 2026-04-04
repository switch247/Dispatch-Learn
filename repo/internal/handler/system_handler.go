package handler

import (
	"net/http"

	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/middleware"
	"dispatchlearn/internal/usecase"

	"github.com/gin-gonic/gin"
)

type SystemHandler struct {
	reportUC  *usecase.ReportUseCase
	webhookUC *usecase.WebhookUseCase
}

func NewSystemHandler(reportUC *usecase.ReportUseCase, webhookUC *usecase.WebhookUseCase) *SystemHandler {
	return &SystemHandler{reportUC: reportUC, webhookUC: webhookUC}
}

// Audit Logs
func (h *SystemHandler) ListAuditLogs(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	entityType := c.Query("entity_type")
	page, perPage := getPagination(c)

	logs, total, err := h.reportUC.ListAuditLogs(tenantID, entityType, page, perPage)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondList(c, logs, page, perPage, total)
}

func (h *SystemHandler) VerifyAuditChain(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	valid, err := h.reportUC.VerifyAuditChain(tenantID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "VERIFY_FAILED", err.Error())
		return
	}

	respondOK(c, gin.H{"valid": valid})
}

// Config Changes
func (h *SystemHandler) ListConfigChanges(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	page, perPage := getPagination(c)

	changes, total, err := h.reportUC.ListConfigChanges(tenantID, page, perPage)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondList(c, changes, page, perPage, total)
}

// Reports
func (h *SystemHandler) GenerateReport(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var req struct {
		ReportType string            `json:"report_type" binding:"required"`
		Parameters map[string]string `json:"parameters"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	report, err := h.reportUC.GenerateKPIReport(tenantID, actorID, req.ReportType, req.Parameters)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "GENERATE_FAILED", err.Error())
		return
	}

	respondCreated(c, report)
}

func (h *SystemHandler) GetReport(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	id := c.Param("id")

	report, err := h.reportUC.GetReport(tenantID, id)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "report not found")
		return
	}

	respondOK(c, report)
}

func (h *SystemHandler) ListReports(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	page, perPage := getPagination(c)

	reports, total, err := h.reportUC.ListReports(tenantID, page, perPage)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondList(c, reports, page, perPage, total)
}

// Webhooks
func (h *SystemHandler) CreateWebhookSubscription(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var req domain.CreateWebhookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	sub, err := h.webhookUC.CreateSubscription(tenantID, actorID, &req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}

	respondCreated(c, sub)
}

func (h *SystemHandler) ListWebhookSubscriptions(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	subs, err := h.webhookUC.ListSubscriptions(tenantID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondOK(c, subs)
}

func (h *SystemHandler) GetWebhookSubscription(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	id := c.Param("id")

	sub, err := h.webhookUC.GetSubscription(tenantID, id)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "subscription not found")
		return
	}

	respondOK(c, sub)
}

func (h *SystemHandler) ListDeadLetters(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	deliveries, err := h.webhookUC.ListDeadLetters(tenantID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondOK(c, deliveries)
}

// Quota Overrides
func (h *SystemHandler) GetQuotaOverride(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	override, err := h.reportUC.GetQuotaOverride(tenantID)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "no quota override found")
		return
	}

	respondOK(c, override)
}

func (h *SystemHandler) SetQuotaOverride(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var override domain.QuotaOverride
	if err := c.ShouldBindJSON(&override); err != nil {
		respondValidation(c, err.Error())
		return
	}

	if err := h.reportUC.SetQuotaOverride(tenantID, actorID, &override); err != nil {
		respondError(c, http.StatusBadRequest, "SET_FAILED", err.Error())
		return
	}

	respondOK(c, gin.H{"message": "quota override set"})
}

// Health check
func HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok", "service": "dispatchlearn"})
}
