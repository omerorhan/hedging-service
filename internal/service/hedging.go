package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	redisCache         storage.Cache
	memCache           *storage.MemoryCache
	distributedManager *DistributedDataManager
	opts               *ServiceOptions
	ctx                context.Context
	cancel             context.CancelFunc
	wg                 sync.WaitGroup
	mu                 sync.RWMutex
	initialized        bool
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

	// Create distributed data manager for multi-pod coordination
	service.distributedManager = NewDistributedDataManager(redisCache, memCache, opts)

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

	// Start distributed data manager for multi-pod coordination
	if err := hs.distributedManager.Start(); err != nil {
		return fmt.Errorf("failed to start distributed data manager: %w", err)
	}

	hs.initialized = true
	hs.log("✅ Hedging Service initialized successfully")
	return nil
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

// IsLeader returns whether this pod is currently the leader
func (hs *HedgingService) IsLeader() bool {
	if hs.distributedManager == nil {
		return false
	}
	return hs.distributedManager.IsLeader()
}

// GetLatestRevision returns the current revision number and validity information
func (hs *HedgingService) GetLatestRevision() (*RevisionInfo, error) {
	if !hs.initialized {
		return nil, fmt.Errorf("service not initialized - call Initialize() first")
	}

	// Get revision metadata from memory cache
	validUntil, revision, hasData := hs.memCache.GetRatesMetadata()

	if !hasData {
		return nil, fmt.Errorf("no rates data available")
	}

	if revision == 0 {
		return nil, fmt.Errorf("invalid revision number")
	}

	// Get last refresh times from memory cache
	ratesLastRefresh := hs.memCache.GetRatesLastRefresh()
	termsLastRefresh := hs.memCache.GetTermsLastRefresh()

	info := &RevisionInfo{
		Revision:         revision,
		ValidUntil:       validUntil,
		RatesLastRefresh: ratesLastRefresh,
		TermsLastRefresh: termsLastRefresh,
		IsValid:          !validUntil.IsZero() && validUntil.After(time.Now().UTC()),
		TimeUntilExpiry:  validUntil.Sub(time.Now().UTC()),
	}

	return info, nil
}

// Stop gracefully shuts down the service
func (hs *HedgingService) Stop() {
	hs.log("🛑 Stopping Hedging Service...")

	// Stop distributed data manager first
	if hs.distributedManager != nil {
		hs.distributedManager.Stop()
	}

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
