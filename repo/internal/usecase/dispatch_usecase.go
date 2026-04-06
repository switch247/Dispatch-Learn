package usecase

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"dispatchlearn/internal/audit"
	"dispatchlearn/internal/crypto"
	"dispatchlearn/internal/domain"
	"dispatchlearn/internal/repository"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DispatchUseCase struct {
	repo      *repository.DispatchRepository
	lmsUC     *LMSUseCase
	audit     *audit.Service
	encryptor *crypto.Encryptor
	webhookUC *WebhookUseCase
}

func NewDispatchUseCase(
	repo *repository.DispatchRepository,
	lmsUC *LMSUseCase,
	audit *audit.Service,
	enc *crypto.Encryptor,
	webhookUC *WebhookUseCase,
) *DispatchUseCase {
	return &DispatchUseCase{repo: repo, lmsUC: lmsUC, audit: audit, encryptor: enc, webhookUC: webhookUC}
}

func (uc *DispatchUseCase) CreateOrder(tenantID, actorID string, req *domain.CreateOrderRequest) (*domain.Order, error) {
	// Encrypt address
	encAddress, err := uc.encryptor.Encrypt(req.Address)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt address: %w", err)
	}

	orderNo := fmt.Sprintf("ORD-%s-%d", tenantID[:8], time.Now().UnixNano())

	var twStart, twEnd *time.Time
	if req.TimeWindowStart != "" {
		t, err := time.Parse(time.RFC3339, req.TimeWindowStart)
		if err == nil {
			twStart = &t
		}
	}
	if req.TimeWindowEnd != "" {
		t, err := time.Parse(time.RFC3339, req.TimeWindowEnd)
		if err == nil {
			twEnd = &t
		}
	}

	mode := "grab"
	var assignedAgentID *string
	if req.AssignmentMode == "assigned" {
		mode = "assigned"
		if req.AssignedAgentID == "" {
			return nil, errors.New("assigned_agent_id is required when assignment_mode is 'assigned'")
		}
		// Validate assigned agent exists in the same tenant
		_, err := uc.repo.FindAgentProfile(tenantID, req.AssignedAgentID)
		if err != nil {
			return nil, fmt.Errorf("assigned_agent_id '%s' not found in this tenant", req.AssignedAgentID)
		}
		assignedAgentID = &req.AssignedAgentID
	}

	order := &domain.Order{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		OrderNo:         orderNo,
		Category:        req.Category,
		Description:     req.Description,
		Status:          domain.OrderCreated,
		ZoneID:          req.ZoneID,
		Address:         encAddress,
		ZipCode:         req.ZipCode,
		TimeWindowStart: twStart,
		TimeWindowEnd:   twEnd,
		AssignmentMode:  mode,
		AssignedAgentID: assignedAgentID,
		Priority:        req.Priority,
	}

	if err := uc.repo.CreateOrder(order); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "order.created",
		EntityType: "order",
		EntityID:   order.ID,
		AfterState: map[string]string{"order_no": orderNo, "status": string(order.Status)},
	})

	uc.webhookUC.DispatchEvent(tenantID, "order.created", map[string]interface{}{"order_id": order.ID, "order_no": orderNo})

	return order, nil
}

func (uc *DispatchUseCase) GetOrder(tenantID, id string) (*domain.Order, error) {
	return uc.repo.FindOrderByID(tenantID, id)
}

func (uc *DispatchUseCase) ListOrders(tenantID string, status *domain.OrderStatus, page, perPage int) ([]domain.Order, int64, error) {
	return uc.repo.ListOrders(tenantID, status, page, perPage)
}

func (uc *DispatchUseCase) TransitionOrder(tenantID, actorID, orderID string, newStatus domain.OrderStatus) error {
	order, err := uc.repo.FindOrderByID(tenantID, orderID)
	if err != nil {
		return errors.New("order not found")
	}

	if !order.Status.CanTransitionTo(newStatus) {
		return fmt.Errorf("invalid transition from %s to %s", order.Status, newStatus)
	}

	oldStatus := order.Status
	if err := uc.repo.UpdateOrderStatus(tenantID, orderID, newStatus); err != nil {
		return err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:    tenantID,
		ActorID:     actorID,
		Action:      "order.status_changed",
		EntityType:  "order",
		EntityID:    orderID,
		BeforeState: map[string]string{"status": string(oldStatus)},
		AfterState:  map[string]string{"status": string(newStatus)},
	})

	return nil
}

func (uc *DispatchUseCase) AcceptOrder(tenantID, agentID string, req *domain.AcceptOrderRequest, orderID string) (*domain.DispatchAcceptance, error) {
	// Check workload cap
	openTasks, err := uc.repo.CountOpenTasks(tenantID, agentID)
	if err != nil {
		return nil, err
	}
	if openTasks >= 8 {
		return nil, errors.New("workload cap exceeded (max 8 open tasks)")
	}

	// Check order exists and assignment mode
	order, err := uc.repo.FindOrderByID(tenantID, orderID)
	if err != nil {
		return nil, errors.New("order not found")
	}

	// Assigned-mode: only the pre-assigned agent can accept
	if order.AssignmentMode == "assigned" {
		if order.AssignedAgentID == nil || *order.AssignedAgentID != agentID {
			return nil, errors.New("FORBIDDEN: order is in assigned mode and you are not the designated agent")
		}
	}

	// Check qualification
	qualified, reason := uc.lmsUC.IsAgentQualified(tenantID, agentID, order.Category)
	if !qualified {
		return nil, fmt.Errorf("agent not qualified: %s", reason)
	}

	acceptance := &domain.DispatchAcceptance{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		OrderID:        orderID,
		AgentID:        agentID,
		IdempotencyKey: req.IdempotencyKey,
		AcceptedAt:     time.Now(),
	}

	if err := uc.repo.AcceptOrder(tenantID, acceptance); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || errors.Is(err, gorm.ErrInvalidData) {
			return nil, errors.New("CONFLICT: order already accepted or not available")
		}
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    agentID,
		Action:     "order.accepted",
		EntityType: "dispatch_acceptance",
		EntityID:   acceptance.ID,
		AfterState: map[string]string{"order_id": orderID, "agent_id": agentID},
	})

	uc.webhookUC.DispatchEvent(tenantID, "order.accepted", map[string]interface{}{"order_id": orderID, "agent_id": agentID})

	return acceptance, nil
}

func (uc *DispatchUseCase) AcceptOrderWithOverride(tenantID, actorID, agentID, orderID, overrideReason string, req *domain.AcceptOrderRequest) (*domain.DispatchAcceptance, error) {
	if overrideReason == "" {
		return nil, errors.New("override_reason required")
	}

	acceptance := &domain.DispatchAcceptance{
		BaseModel: domain.BaseModel{
			ID:       uuid.New().String(),
			TenantID: tenantID,
		},
		OrderID:        orderID,
		AgentID:        agentID,
		IdempotencyKey: req.IdempotencyKey,
		AcceptedAt:     time.Now(),
	}

	if err := uc.repo.AcceptOrder(tenantID, acceptance); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "order.accepted.override",
		EntityType: "dispatch_acceptance",
		EntityID:   acceptance.ID,
		AfterState: map[string]string{
			"order_id":        orderID,
			"agent_id":        agentID,
			"override_reason": overrideReason,
		},
	})

	return acceptance, nil
}

// RecommendAgents ranks agents for an order based on distance, reputation, workload
func (uc *DispatchUseCase) RecommendAgents(tenantID, orderID string) ([]domain.RecommendationResponse, error) {
	order, err := uc.repo.FindOrderByID(tenantID, orderID)
	if err != nil {
		return nil, errors.New("order not found")
	}

	agents, err := uc.repo.ListAvailableAgents(tenantID)
	if err != nil {
		return nil, err
	}

	var recommendations []domain.RecommendationResponse

	var maxDist float64
	distances := make(map[string]float64)

	// Calculate distances
	for _, agent := range agents {
		dist := uc.calculateDistance(tenantID, agent.ZoneID, order.ZoneID, agent.ZipCode, order.ZipCode)
		distances[agent.ID] = dist
		if dist > maxDist {
			maxDist = dist
		}
	}

	for _, agent := range agents {
		// Check workload cap
		openTasks, _ := uc.repo.CountOpenTasks(tenantID, agent.UserID)
		if openTasks >= int64(agent.MaxWorkload) {
			continue
		}

		// Check qualification
		qualified, _ := uc.lmsUC.IsAgentQualified(tenantID, agent.UserID, order.Category)

		// Calculate ranking score
		normalizedDist := 0.0
		if maxDist > 0 {
			normalizedDist = 1.0 - (distances[agent.ID] / maxDist) // Lower distance = higher score
		}

		workloadPenalty := 1.0 - (float64(openTasks) / float64(agent.MaxWorkload))
		reputationNorm := agent.ReputationScore / 100.0

		rankingScore := 0.50*normalizedDist + 0.30*reputationNorm + 0.20*workloadPenalty

		recommendations = append(recommendations, domain.RecommendationResponse{
			AgentID:         agent.ID,
			UserID:          agent.UserID,
			Distance:        distances[agent.ID],
			ReputationScore: agent.ReputationScore,
			OpenTasks:       int(openTasks),
			RankingScore:    math.Round(rankingScore*100) / 100,
			IsQualified:     qualified,
		})
	}

	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].RankingScore > recommendations[j].RankingScore
	})

	return recommendations, nil
}

func (uc *DispatchUseCase) calculateDistance(tenantID, fromZoneID, toZoneID, fromZip, toZip string) float64 {
	// Try precomputed distance matrix first
	if fromZoneID != "" && toZoneID != "" {
		dm, err := uc.repo.FindDistance(tenantID, fromZoneID, toZoneID)
		if err == nil {
			return dm.DistanceKm
		}
	}

	// Try ZIP+4 centroids
	if fromZip != "" && toZip != "" {
		fromCentroid, err1 := uc.repo.FindZip4Centroid(tenantID, fromZip)
		toCentroid, err2 := uc.repo.FindZip4Centroid(tenantID, toZip)
		if err1 == nil && err2 == nil {
			return haversine(fromCentroid.Latitude, fromCentroid.Longitude,
				toCentroid.Latitude, toCentroid.Longitude)
		}
	}

	// Fallback: default distance
	return 50.0
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371.0 // Earth radius in km
	dLat := (lat2 - lat1) * math.Pi / 180
	dLon := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLon/2)*math.Sin(dLon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return R * c
}

// CancelExpiredOrders is the single source of truth for order cancellation policy.
// Called by both the background worker and the manual API trigger.
func (uc *DispatchUseCase) CancelExpiredOrders() (expired int64, cancelled int64) {
	expiredOrders, _ := uc.repo.FindAndExpireAvailable()
	for _, order := range expiredOrders {
		uc.audit.Log(audit.LogEntry{
			TenantID:    order.TenantID,
			ActorID:     "system",
			Action:      "order.auto_expired",
			EntityType:  "order",
			EntityID:    order.ID,
			BeforeState: map[string]string{"status": string(domain.OrderAvailable)},
			AfterState:  map[string]string{"status": string(domain.OrderExpired)},
		})
	}
	expired = int64(len(expiredOrders))

	cancelledOrders, _ := uc.repo.FindAndCancelExpiredAccepted()
	for _, order := range cancelledOrders {
		reason := "scheduled window expired (time_window_end passed)"
		if order.TimeWindowEnd == nil && order.TimeWindowStart != nil {
			reason = "scheduled window expired (time_window_start + 2h)"
		} else if order.TimeWindowEnd == nil && order.TimeWindowStart == nil {
			reason = "not started within 2h of acceptance"
		}
		uc.audit.Log(audit.LogEntry{
			TenantID:    order.TenantID,
			ActorID:     "system",
			Action:      "order.auto_cancelled",
			EntityType:  "order",
			EntityID:    order.ID,
			BeforeState: map[string]string{"status": string(domain.OrderAccepted)},
			AfterState:  map[string]string{"status": string(domain.OrderCancelled), "reason": reason},
		})
	}
	cancelled = int64(len(cancelledOrders))
	return
}

// ExpireStaleOrders is kept for backward compatibility with the handler API.
func (uc *DispatchUseCase) ExpireStaleOrders(tenantID string) (int64, error) {
	return uc.repo.ExpireStaleOrders(tenantID)
}

func (uc *DispatchUseCase) CancelStaleAccepted(tenantID string) (int64, error) {
	return uc.repo.CancelStaleAccepted(tenantID)
}

// Service Zones
func (uc *DispatchUseCase) CreateServiceZone(tenantID, actorID string, zone *domain.ServiceZone) (*domain.ServiceZone, error) {
	zone.BaseModel = domain.BaseModel{
		ID:       uuid.New().String(),
		TenantID: tenantID,
	}

	if err := uc.repo.CreateServiceZone(zone); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "zone.created",
		EntityType: "service_zone",
		EntityID:   zone.ID,
		AfterState: zone,
	})

	return zone, nil
}

func (uc *DispatchUseCase) ListServiceZones(tenantID string) ([]domain.ServiceZone, error) {
	return uc.repo.ListServiceZones(tenantID)
}

// Agent Profiles
func (uc *DispatchUseCase) CreateAgentProfile(tenantID, actorID string, profile *domain.AgentProfile) (*domain.AgentProfile, error) {
	profile.BaseModel = domain.BaseModel{
		ID:       uuid.New().String(),
		TenantID: tenantID,
	}

	if err := uc.repo.CreateAgentProfile(profile); err != nil {
		return nil, err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:   tenantID,
		ActorID:    actorID,
		Action:     "agent_profile.created",
		EntityType: "agent_profile",
		EntityID:   profile.ID,
		AfterState: profile,
	})

	return profile, nil
}

func (uc *DispatchUseCase) GetAgentProfile(tenantID, userID string) (*domain.AgentProfile, error) {
	return uc.repo.FindAgentProfile(tenantID, userID)
}

func (uc *DispatchUseCase) UpdateAgentProfile(tenantID, actorID string, profile *domain.AgentProfile) error {
	existing, err := uc.repo.FindAgentProfile(tenantID, profile.UserID)
	if err != nil {
		return err
	}

	uc.audit.Log(audit.LogEntry{
		TenantID:    tenantID,
		ActorID:     actorID,
		Action:      "agent_profile.updated",
		EntityType:  "agent_profile",
		EntityID:    existing.ID,
		BeforeState: existing,
		AfterState:  profile,
	})

	return uc.repo.UpdateAgentProfile(profile)
}

// Reputation Score calculation
func (uc *DispatchUseCase) CalculateReputationScore(tenantID, agentID string) (float64, error) {
	metrics, err := uc.repo.FindAgentMetrics(tenantID, agentID)
	if err != nil {
		return 50.0, nil // Default score
	}

	// Minimum 5 completed orders required
	if metrics.CompletedOrders < 5 {
		return 50.0, nil
	}

	// Score = 50% timeliness + 30% avg_grade + 20% completion_rate
	score := 0.50*metrics.FulfillmentTimeliness +
		0.30*metrics.AverageGrade +
		0.20*metrics.CompletionRate

	// Clamp to 0-100
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return math.Round(score*100) / 100, nil
}
