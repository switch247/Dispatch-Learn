package middleware

import (
	"sync"
	"time"

	"dispatchlearn/internal/domain"

	"gorm.io/gorm"
)

// DBQuotaProvider fetches tenant quota overrides from the database with a 1-minute cache.
type DBQuotaProvider struct {
	db    *gorm.DB
	mu    sync.RWMutex
	cache map[string]*cachedQuota
}

type cachedQuota struct {
	rpm          int
	burst        int
	webhookDaily int
	fetchedAt    time.Time
}

const quotaCacheTTL = 1 * time.Minute

func NewDBQuotaProvider(db *gorm.DB) *DBQuotaProvider {
	return &DBQuotaProvider{
		db:    db,
		cache: make(map[string]*cachedQuota),
	}
}

func (p *DBQuotaProvider) GetTenantQuota(tenantID string) (rpm int, burst int, webhookDaily int, found bool) {
	// Check cache first
	p.mu.RLock()
	if cached, ok := p.cache[tenantID]; ok && time.Since(cached.fetchedAt) < quotaCacheTTL {
		p.mu.RUnlock()
		return cached.rpm, cached.burst, cached.webhookDaily, true
	}
	p.mu.RUnlock()

	// Fetch from DB
	var override domain.QuotaOverride
	err := p.db.Where("tenant_id = ?", tenantID).First(&override).Error
	if err != nil {
		return 0, 0, 0, false
	}

	// Update cache
	p.mu.Lock()
	p.cache[tenantID] = &cachedQuota{
		rpm:          override.RPM,
		burst:        override.Burst,
		webhookDaily: override.WebhookDailyLimit,
		fetchedAt:    time.Now(),
	}
	p.mu.Unlock()

	return override.RPM, override.Burst, override.WebhookDailyLimit, true
}
