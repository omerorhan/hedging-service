package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/omerorhan/hedging-service/internal/storage"
)

type AgencyPaymentTerm struct {
	AgencyId              int `json:"agencyId"`
	BaseForPaymentDueDate int `json:"baseForPaymentDueDate"`
	PaymentFrequency      int `json:"paymentFrequency"`
	DaysAfter             int `json:"daysAfterPaymentPeriod"`
}

func parseBasicAuthPair(auth string) (username, password string, ok bool) {
	if auth == "" {
		return "", "", false
	}
	parts := strings.SplitN(auth, ":", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// HedgingService is a self-managing service that automatically handles caching and data updates
type HedgingService struct {
	redisCache  storage.Cache
	memCache    *storage.MemoryCache
	opts        *ServiceOptions
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	mu          sync.RWMutex
	initialized bool
}

// ServiceOptions provides configuration for the hedging service
type ServiceOptions struct {
	RedisAddr             string        `json:"redisAddr"`
	RatesRefreshInterval  time.Duration `json:"ratesRefreshInterval"`
	TermsRefreshInterval  time.Duration `json:"termsRefreshInterval"`
	InitialLoadTimeout    time.Duration `json:"initialLoadTimeout"`
	EnableLogging         bool          `json:"enableLogging"`
	MaxRetries            int           `json:"maxRetries"`
	RetryDelay            time.Duration `json:"retryDelay"`
	RatesBaseUrl          string        `json:"ratesBaseUrl"`
	RatesBasicAuth        string        `json:"ratesBasicAuth"`
	PaymentTermsBaseUrl   string        `json:"paymentTermsBaseUrl"`
	PaymentTermsBasicAuth string        `json:"paymentTermsBasicAuth"`
}

// DefaultServiceOptions returns sensible default options
func DefaultServiceOptions() *ServiceOptions {
	return &ServiceOptions{
		RedisAddr:          "tcp://localhost:6379",
		InitialLoadTimeout: 30 * time.Second,
		EnableLogging:      true,
	}
}

// NewHedgingService creates a new self-managing hedging service
func NewHedgingService(options ...ServiceOption) (*HedgingService, error) {
	opts := DefaultServiceOptions()

	// Apply options
	for _, option := range options {
		option(opts)
	}

	// Create Redis cache
	redisCache, err := storage.NewRedisCache(
		opts.RedisAddr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis cache: %w", err)
	}

	// Create in-memory cache
	memCache := storage.NewMemoryCache()
	ctx, cancel := context.WithCancel(context.Background())

	service := &HedgingService{
		redisCache: redisCache,
		memCache:   memCache,
		opts:       opts,
		ctx:        ctx,
		cancel:     cancel,
	}

	return service, nil
}

// ServiceOption is a function that configures service options
type ServiceOption func(*ServiceOptions)

// WithRateBaseUrl sets Redis configuration
func WithRateBaseUrl(url, auth string) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.RatesBaseUrl = url
		opts.RatesBasicAuth = auth
	}
}

// WithPaymentTermsBaseUrl sets Redis configuration
func WithPaymentTermsBaseUrl(url, auth string) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.PaymentTermsBaseUrl = url
		opts.PaymentTermsBasicAuth = auth
	}
}

// WithRedisConfig sets Redis configuration
func WithRedisConfig(addr string) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.RedisAddr = addr
	}
}

func WithRatesRefreshInterval(interval time.Duration) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.RatesRefreshInterval = interval
	}
}
func WithTermsRefreshInterval(interval time.Duration) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.TermsRefreshInterval = interval
	}
}

// WithLogging enables/disables logging
func WithLogging(enabled bool) ServiceOption {
	return func(opts *ServiceOptions) {
		opts.EnableLogging = enabled
	}
}

// Initialize starts the service and loads initial data
func (hs *HedgingService) Initialize() error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if hs.initialized {
		return nil
	}

	hs.log("🚀 Initializing Hedging Service...")

	// Initialize caches from Redis
	if err := hs.initializeCachesFromRedis(); err != nil {
		hs.log("⚠️ Warning: Failed to initialize caches from Redis: %v", err)
	}

	// Start schedulers
	hs.startSchedulers()

	hs.initialized = true
	hs.log("✅ Hedging Service initialized successfully")
	return nil
}

func (hs *HedgingService) initializeCachesFromRedis() error {
	// hs.log("📥 Loading existing data from Redis...")

	ratesFromBackup, err := hs.redisCache.GetRatesBackup()
	ratesNeedsRefresh := true

	if err != nil {
		hs.log("⚠️ Warning: Failed to load backup from Redis: %v", err)
	} else if ratesFromBackup != nil {
		hs.log("✅ Loaded backup from Redis (revision: %d)", ratesFromBackup.Revision)
		ratesNeedsRefresh = false

		validUntil, err := time.Parse("2006-01-02T15:04:05", ratesFromBackup.ValidUntilDate)
		if err != nil || validUntil.IsZero() || validUntil.Before(time.Now().UTC()) {
			hs.log("⏰ Backup data has expired (validUntil: %s), will force refresh", ratesFromBackup.ValidUntilDate)
			ratesNeedsRefresh = true
		}
	}

	if ratesNeedsRefresh {
		hs.refreshRates()
	} else {
		// dump rates to memory from backup
		err := hs.memCache.DumpRates(ratesFromBackup)
		if err != nil {
			return err
		}
	}

	termsNeedsRefresh := true
	// Load payment terms from Redis
	termsFromBackup, err := hs.redisCache.GetTermsBackup()
	if err != nil {
		hs.log("⚠️ Warning: Failed to load payment terms from Redis: %v", err)
	} else if termsFromBackup != nil {
		hs.log("✅ Loaded payment terms for %d agencies from Redis", len(termsFromBackup.ByAgency))
		termsNeedsRefresh = false

		err := hs.memCache.DumpTerms(termsFromBackup)
		if err != nil {
			return err
		}
	}

	if termsNeedsRefresh {
		hs.refreshPaymentTerms()
	}

	hs.log("📊 Cache initialization completed")
	return nil
}

// startSchedulers starts background goroutines to keep data fresh
func (hs *HedgingService) startSchedulers() {
	hs.log("⏰ Starting schedulers...")

	// Start rates refresh scheduler
	hs.wg.Add(1)
	go hs.ratesRefreshScheduler()

	// Start payment terms refresh scheduler
	hs.wg.Add(1)
	go hs.termsRefreshScheduler()

	hs.log("✅ Schedulers started")
}

func (hs *HedgingService) ratesRefreshScheduler() {
	defer hs.wg.Done()

	// Add jitter (±10% of interval) to prevent thundering herd in K8s
	jitteredInterval := hs.addJitter(hs.opts.RatesRefreshInterval, 0.1)
	ticker := time.NewTicker(jitteredInterval)
	defer ticker.Stop()

	hs.log("🔄 Rates refresh scheduler started (base interval: %v, jittered: %v)", hs.opts.RatesRefreshInterval, jitteredInterval)

	for {
		select {
		case <-hs.ctx.Done():
			hs.log("🛑 Rates refresh scheduler stopped")
			return
		case <-ticker.C:
			// Add small jitter to each refresh as well
			go func() {
				jitterDelay := time.Duration(rand.Int63n(int64(5 * time.Second)))
				time.Sleep(jitterDelay)
				hs.refreshRates()
			}()
		}
	}
}

// paymentTermsRefreshScheduler periodically refreshes payment terms
func (hs *HedgingService) termsRefreshScheduler() {
	defer hs.wg.Done()

	// Add jitter (±10% of interval) to prevent thundering herd in K8s
	jitteredInterval := hs.addJitter(hs.opts.TermsRefreshInterval, 0.1)
	ticker := time.NewTicker(jitteredInterval)
	defer ticker.Stop()

	hs.log("🔄 Payment terms refresh scheduler started (base interval: %v, jittered: %v)", hs.opts.TermsRefreshInterval, jitteredInterval)

	for {
		select {
		case <-hs.ctx.Done():
			hs.log("🛑 Payment terms refresh scheduler stopped")
			return
		case <-ticker.C:
			// Add small jitter to each refresh as well
			go func() {
				jitterDelay := time.Duration(rand.Int63n(int64(10 * time.Second)))
				time.Sleep(jitterDelay)
				hs.refreshPaymentTerms()
			}()
		}
	}
}

func (hs *HedgingService) refreshRates() {
	// Check if current rates will expire before next refresh
	if !hs.shouldRefreshRates() {
		hs.log("⏭️ Skipping rates refresh - current data valid until next refresh")
		return
	}

	hs.log("🔄 Refreshing exchange rates...")

	// Fetch fresh rates from external API using your source code logic
	ratesEnvelope, err := hs.fetchRates(hs.ctx)
	if err != nil {
		hs.log("❌ Failed to fetch rates: %v", err)
		return
	}

	// Hydrate rates using your source code logic
	if err := hs.hydrateRates(ratesEnvelope); err != nil {
		hs.log("❌ Failed to hydrate rates: %v", err)
		return
	}

	hs.log("✅ Exchange rates refreshed successfully")
}

// refreshPaymentTerms fetches fresh payment terms
func (hs *HedgingService) refreshPaymentTerms() {
	hs.log("🔄 Refreshing payment terms...")

	// Fetch fresh payment terms from external API using your source code logic
	termsEnvelope, err := hs.fetchPaymentTerms(hs.ctx)
	if err != nil {
		hs.log("❌ Failed to fetch payment terms: %v", err)
		return
	}

	// Hydrate payment terms using your source code logic
	if err := hs.hydrateTerms(termsEnvelope); err != nil {
		hs.log("❌ Failed to hydrate payment terms: %v", err)
		return
	}

	hs.log("✅ Payment terms refreshed successfully")
}

func (hs *HedgingService) GiveMeRate(req GiveMeRateReq) (*GiveMeRateResp, error) {
	if !hs.initialized {
		return nil, fmt.Errorf("service not initialized - call Initialize() first")
	}

	term, bpddName, freqName, err := hs.memCache.GetTerms(req.AgencyId)
	if err != nil {
		return nil, err
	}

	// Base date
	baseDate, baseSrc, err := chooseBaseDate(req, term.BaseForPaymentDueDate, bpddName)
	if err != nil {
		return nil, fmt.Errorf("base date error: %w", err)
	}

	// Days due from frequency
	dd, ddExplain := daysDueFromFrequency(baseDate, term.PaymentFrequency, freqName, term.DaysAfter)

	// Lead time (bookingCreatedAt -> baseDate)
	lead := clampNonNegative(daysBetween(req.BookingCreatedAt.UTC(), baseDate.UTC()))

	// Days to hedge
	dth := clampNonNegative(lead + dd)

	// Rates pair
	pairKey := getPairKey(req.From, req.To)

	// Get all rate data in a single call
	rateData, err := hs.memCache.GetRates(pairKey)
	if err != nil {
		return nil, err
	}

	var rate float64
	var rateType string
	var explain string

	if strings.EqualFold(req.From, req.To) {
		return nil, fmt.Errorf("from and to are same")
	}

	// Select tenor
	ten, _ := selectTenor(rateData.Pair.Tenors, dth) // no need to check error, if tenor not found it should use spot as fallback

	if ten.Rate != 0 {
		rate = ten.Rate
		rateType = Hedged
	}

	if rateData.Pair.Spot != nil && (ten.Rate == 0 || ten.Days == 0) {
		rate = *rateData.Pair.Spot
		rateType = Spot
	}

	if rateData.Pair.Spot == nil {
		return nil, fmt.Errorf("no tenor, no spot found from: %s, to: %s", req.From, req.To)
	}

	explain = fmt.Sprintf("%s => Base=%s; Lead=%dd + DaysDue(%d) => DaysToHedge=%d; tenor=%dd",
		ddExplain, baseSrc, lead, dd, dth, ten.Days)

	// Use validUntil and revisionNumber from the single call
	validUntil := rateData.ValidUntil
	revisionNumber := rateData.RevisionNumber

	if validUntil.IsZero() || validUntil.Before(time.Now().UTC()) {
		return nil, fmt.Errorf("rate validUntil has passed, it's not valid anymore")
	}

	if revisionNumber == 0 {
		return nil, fmt.Errorf("rate revision could not be found through memory: %v", err)
	}

	dueDate := req.BookingCreatedAt.Add(time.Duration(dth) * 24 * time.Hour)

	resp := &GiveMeRateResp{
		From:         req.From,
		To:           req.To,
		Rate:         rate,
		IsRefundable: !req.Nonrefundable,
		RevisionId:   revisionNumber,
		DueDate:      dueDate,
		ValidUntil:   validUntil,
		Type:         rateType,
		Explain:      explain,
	}
	return resp, nil
}

// Stop gracefully shuts down the service
func (hs *HedgingService) Stop() {
	hs.log("🛑 Stopping Hedging Service...")

	hs.cancel()  // Signal all goroutines to stop
	hs.wg.Wait() // Wait for all goroutines to finish

	// Close caches
	hs.redisCache.Close()

	hs.log("✅ Hedging Service stopped")
}

func (hs *HedgingService) log(format string, args ...interface{}) {
	if hs.opts.EnableLogging {
		// Use fmt.Printf to write to stdout (INFO level in GCP) instead of stderr (ERROR level)
		fmt.Printf("[HedgingService] "+format+"\n", args...)
	}
}

// shouldRefreshRates checks if rates will expire before next refresh
// Also syncs memory cache with Redis if Redis has newer revision
func (hs *HedgingService) shouldRefreshRates() bool {
	// Get current rate metadata from Redis to check validUntil
	envelope, err := hs.redisCache.GetRatesBackup()
	if err != nil {
		hs.log("⚠️ Failed to get rates backup from Redis: %v - refresh needed", err)
		return true
	}
	if envelope == nil {
		hs.log("🔄 No rates backup found in Redis - refresh needed")
		return true
	}

	// Get current memory cache revision for comparison
	_, memoryRevision, memoryHasData := hs.memCache.GetRatesMetadata()

	// Check if Redis has newer data than memory (multi-pod sync)
	if memoryHasData && envelope.Revision > memoryRevision {
		hs.log("🔄 Redis has newer revision (%d > %d) - syncing memory cache", envelope.Revision, memoryRevision)
		if err := hs.memCache.DumpRates(envelope); err != nil {
			hs.log("⚠️ Failed to sync memory cache from Redis: %v", err)
			// Continue with normal logic despite sync failure
		} else {
			hs.log("✅ Memory cache synced with Redis revision %d", envelope.Revision)
		}
	}

	// Parse validUntil date from Redis backup
	validUntil, err := time.Parse("2006-01-02T15:04:05", envelope.ValidUntilDate)
	if err != nil {
		hs.log("⚠️ Failed to parse validUntilDate from Redis: %v - refresh needed", err)
		return true
	}

	now := time.Now().UTC()
	nextRefresh := now.Add(hs.opts.RatesRefreshInterval)

	// Add buffer time (10% of refresh interval) to ensure we don't cut it too close
	bufferTime := time.Duration(float64(hs.opts.RatesRefreshInterval) * 0.1)
	safeNextRefresh := nextRefresh.Add(bufferTime)

	// Refresh if data will expire before next safe refresh
	willExpire := validUntil.Before(safeNextRefresh)

	if willExpire {
		hs.log("⏰ Rates (rev:%d) expire at %v, next refresh at %v (with buffer) - refresh needed",
			envelope.Revision, validUntil.Format(time.RFC3339), safeNextRefresh.Format(time.RFC3339))
	} else {
		hs.log("✅ Rates (rev:%d) valid until %v, next refresh at %v - no refresh needed",
			envelope.Revision, validUntil.Format(time.RFC3339), safeNextRefresh.Format(time.RFC3339))
	}

	return willExpire
}

// addJitter adds random jitter to prevent thundering herd in K8s deployments
// jitterPercent should be between 0.0-1.0 (e.g., 0.1 = ±10%)
func (hs *HedgingService) addJitter(duration time.Duration, jitterPercent float64) time.Duration {
	if jitterPercent <= 0 {
		return duration
	}

	// Calculate jitter range
	jitterRange := float64(duration) * jitterPercent

	// Generate random jitter: ±jitterPercent of original duration
	jitter := (rand.Float64() - 0.5) * 2 * jitterRange

	// Apply jitter, ensure positive result
	result := time.Duration(float64(duration) + jitter)
	if result <= 0 {
		result = duration / 2 // Fallback to half duration if negative
	}

	return result
}

// fetchRates fetches rates from external API using your source code logic
func (hs *HedgingService) fetchRates(ctx context.Context) (*RatesEnvelope, error) {
	url := hs.opts.RatesBaseUrl + "/api/v1/mna/exchange_rate_service/get_rates"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if user, pass, ok := parseBasicAuthPair(hs.opts.RatesBasicAuth); ok {
		req.SetBasicAuth(user, pass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("rates http %d: %s", resp.StatusCode, string(b))
	}
	var env RatesEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// fetchPaymentTerms fetches payment terms from external API using your source code logic
func (hs *HedgingService) fetchPaymentTerms(ctx context.Context) (*PaymentTermsEnvelope, error) {
	url := hs.opts.PaymentTermsBaseUrl + "/api/v1/agency/payment_terms/get"
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if user, pass, ok := parseBasicAuthPair(hs.opts.PaymentTermsBasicAuth); ok {
		req.SetBasicAuth(user, pass)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("terms http %d: %s", resp.StatusCode, string(b))
	}
	var env PaymentTermsEnvelope
	if err := json.Unmarshal(b, &env); err != nil {
		return nil, err
	}
	return &env, nil
}

// hydrateRates hydrates rates data using simplified backup approach
func (hs *HedgingService) hydrateRates(envelope *storage.RatesEnvelope) error {
	if envelope == nil || !envelope.IsSuccessful {
		return errors.New("bad rates envelope or unsuccessful")
	}

	// PRIORITY 1: Store complete provider backup in Redis (single key, simple scaling)
	if err := hs.redisCache.SetRatesBackup(envelope); err != nil {
		hs.log("Warning: failed to store backup in Redis: %v", err)
		// Continue anyway - we'll still process for memory
	}

	// PRIORITY 2: Process and store in memory cache for immediate use
	if err := hs.memCache.DumpRates(envelope); err != nil {
		return fmt.Errorf("failed to process data in memory cache: %w", err)
	}

	hs.log("✅ Stored backup in Redis and processed %d currency pairs in memory",
		len(envelope.CurrencyCollection.Hedged)+len(envelope.CurrencyCollection.Spot))
	return nil
}

// hydrateTerms hydrates payment terms data using your source code logic
func (hs *HedgingService) hydrateTerms(envelope *PaymentTermsEnvelope) error {
	if envelope == nil {
		return errors.New("nil terms")
	}
	m := make(map[int]storage.AgencyPaymentTerm, len(envelope.AgencyPaymentTerms))
	for _, t := range envelope.AgencyPaymentTerms {
		m[t.AgencyId] = storage.AgencyPaymentTerm{
			AgencyId:              t.AgencyId,
			BaseForPaymentDueDate: t.BaseForPaymentDueDate,
			PaymentFrequency:      t.PaymentFrequency,
			DaysAfter:             t.DaysAfter,
		}
	}
	bpdd := make(map[int]string)
	for _, e := range envelope.BaseForPaymentDueDateMap {
		bpdd[e.Id] = e.Name
	}
	freq := make(map[int]string)
	for _, e := range envelope.PaymentFrequencyMap {
		freq[e.Id] = e.Name
	}

	// Store in Redis cache
	termsData := &storage.TermsCacheData{
		ByAgency:    m,
		BpddNames:   bpdd,
		FreqNames:   freq,
		LastRefresh: time.Now().UTC(),
	}
	if err := hs.redisCache.SetTermsBackup(termsData); err != nil {
		hs.log("Warning: failed to store terms in Redis: %v", err)
	}

	err := hs.memCache.DumpTerms(termsData)
	if err != nil {
		return err
	}

	hs.log("Hydrated payment terms count: %d", len(m))
	return nil
}
