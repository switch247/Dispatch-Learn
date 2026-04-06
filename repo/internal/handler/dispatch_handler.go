package handler

import (
	"net/http"
	"strings"

	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/middleware"
	"dispatchlearn/internal/usecase"

	"github.com/gin-gonic/gin"
)

type DispatchHandler struct {
	uc *usecase.DispatchUseCase
}

func NewDispatchHandler(uc *usecase.DispatchUseCase) *DispatchHandler {
	return &DispatchHandler{uc: uc}
}

func (h *DispatchHandler) CreateOrder(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var req domain.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	order, err := h.uc.CreateOrder(tenantID, actorID, &req)
	if err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}

	respondCreated(c, order)
}

func (h *DispatchHandler) GetOrder(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	id := c.Param("id")

	order, err := h.uc.GetOrder(tenantID, id)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "order not found")
		return
	}

	respondOK(c, order)
}

func (h *DispatchHandler) ListOrders(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	page, perPage := getPagination(c)

	var status *domain.OrderStatus
	if s := c.Query("status"); s != "" {
		os := domain.OrderStatus(s)
		status = &os
	}

	orders, total, err := h.uc.ListOrders(tenantID, status, page, perPage)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondList(c, orders, page, perPage, total)
}

func (h *DispatchHandler) TransitionOrder(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)
	orderID := c.Param("id")

	var req struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	if err := h.uc.TransitionOrder(tenantID, actorID, orderID, domain.OrderStatus(req.Status)); err != nil {
		if strings.Contains(err.Error(), "invalid transition") {
			respondValidation(c, err.Error())
			return
		}
		respondError(c, http.StatusBadRequest, "TRANSITION_FAILED", err.Error())
		return
	}

	respondOK(c, gin.H{"message": "order status updated"})
}

func (h *DispatchHandler) AcceptOrder(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	agentID := middleware.GetUserID(c)
	orderID := c.Param("id")

	var req domain.AcceptOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondValidation(c, err.Error())
		return
	}

	acceptance, err := h.uc.AcceptOrder(tenantID, agentID, &req, orderID)
	if err != nil {
		if strings.Contains(err.Error(), "CONFLICT") {
			respondConflict(c, err.Error())
			return
		}
		if strings.Contains(err.Error(), "FORBIDDEN") {
			respondError(c, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if strings.Contains(err.Error(), "not qualified") {
			respondError(c, http.StatusForbidden, "NOT_QUALIFIED", err.Error())
			return
		}
		if strings.Contains(err.Error(), "workload cap") {
			respondValidation(c, err.Error())
			return
		}
		respondError(c, http.StatusBadRequest, "ACCEPT_FAILED", err.Error())
		return
	}

	respondCreated(c, acceptance)
}

func (h *DispatchHandler) RecommendAgents(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	orderID := c.Param("id")

	recommendations, err := h.uc.RecommendAgents(tenantID, orderID)
	if err != nil {
		respondError(c, http.StatusBadRequest, "RECOMMEND_FAILED", err.Error())
		return
	}

	respondOK(c, recommendations)
}

// Service Zones
func (h *DispatchHandler) CreateServiceZone(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var zone domain.ServiceZone
	if err := c.ShouldBindJSON(&zone); err != nil {
		respondValidation(c, err.Error())
		return
	}

	result, err := h.uc.CreateServiceZone(tenantID, actorID, &zone)
	if err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}

	respondCreated(c, result)
}

func (h *DispatchHandler) ListServiceZones(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)

	zones, err := h.uc.ListServiceZones(tenantID)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	respondOK(c, zones)
}

// Agent Profiles
func (h *DispatchHandler) CreateAgentProfile(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	actorID := middleware.GetUserID(c)

	var profile domain.AgentProfile
	if err := c.ShouldBindJSON(&profile); err != nil {
		respondValidation(c, err.Error())
		return
	}

	result, err := h.uc.CreateAgentProfile(tenantID, actorID, &profile)
	if err != nil {
		respondError(c, http.StatusBadRequest, "CREATE_FAILED", err.Error())
		return
	}

	respondCreated(c, result)
}

func (h *DispatchHandler) GetAgentProfile(c *gin.Context) {
	tenantID := middleware.GetTenantID(c)
	userID := c.Param("user_id")

	profile, err := h.uc.GetAgentProfile(tenantID, userID)
	if err != nil {
		respondError(c, http.StatusNotFound, "NOT_FOUND", "agent profile not found")
		return
	}

	respondOK(c, profile)
}

func (h *DispatchHandler) ExpireStaleOrders(c *gin.Context) {
	expired, cancelled := h.uc.CancelExpiredOrders()
	respondOK(c, gin.H{"expired": expired, "cancelled": cancelled})
}
