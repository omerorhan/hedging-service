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
	syncInterval         time.Duration
	lockTTL              time.Duration
	opts                 *ServiceOptions
	started              bool
	startMu              sync.Mutex
	// Dynamic refresh tracking
	nextRatesRefresh time.Time
	nextTermsRefresh time.Time
	refreshMu        sync.RWMutex
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
		termsInterval = 2 * time.Hour // Default 2 hours for terms (static interval)
	}

	syncInterval := 5 * time.Second // Default 5 seconds for data sync
	if opts.SyncInterval != 0 {
		syncInterval = opts.SyncInterval
	}

	return &DistributedDataManager{
		redisCache:           redisCache,
		memCache:             memCache,
		podID:                podID,
		ratesRefreshInterval: ratesInterval,
		termsRefreshInterval: termsInterval,
		syncInterval:         syncInterval,
		lockTTL:              2 * time.Minute, // Lock expires in 2 minutes
		opts:                 opts,
		ctx:                  ctx,
		cancel:               cancel,
	}
}

// Start begins the distributed data management
func (ddm *DistributedDataManager) Start() error {
	ddm.startMu.Lock()
	defer ddm.startMu.Unlock()

	if ddm.started {
		return fmt.Errorf("distributed data manager already started")
	}

	ddm.log("🚀 Starting Distributed Data Manager (Pod ID: %s)", ddm.podID)

	// Start the main loop
	ddm.wg.Add(1)
	go ddm.mainLoop()

	ddm.started = true
	ddm.log("✅ Distributed Data Manager started")
	return nil
}

// Stop gracefully shuts down the distributed data manager
func (ddm *DistributedDataManager) Stop() {
	ddm.log("🛑 Stopping Distributed Data Manager...")

	ddm.cancel()

	// Wait for goroutines with timeout to prevent infinite blocking
	done := make(chan struct{})
	go func() {
		ddm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		ddm.log("✅ All goroutines stopped gracefully")
	case <-time.After(10 * time.Second):
		ddm.log("⚠️ Timeout waiting for goroutines to stop")
	}

	// Release leadership if we're the leader (with proper locking)
	ddm.mu.Lock()
	wasLeader := ddm.isLeader
	ddm.mu.Unlock()

	if wasLeader {
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

// GetPodID returns the pod ID for testing purposes
func (ddm *DistributedDataManager) GetPodID() string {
	return ddm.podID
}

// calculateNextRatesRefresh calculates when to refresh rates based on ValidUntil date
func (ddm *DistributedDataManager) calculateNextRatesRefresh() time.Time {
	validUntil, _, hasData := ddm.memCache.GetRatesMetadata()
	if !hasData {
		// No data yet, refresh immediately
		return time.Now().UTC()
	}

	if validUntil.IsZero() {
		// No ValidUntil date, use fallback interval
		return time.Now().UTC().Add(ddm.ratesRefreshInterval)
	}

	// Refresh 15 seconds before data expires
	nextRefresh := validUntil.Add(-11 * time.Minute)

	// Ensure we don't schedule a refresh in the past
	if nextRefresh.Before(time.Now().UTC()) {
		return time.Now().UTC()
	}

	return nextRefresh
}

// calculateNextTermsRefresh calculates when to refresh terms based on last refresh time
func (ddm *DistributedDataManager) calculateNextTermsRefresh() time.Time {
	lastRefresh := ddm.memCache.GetTermsLastRefresh()
	if lastRefresh.IsZero() {
		// No terms data yet, refresh immediately
		return time.Now().UTC()
	}

	// Use the configured interval for terms (they don't have ValidUntil)
	return lastRefresh.Add(ddm.termsRefreshInterval)
}

// updateNextRefreshTimes updates the next refresh times based on current data
func (ddm *DistributedDataManager) updateNextRefreshTimes() {
	ddm.refreshMu.Lock()
	defer ddm.refreshMu.Unlock()

	ddm.nextRatesRefresh = ddm.calculateNextRatesRefresh()
	ddm.nextTermsRefresh = ddm.calculateNextTermsRefresh()

	ddm.log("📅 Next rates refresh: %v (ValidUntil - 15s)", ddm.nextRatesRefresh.Format("2006-01-02 15:04:05 UTC"))
	ddm.log("📅 Next terms refresh: %v (static %v interval)", ddm.nextTermsRefresh.Format("2006-01-02 15:04:05 UTC"), ddm.termsRefreshInterval)
}

// getNextRefreshTime returns the earliest of the next refresh times
func (ddm *DistributedDataManager) getNextRefreshTime() time.Time {
	ddm.refreshMu.RLock()
	defer ddm.refreshMu.RUnlock()

	return ddm.nextRatesRefresh
}

// mainLoop is the main loop that handles leader election and data management
func (ddm *DistributedDataManager) mainLoop() {
	defer ddm.wg.Done()

	// Leader election ticker - check every 30 seconds
	leaderTicker := time.NewTicker(20 * time.Second)
	defer leaderTicker.Stop()

	// Data sync ticker - check periodically for non-leader pods
	syncTicker := time.NewTicker(ddm.syncInterval)
	defer syncTicker.Stop()

	// Initial sync from Redis
	ddm.syncFromRedis()

	// Perform initial leader election immediately
	ddm.performLeaderElection()

	for {
		select {
		case <-ddm.ctx.Done():
			return
		case <-leaderTicker.C:
			ddm.performLeaderElection()
		case <-syncTicker.C:
			// Only sync if we're not the leader (leaders update data themselves)
			if !ddm.IsLeader() {
				ddm.log("🔄 Non-leader pod checking for data updates...")
				if ddm.needsDataSync() {
					ddm.syncFromRedis()
				} else {
					ddm.log("📝 Data is up to date, skipping sync")
				}
			}
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

	// Initial refresh of both
	ddm.refreshDataFromAPIs()

	// Update refresh times after initial load
	ddm.updateNextRefreshTimes()

	for {
		// Check if we're still the leader
		if !ddm.IsLeader() {
			ddm.log("👑 No longer leader, stopping data refresh")
			return
		}

		// Update refresh times and get the next refresh time
		ddm.updateNextRefreshTimes()
		nextRefresh := ddm.getNextRefreshTime()
		now := time.Now().UTC()

		// Wait until next refresh time if it's in the future
		if nextRefresh.After(now) {
			waitDuration := nextRefresh.Sub(now)
			ddm.log("⏰ Next refresh in %v (at %v)", waitDuration, nextRefresh.Format("2006-01-02 15:04:05 UTC"))

			// Wait until next refresh time or context cancellation
			timer := time.NewTimer(waitDuration)
			select {
			case <-ddm.ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
				// Time to refresh - recalculate times and refresh what's needed
				ddm.updateNextRefreshTimes()
			}
		}

		// Get current refresh times after waiting/updating
		ddm.refreshMu.RLock()
		ratesTime := ddm.nextRatesRefresh
		termsTime := ddm.nextTermsRefresh
		ddm.refreshMu.RUnlock()

		now = time.Now().UTC()

		// Refresh rates if it's time (ValidUntil-based)
		if !ratesTime.After(now) {
			ddm.log("🔄 Time to refresh rates (ValidUntil-based)")
			if err := ddm.refreshRatesFromAPI(); err != nil {
				ddm.log("❌ Failed to refresh rates: %v", err)
			} else {
				ddm.log("✅ Rates refreshed successfully")
			}
		}

		// Refresh terms if it's time (static interval-based)
		if !termsTime.After(now) {
			ddm.log("🔄 Time to refresh terms (static interval-based)")
			if err := ddm.refreshTermsFromAPI(); err != nil {
				ddm.log("❌ Failed to refresh payment terms: %v", err)
			} else {
				ddm.log("✅ Payment terms refreshed successfully")
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

// getDataVersionInfo retrieves and compares data versions between Redis and local cache
// Returns: (needsRatesSync, needsTermsSync, currentVersion, localRevision, error)
func (ddm *DistributedDataManager) getDataVersionInfo() (bool, *storage.DataVersion, int, error) {
	// Get current data version from Redis
	currentVersion, err := ddm.redisCache.GetDataVersion()
	if err != nil {
		ddm.log("⚠️ Failed to get current data version: %v", err)
		return false, nil, 0, err
	}

	// Get local data version
	_, localRevision, hasData := ddm.memCache.GetRatesMetadata()
	if !hasData {
		ddm.log("📝 No local data found, full sync needed")
		return true, currentVersion, 0, nil
	}

	if currentVersion == nil {
		ddm.log("📝 No data version found in Redis, but local data exists - no sync needed")
		return false, nil, localRevision, nil
	}

	// Check if rates need syncing (only rates have real versions)
	needsRatesSync := currentVersion.RatesRevision > localRevision
	// needsTermsSync := true // Always sync terms since they don't have versioning

	if needsRatesSync {
		ddm.log("🔄 Data sync needed: Redis rates revision %d > local %d", currentVersion.RatesRevision, localRevision)
	} else {
		ddm.log("📝 Rates are up to date (Redis: %d, Local: %d)", currentVersion.RatesRevision, localRevision)
	}

	return needsRatesSync, currentVersion, localRevision, nil
}

// needsDataSync checks if data needs to be synced from Redis
func (ddm *DistributedDataManager) needsDataSync() bool {
	needsRatesSync, _, _, err := ddm.getDataVersionInfo()
	if err != nil {
		return false // Don't sync if we can't check version
	}
	return needsRatesSync
}

// syncFromRedis syncs data from Redis to local memory cache
func (ddm *DistributedDataManager) syncFromRedis() {
	ddm.log("📥 Syncing data from Redis...")

	needsRatesSync, currentVersion, localRevision, err := ddm.getDataVersionInfo()
	if err != nil {
		return
	}

	if currentVersion == nil {
		ddm.log("📝 No data version found, skipping sync")
		return
	}

	// Sync rates if needed
	if needsRatesSync {
		ddm.log("🔄 Syncing rates (Redis: %d, Local: %d)", currentVersion.RatesRevision, localRevision)
		ddm.syncRatesFromRedis()
	}

	// Always sync terms since they don't have versioning
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
