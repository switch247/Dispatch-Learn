package middleware

import (
	"net/http"
	"sync"
	"time"

	"dispatchlearn/internal/domain"
	"dispatchlearn/logging"

	"github.com/gin-gonic/gin"
)

// QuotaProvider abstracts database access for tenant-specific quota overrides.
// Implementations should cache results to avoid per-request DB queries.
type QuotaProvider interface {
	GetTenantQuota(tenantID string) (rpm int, burst int, webhookDaily int, found bool)
}

type tokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * tb.refillRate
	if tb.tokens > tb.maxTokens {
		tb.tokens = tb.maxTokens
	}
	tb.lastRefill = now

	if tb.tokens >= 1 {
		tb.tokens--
		return true
	}
	return false
}

type RateLimiter struct {
	mu            sync.RWMutex
	buckets       map[string]*tokenBucket
	defaultRPM    int
	defaultBurst  int
	quotaProvider QuotaProvider
}

func NewRateLimiter(rpm, burst int, provider QuotaProvider) *RateLimiter {
	return &RateLimiter{
		buckets:       make(map[string]*tokenBucket),
		defaultRPM:    rpm,
		defaultBurst:  burst,
		quotaProvider: provider,
	}
}

func (rl *RateLimiter) getBucket(tenantID string) *tokenBucket {
	// Check for tenant-specific override
	rpm, burst := rl.defaultRPM, rl.defaultBurst
	if rl.quotaProvider != nil {
		if tRPM, tBurst, _, found := rl.quotaProvider.GetTenantQuota(tenantID); found {
			rpm = tRPM
			burst = tBurst
		}
	}

	rl.mu.RLock()
	bucket, exists := rl.buckets[tenantID]
	rl.mu.RUnlock()

	if exists {
		// Update refill rate if override changed
		bucket.mu.Lock()
		bucket.refillRate = float64(rpm) / 60.0
		bucket.maxTokens = float64(burst)
		bucket.mu.Unlock()
		return bucket
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	if bucket, exists = rl.buckets[tenantID]; exists {
		return bucket
	}

	bucket = &tokenBucket{
		tokens:     float64(burst),
		maxTokens:  float64(burst),
		refillRate: float64(rpm) / 60.0,
		lastRefill: time.Now(),
	}
	rl.buckets[tenantID] = bucket
	return bucket
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := GetTenantID(c)
		if tenantID == "" {
			c.Next()
			return
		}

		bucket := rl.getBucket(tenantID)
		if !bucket.allow() {
			logging.Warn("ratelimit", "quota", "rate limit exceeded for tenant: "+tenantID)
			c.AbortWithStatusJSON(http.StatusTooManyRequests, domain.APIResponse{
				Errors: []domain.APIError{{Code: "RATE_LIMITED", Message: "request rate limit exceeded"}},
			})
			return
		}
		c.Next()
	}
}

// WebhookQuotaTracker tracks daily webhook delivery counts per tenant
// with support for DB-backed per-tenant overrides via QuotaProvider.
type WebhookQuotaTracker struct {
	mu            sync.RWMutex
	counts        map[string]*dailyCount
	tenantCaps    map[string]int // per-tenant cap overrides
	defaultCap    int
	quotaProvider QuotaProvider
}

type dailyCount struct {
	count int
	date  string // YYYY-MM-DD
}

func NewWebhookQuotaTracker(defaultCap int, provider QuotaProvider) *WebhookQuotaTracker {
	return &WebhookQuotaTracker{
		counts:        make(map[string]*dailyCount),
		tenantCaps:    make(map[string]int),
		defaultCap:    defaultCap,
		quotaProvider: provider,
	}
}

func (wq *WebhookQuotaTracker) getCapForTenant(tenantID string) int {
	// Check DB-backed override via provider
	if wq.quotaProvider != nil {
		if _, _, webhookCap, found := wq.quotaProvider.GetTenantQuota(tenantID); found && webhookCap > 0 {
			return webhookCap
		}
	}
	// Check in-memory override
	wq.mu.RLock()
	if cap, ok := wq.tenantCaps[tenantID]; ok {
		wq.mu.RUnlock()
		return cap
	}
	wq.mu.RUnlock()
	return wq.defaultCap
}

// Increment increments the daily webhook count for a tenant.
// Returns false and logs a quota violation if the daily cap is exceeded.
func (wq *WebhookQuotaTracker) Increment(tenantID string) bool {
	cap := wq.getCapForTenant(tenantID)

	wq.mu.Lock()
	defer wq.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	dc, exists := wq.counts[tenantID]
	if !exists || dc.date != today {
		wq.counts[tenantID] = &dailyCount{count: 1, date: today}
		return true
	}
	if dc.count >= cap {
		logging.Warn("ratelimit", "webhook-quota",
			"webhook daily quota exceeded for tenant: "+tenantID)
		return false
	}
	dc.count++
	return true
}

// SetCap sets a per-tenant webhook daily cap override.
func (wq *WebhookQuotaTracker) SetCap(tenantID string, cap int) {
	wq.mu.Lock()
	defer wq.mu.Unlock()
	wq.tenantCaps[tenantID] = cap
}

func (wq *WebhookQuotaTracker) GetCount(tenantID string) int {
	wq.mu.RLock()
	defer wq.mu.RUnlock()
	today := time.Now().Format("2006-01-02")
	if dc, exists := wq.counts[tenantID]; exists && dc.date == today {
		return dc.count
	}
	return 0
}
