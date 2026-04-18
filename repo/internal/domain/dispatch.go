package domain

import "time"

type OrderStatus string

const (
	OrderCreated    OrderStatus = "CREATED"
	OrderAvailable  OrderStatus = "AVAILABLE"
	OrderAccepted   OrderStatus = "ACCEPTED"
	OrderInProgress OrderStatus = "IN_PROGRESS"
	OrderCompleted  OrderStatus = "COMPLETED"
	OrderExpired    OrderStatus = "EXPIRED"
	OrderCancelled  OrderStatus = "CANCELLED"
	OrderReturned   OrderStatus = "RETURNED"
)

var ValidTransitions = map[OrderStatus][]OrderStatus{
	OrderCreated:    {OrderAvailable},
	OrderAvailable:  {OrderAccepted, OrderExpired},
	OrderAccepted:   {OrderInProgress, OrderCancelled, OrderReturned},
	OrderInProgress: {OrderCompleted, OrderReturned},
	OrderCancelled:  {OrderReturned},
}

func (s OrderStatus) CanTransitionTo(target OrderStatus) bool {
	for _, valid := range ValidTransitions[s] {
		if valid == target {
			return true
		}
	}
	return false
}

type Order struct {
	BaseModel
	OrderNo         string      `gorm:"type:varchar(50);uniqueIndex;not null" json:"order_no"`
	Category        string      `gorm:"type:varchar(100);not null" json:"category"`
	Description     string      `gorm:"type:text" json:"description"`
	Status          OrderStatus `gorm:"type:varchar(20);not null;default:CREATED;index" json:"status"`
	ZoneID          string      `gorm:"type:char(36);index" json:"zone_id"`
	Address         string      `gorm:"type:varchar(500)" json:"address"` // encrypted
	ZipCode         string      `gorm:"type:varchar(10)" json:"zip_code"`
	TimeWindowStart *time.Time  `json:"time_window_start"`
	TimeWindowEnd   *time.Time  `json:"time_window_end"`
	AssignedAgentID *string     `gorm:"type:char(36);index" json:"assigned_agent_id,omitempty"`
	AssignmentMode  string      `gorm:"type:enum('grab','assigned');default:'grab'" json:"assignment_mode"`
	Priority        int         `gorm:"default:0" json:"priority"`
	AvailableAt     *time.Time  `json:"available_at,omitempty"`
	AcceptedAt      *time.Time  `json:"accepted_at,omitempty"`
	CompletedAt     *time.Time  `json:"completed_at,omitempty"`
	OverrideReason  string      `gorm:"type:text" json:"override_reason,omitempty"`
}

func (Order) TableName() string { return "orders" }

type DispatchAcceptance struct {
	BaseModel
	OrderID        string    `gorm:"type:char(36);uniqueIndex;not null" json:"order_id"`
	AgentID        string    `gorm:"type:char(36);not null;index" json:"agent_id"`
	IdempotencyKey string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"idempotency_key"`
	AcceptedAt     time.Time `json:"accepted_at"`
}

func (DispatchAcceptance) TableName() string { return "dispatch_acceptances" }

type ServiceZone struct {
	BaseModel
	Name        string  `gorm:"type:varchar(255);not null" json:"name"`
	ZipCodes    string  `gorm:"type:text" json:"zip_codes"` // comma-separated
	CentroidLat float64 `gorm:"type:decimal(10,7)" json:"centroid_lat"`
	CentroidLng float64 `gorm:"type:decimal(10,7)" json:"centroid_lng"`
}

func (ServiceZone) TableName() string { return "service_zones" }

type DistanceMatrix struct {
	BaseModel
	FromZoneID string  `gorm:"type:char(36);not null;index" json:"from_zone_id"`
	ToZoneID   string  `gorm:"type:char(36);not null;index" json:"to_zone_id"`
	DistanceKm float64 `gorm:"type:decimal(10,2)" json:"distance_km"`
	Source     string  `gorm:"type:enum('zip4_centroid','precomputed','manual');not null" json:"source"`
}

func (DistanceMatrix) TableName() string { return "distance_matrix" }

type Zip4Centroid struct {
	BaseModel
	ZipCode       string  `gorm:"type:varchar(10);uniqueIndex:idx_zip_tenant,priority:2;not null" json:"zip_code"`
	Latitude      float64 `gorm:"type:decimal(10,7)" json:"latitude"`
	Longitude     float64 `gorm:"type:decimal(10,7)" json:"longitude"`
	SourceVersion string  `gorm:"type:varchar(50)" json:"source_version"`
}

func (Zip4Centroid) TableName() string { return "zip4_centroids" }

type AgentProfile struct {
	BaseModel
	UserID          string  `gorm:"type:char(36);uniqueIndex:idx_agent_tenant,priority:2;not null" json:"user_id"`
	ZoneID          string  `gorm:"type:char(36);index" json:"zone_id"`
	ZipCode         string  `gorm:"type:varchar(10)" json:"zip_code"`
	IsAvailable     bool    `gorm:"default:true" json:"is_available"`
	MaxWorkload     int     `gorm:"default:8" json:"max_workload"`
	ReputationScore float64 `gorm:"type:decimal(5,2);default:50.00" json:"reputation_score"`
}

func (AgentProfile) TableName() string { return "agent_profiles" }

type AgentMetrics struct {
	BaseModel
	AgentID               string    `gorm:"type:char(36);not null;index" json:"agent_id"`
	CompletedOrders       int       `gorm:"default:0" json:"completed_orders"`
	TotalOrders           int       `gorm:"default:0" json:"total_orders"`
	AverageGrade          float64   `gorm:"type:decimal(5,2);default:0" json:"average_grade"`
	FulfillmentTimeliness float64   `gorm:"type:decimal(5,2);default:0" json:"fulfillment_timeliness"`
	CompletionRate        float64   `gorm:"type:decimal(5,2);default:0" json:"completion_rate"`
	OpenTaskCount         int       `gorm:"default:0" json:"open_task_count"`
	LastCalculated        time.Time `json:"last_calculated"`
}

func (AgentMetrics) TableName() string { return "agent_metrics" }

// Dispatch request types
type CreateOrderRequest struct {
	Category        string `json:"category" binding:"required"`
	Description     string `json:"description"`
	ZoneID          string `json:"zone_id"`
	Address         string `json:"address"`
	ZipCode         string `json:"zip_code"`
	TimeWindowStart string `json:"time_window_start"`
	TimeWindowEnd   string `json:"time_window_end"`
	AssignmentMode  string `json:"assignment_mode"`
	AssignedAgentID string `json:"assigned_agent_id"` // Required when assignment_mode == "assigned"
	Priority        int    `json:"priority"`
}

type AcceptOrderRequest struct {
	IdempotencyKey string `json:"idempotency_key" binding:"required"`
}

type RecommendationResponse struct {
	AgentID         string  `json:"agent_id"`
	UserID          string  `json:"user_id"`
	FullName        string  `json:"full_name"`
	Distance        float64 `json:"distance_km"`
	ReputationScore float64 `json:"reputation_score"`
	OpenTasks       int     `json:"open_tasks"`
	RankingScore    float64 `json:"ranking_score"`
	IsQualified     bool    `json:"is_qualified"`
}
