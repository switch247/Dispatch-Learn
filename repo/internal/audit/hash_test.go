package audit

import (
	"testing"
	"time"

	"dispatchlearn/internal/domain"

	"github.com/stretchr/testify/assert"
)

// computeHash is unexported — tested here within the audit package.

func makeRecord(prevHash, tenantID, actorID, action, entityType, entityID, before, after string, tsMs int64) domain.AuditLog {
	return domain.AuditLog{
		TenantID:     tenantID,
		ActorID:      actorID,
		Action:       action,
		EntityType:   entityType,
		EntityID:     entityID,
		BeforeState:  before,
		AfterState:   after,
		Timestamp:    time.Now(),
		TimestampMs:  tsMs,
		PreviousHash: prevHash,
	}
}

func TestComputeHashDeterminism(t *testing.T) {
	r := makeRecord("", "tenant-1", "actor-1", "order.created", "order", "id-1", "{}", `{"status":"CREATED"}`, 1700000000000)

	h1 := computeHash("", r)
	h2 := computeHash("", r)
	assert.Equal(t, h1, h2, "same inputs must produce same hash")
}

func TestComputeHashLength(t *testing.T) {
	r := makeRecord("", "tenant-1", "actor-1", "login", "user", "user-1", "", "{}", 1700000000001)
	h := computeHash("", r)
	assert.Len(t, h, 64, "SHA-256 hex digest must be 64 characters")
}

func TestComputeHashChainLinking(t *testing.T) {
	// Different previousHash must produce different currentHash
	r := makeRecord("", "tenant-1", "actor-1", "order.created", "order", "id-1", "{}", "{}", 1700000000002)

	hashA := computeHash("prev-hash-A", r)
	hashB := computeHash("prev-hash-B", r)
	assert.NotEqual(t, hashA, hashB, "different previousHash must produce different hash")
}

func TestComputeHashSensitiveToFields(t *testing.T) {
	base := makeRecord("prev", "tenant-1", "actor-1", "order.created", "order", "id-1", "{}", "{}", 1700000000003)

	hashBase := computeHash("prev", base)

	// Change action
	modified := base
	modified.Action = "order.updated"
	assert.NotEqual(t, hashBase, computeHash("prev", modified))

	// Change entityID
	modified = base
	modified.EntityID = "id-2"
	assert.NotEqual(t, hashBase, computeHash("prev", modified))

	// Change tenantID
	modified = base
	modified.TenantID = "tenant-2"
	assert.NotEqual(t, hashBase, computeHash("prev", modified))

	// Change actorID
	modified = base
	modified.ActorID = "actor-2"
	assert.NotEqual(t, hashBase, computeHash("prev", modified))

	// Change afterState
	modified = base
	modified.AfterState = `{"status":"UPDATED"}`
	assert.NotEqual(t, hashBase, computeHash("prev", modified))

	// Change timestamp ms
	modified = base
	modified.TimestampMs = base.TimestampMs + 1
	assert.NotEqual(t, hashBase, computeHash("prev", modified))
}

func TestAuditHashChainIntegrity(t *testing.T) {
	// Simulate a chain of 5 records and verify each links correctly
	type entry struct {
		prevHash string
		record   domain.AuditLog
		hash     string
	}

	var chain []entry
	prevHash := ""

	actions := []string{"user.registered", "order.created", "order.accepted", "payment.recorded", "report.generated"}
	for i, action := range actions {
		r := makeRecord(prevHash, "tenant-x", "actor-x", action, "entity", "id-x", "", "{}", int64(1700000000000+i))
		h := computeHash(prevHash, r)
		chain = append(chain, entry{prevHash: prevHash, record: r, hash: h})
		prevHash = h
	}

	// Re-verify the entire chain
	prevHash = ""
	for _, e := range chain {
		expected := computeHash(prevHash, e.record)
		assert.Equal(t, expected, e.hash, "chain entry for %s must verify", e.record.Action)
		prevHash = e.hash
	}
}

func TestAuditHashTamperDetection(t *testing.T) {
	// Simulates what VerifyChain detects: modifying afterState breaks the hash
	r := makeRecord("", "tenant-1", "actor-1", "payment.recorded", "payment", "pay-1", "{}", `{"amount":100}`, 1700000001000)
	originalHash := computeHash("", r)

	// Tamper with after state
	r.AfterState = `{"amount":999}`
	tamperedHash := computeHash("", r)

	assert.NotEqual(t, originalHash, tamperedHash, "tampered record must produce different hash")
}

func TestEmptyChainIsValid(t *testing.T) {
	// Zero records: valid by definition
	var logs []domain.AuditLog
	assert.Empty(t, logs)
	// A chain with no entries is trivially valid
	prevHash := ""
	for _, log := range logs {
		expected := computeHash(prevHash, log)
		assert.Equal(t, expected, log.CurrentHash)
		prevHash = log.CurrentHash
	}
}
