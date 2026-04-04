package audit

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"dispatchlearn/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Service struct {
	db           *gorm.DB
	mu           sync.Mutex
	lastHash     string
}

func NewService(db *gorm.DB) *Service {
	svc := &Service{db: db}
	// Load last hash from DB
	var last domain.AuditLog
	if err := db.Order("timestamp DESC").First(&last).Error; err == nil {
		svc.lastHash = last.CurrentHash
	}
	return svc
}

type LogEntry struct {
	TenantID    string
	ActorID     string
	Action      string
	EntityType  string
	EntityID    string
	BeforeState interface{}
	AfterState  interface{}
	IPAddress   string
}

func (s *Service) Log(entry LogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	beforeJSON, _ := json.Marshal(entry.BeforeState)
	afterJSON, _ := json.Marshal(entry.AfterState)

	now := time.Now().UTC()
	record := domain.AuditLog{
		ID:           uuid.New().String(),
		TenantID:     entry.TenantID,
		ActorID:      entry.ActorID,
		Action:       entry.Action,
		EntityType:   entry.EntityType,
		EntityID:     entry.EntityID,
		BeforeState:  string(beforeJSON),
		AfterState:   string(afterJSON),
		Timestamp:    now,
		TimestampMs:  now.UnixMilli(),
		PreviousHash: s.lastHash,
		IPAddress:    entry.IPAddress,
	}

	// Tamper-evident hash chain
	record.CurrentHash = computeHash(s.lastHash, record)

	if err := s.db.Create(&record).Error; err != nil {
		return fmt.Errorf("failed to create audit log: %w", err)
	}

	s.lastHash = record.CurrentHash
	return nil
}

func computeHash(previousHash string, record domain.AuditLog) string {
	// Use stored millisecond timestamp for deterministic hashing
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%s|%d",
		previousHash,
		record.TenantID,
		record.ActorID,
		record.Action,
		record.EntityType,
		record.EntityID,
		record.BeforeState,
		record.AfterState,
		record.TimestampMs,
	)
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func (s *Service) VerifyChain(tenantID string) (bool, error) {
	// Verify global chain integrity (audit logs form a single global chain)
	var logs []domain.AuditLog
	if err := s.db.Order("timestamp ASC").Find(&logs).Error; err != nil {
		return false, err
	}

	if len(logs) == 0 {
		return true, nil
	}

	prevHash := ""
	for _, log := range logs {
		expected := computeHash(prevHash, log)
		if log.CurrentHash != expected {
			return false, fmt.Errorf("hash mismatch at log %s", log.ID)
		}
		if log.PreviousHash != prevHash {
			return false, fmt.Errorf("chain broken at log %s", log.ID)
		}
		prevHash = log.CurrentHash
	}
	return true, nil
}
