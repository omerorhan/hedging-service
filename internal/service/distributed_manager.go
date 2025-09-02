package service

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/omerorhan/hedging-service/internal/storage"
)

// DistributedDataManager handles distributed data updates across multiple pods
type DistributedDataManager struct {
	redisCache           storage.Cache
	memCache             *storage.MemoryCache
	podID                string
	isLeader             bool
	mu                   sync.RWMutex
	ctx                  context.Context
	cancel               context.CancelFunc
	wg                   sync.WaitGroup
	ratesRefreshInterval time.Duration
	termsRefreshInterval time.Duration
	lockTTL              time.Duration
	opts                 *ServiceOptions
}

// NewDistributedDataManager creates a new distributed data manager
func NewDistributedDataManager(redisCache storage.Cache, memCache *storage.MemoryCache, opts *ServiceOptions) *DistributedDataManager {
	// Generate unique pod ID using hostname, PID, and nanosecond timestamp
	hostname, _ := os.Hostname()
	podID := fmt.Sprintf("%s-%d-%d", hostname, os.Getpid(), time.Now().UnixNano())

	ctx, cancel := context.WithCancel(context.Background())

	// Set default intervals if not specified
	ratesInterval := opts.RatesRefreshInterval
	if ratesInterval == 0 {
		ratesInterval = 5 * time.Minute // Default 5 minutes for rates
	}

	termsInterval := opts.TermsRefreshInterval
	if termsInterval == 0 {
		termsInterval = 30 * time.Minute // Default 30 minutes for terms (longer period)
	}

	return &DistributedDataManager{
		redisCache:           redisCache,
		memCache:             memCache,
		podID:                podID,
		ratesRefreshInterval: ratesInterval,
		termsRefreshInterval: termsInterval,
		lockTTL:              2 * time.Minute, // Lock expires in 2 minutes
		opts:                 opts,
		ctx:                  ctx,
		cancel:               cancel,
	}
}

// Start begins the distributed data management
func (ddm *DistributedDataManager) Start() error {
	ddm.log("🚀 Starting Distributed Data Manager (Pod ID: %s)", ddm.podID)

	// Start the main loop
	ddm.wg.Add(1)
	go ddm.mainLoop()

	ddm.log("✅ Distributed Data Manager started")
	return nil
}

// Stop gracefully shuts down the distributed data manager
func (ddm *DistributedDataManager) Stop() {
	ddm.log("🛑 Stopping Distributed Data Manager...")

	ddm.cancel()
	ddm.wg.Wait()

	// Release leadership if we're the leader
	if ddm.isLeader {
		ddm.releaseLeadership()
	}

	ddm.log("✅ Distributed Data Manager stopped")
}

// IsLeader returns whether this pod is currently the leader
func (ddm *DistributedDataManager) IsLeader() bool {
	ddm.mu.RLock()
	defer ddm.mu.RUnlock()
	return ddm.isLeader
}

// mainLoop is the main loop that handles leader election and data management
func (ddm *DistributedDataManager) mainLoop() {
	defer ddm.wg.Done()

	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	// Initial sync from Redis
	ddm.syncFromRedis()

	// Perform initial leader election immediately
	ddm.performLeaderElection()

	for {
		select {
		case <-ddm.ctx.Done():
			return
		case <-ticker.C:
			ddm.performLeaderElection()
		}
	}
}

// performLeaderElection handles the leader election logic
func (ddm *DistributedDataManager) performLeaderElection() {
	ddm.mu.Lock()
	defer ddm.mu.Unlock()

	if ddm.isLeader {
		// Try to renew leadership
		renewed, err := ddm.redisCache.RenewLeadership(ddm.podID, ddm.lockTTL)
		if err != nil {
			ddm.log("⚠️ Failed to renew leadership: %v", err)
			ddm.isLeader = false
			return
		}

		if !renewed {
			ddm.log("👑 Leadership lost, becoming follower")
			ddm.isLeader = false
		}
	} else {
		// Try to acquire leadership
		acquired, err := ddm.redisCache.AcquireLeaderLock(ddm.podID, ddm.lockTTL)
		if err != nil {
			ddm.log("⚠️ Failed to acquire leadership: %v", err)
			return
		}

		if acquired {
			ddm.log("👑 Became leader! Starting data refresh loop")
			ddm.isLeader = true

			// Start data refresh loop for leader
			ddm.wg.Add(1)
			go ddm.leaderDataRefreshLoop()
		}
	}
}

// leaderDataRefreshLoop runs only on the leader to fetch data from external APIs
func (ddm *DistributedDataManager) leaderDataRefreshLoop() {
	defer ddm.wg.Done()

	// Create separate tickers for rates and terms
	ratesTicker := time.NewTicker(ddm.ratesRefreshInterval)
	defer ratesTicker.Stop()

	termsTicker := time.NewTicker(ddm.termsRefreshInterval)
	defer termsTicker.Stop()

	// Initial refresh of both
	ddm.refreshDataFromAPIs()

	for {
		select {
		case <-ddm.ctx.Done():
			return
		case <-ratesTicker.C:
			// Check if we're still the leader before refreshing rates
			if ddm.IsLeader() {
				ddm.log("🔄 Time to refresh rates (every %v)", ddm.ratesRefreshInterval)
				if err := ddm.refreshRatesFromAPI(); err != nil {
					ddm.log("❌ Failed to refresh rates: %v", err)
				} else {
					ddm.log("✅ Rates refreshed successfully")
				}
			} else {
				ddm.log("👑 No longer leader, stopping data refresh")
				return
			}
		case <-termsTicker.C:
			// Check if we're still the leader before refreshing terms
			if ddm.IsLeader() {
				ddm.log("🔄 Time to refresh terms (every %v)", ddm.termsRefreshInterval)
				if err := ddm.refreshTermsFromAPI(); err != nil {
					ddm.log("❌ Failed to refresh payment terms: %v", err)
				} else {
					ddm.log("✅ Payment terms refreshed successfully")
				}
			} else {
				ddm.log("👑 No longer leader, stopping data refresh")
				return
			}
		}
	}
}

// refreshDataFromAPIs fetches fresh data from external APIs (leader only)
func (ddm *DistributedDataManager) refreshDataFromAPIs() {
	ddm.log("🔄 Leader refreshing data from external APIs...")

	// Refresh rates
	if err := ddm.refreshRatesFromAPI(); err != nil {
		ddm.log("❌ Failed to refresh rates: %v", err)
	} else {
		ddm.log("✅ Rates refreshed successfully")
	}

	// Refresh payment terms
	if err := ddm.refreshTermsFromAPI(); err != nil {
		ddm.log("❌ Failed to refresh payment terms: %v", err)
	} else {
		ddm.log("✅ Payment terms refreshed successfully")
	}

	// Data versions are updated in individual refresh functions
}

// refreshRatesFromAPI fetches rates from external API
func (ddm *DistributedDataManager) refreshRatesFromAPI() error {
	// Create a temporary hedging service to use existing fetch logic
	tempService := &HedgingService{
		redisCache: ddm.redisCache,
		memCache:   ddm.memCache,
		opts:       ddm.opts,
		ctx:        ddm.ctx,
	}

	ratesEnvelope, err := tempService.fetchRates(ddm.ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch rates: %w", err)
	}

	// Store in Redis
	if err := ddm.redisCache.SetRatesBackup(ratesEnvelope); err != nil {
		return fmt.Errorf("failed to store rates in Redis: %w", err)
	}

	// Also update local memory cache
	if err := ddm.memCache.DumpRates(ratesEnvelope); err != nil {
		ddm.log("⚠️ Failed to update local memory cache: %v", err)
	}

	if ratesEnvelope == nil {
		return fmt.Errorf("rates envelope is nil")
	}

	// Create version (only track rates since terms don't have real versions)
	version := &storage.DataVersion{
		RatesRevision: ratesEnvelope.Revision,
		LastUpdated:   time.Now().UTC(),
		LastUpdatedBy: ddm.podID,
	}

	// Update Redis
	if err := ddm.redisCache.SetDataVersion(version); err != nil {
		return fmt.Errorf("failed to update Redis data version: %w", err)
	}

	ddm.log("📝 Updated Redis rates version: %d", version.RatesRevision)

	return nil
}

// refreshTermsFromAPI fetches payment terms from external API
func (ddm *DistributedDataManager) refreshTermsFromAPI() error {
	// Create a temporary hedging service to use existing fetch logic
	tempService := &HedgingService{
		redisCache: ddm.redisCache,
		memCache:   ddm.memCache,
		opts:       ddm.opts,
		ctx:        ddm.ctx,
	}

	termsEnvelope, err := tempService.fetchPaymentTerms(ddm.ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch payment terms: %w", err)
	}

	// Convert to TermsCacheData
	termsData := &storage.TermsCacheData{
		ByAgency:    make(map[int]storage.AgencyPaymentTerm),
		BpddNames:   make(map[int]string),
		FreqNames:   make(map[int]string),
		LastRefresh: time.Now().UTC(),
	}

	for _, t := range termsEnvelope.AgencyPaymentTerms {
		termsData.ByAgency[t.AgencyId] = storage.AgencyPaymentTerm{
			AgencyId:              t.AgencyId,
			BaseForPaymentDueDate: t.BaseForPaymentDueDate,
			PaymentFrequency:      t.PaymentFrequency,
			DaysAfter:             t.DaysAfter,
		}
	}

	for _, e := range termsEnvelope.BaseForPaymentDueDateMap {
		termsData.BpddNames[e.Id] = e.Name
	}

	for _, e := range termsEnvelope.PaymentFrequencyMap {
		termsData.FreqNames[e.Id] = e.Name
	}

	// Store in Redis
	if err := ddm.redisCache.SetTermsBackup(termsData); err != nil {
		return fmt.Errorf("failed to store terms in Redis: %w", err)
	}

	// Also update local memory cache
	if err := ddm.memCache.DumpTerms(termsData); err != nil {
		ddm.log("⚠️ Failed to update local memory cache: %v", err)
	}

	// Payment terms don't have real versions from API, so no version tracking needed

	return nil
}

// syncFromRedis syncs data from Redis to local memory cache
func (ddm *DistributedDataManager) syncFromRedis() {
	ddm.log("📥 Syncing data from Redis...")

	// Get current data version
	currentVersion, err := ddm.redisCache.GetDataVersion()
	if err != nil {
		ddm.log("⚠️ Failed to get current data version: %v", err)
		return
	}

	if currentVersion == nil {
		ddm.log("📝 No data version found, skipping sync")
		return
	}

	// Get local data version
	_, localRevision, hasData := ddm.memCache.GetRatesMetadata()
	if !hasData {
		// no data yet so it should pull
		ddm.syncRatesFromRedis()
		ddm.syncTermsFromRedis()
		ddm.log("✅ Initial sync completed")
		return
	}

	// Check if we need to sync rates (only rates have real versions)
	if currentVersion.RatesRevision > localRevision {
		ddm.log("🔄 Syncing rates (Redis: %d, Local: %d)", currentVersion.RatesRevision, localRevision)
		ddm.syncRatesFromRedis()
	}

	// Always sync terms since they don't have real versions
	ddm.syncTermsFromRedis()

	ddm.log("✅ Sync completed")
}

// syncRatesFromRedis syncs rates data from Redis to local memory cache
func (ddm *DistributedDataManager) syncRatesFromRedis() {
	envelope, err := ddm.redisCache.GetRatesBackup()
	if err != nil {
		ddm.log("⚠️ Failed to get rates from Redis for sync: %v", err)
		return
	}

	if envelope == nil {
		ddm.log("⚠️ No rates data found in Redis for sync")
		return
	}

	if err := ddm.memCache.DumpRates(envelope); err != nil {
		ddm.log("⚠️ Failed to sync rates to memory cache: %v", err)
		return
	}

	ddm.log("✅ Rates synced from Redis to memory cache (revision: %d)", envelope.Revision)
}

// syncTermsFromRedis syncs payment terms data from Redis to local memory cache
func (ddm *DistributedDataManager) syncTermsFromRedis() {
	termsData, err := ddm.redisCache.GetTermsBackup()
	if err != nil {
		ddm.log("⚠️ Failed to get terms from Redis for sync: %v", err)
		return
	}

	if termsData == nil {
		ddm.log("⚠️ No terms data found in Redis for sync")
		return
	}

	if err := ddm.memCache.DumpTerms(termsData); err != nil {
		ddm.log("⚠️ Failed to sync terms to memory cache: %v", err)
		return
	}

	ddm.log("✅ Terms synced from Redis to memory cache (%d agencies)", len(termsData.ByAgency))
}

// releaseLeadership releases the leadership
func (ddm *DistributedDataManager) releaseLeadership() {
	if err := ddm.redisCache.ReleaseLeaderLock(ddm.podID); err != nil {
		ddm.log("⚠️ Failed to release leadership: %v", err)
	} else {
		ddm.log("👑 Leadership released")
	}
}

// log logs messages with distributed manager prefix
func (ddm *DistributedDataManager) log(format string, args ...interface{}) {
	if ddm.opts.EnableLogging {
		fmt.Printf("[DistributedManager] "+format+"\n", args...)
	}
}
