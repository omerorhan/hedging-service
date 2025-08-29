package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/omerorhan/hedging-service/pkg/cache"
)

// Data structures from your source code for API responses
type RatesEnvelope struct {
	CurrencyCollection CurrencyCollection `json:"currencyCollection"`
	Revision           int                `json:"revision"`
	ValidUntilDate     string             `json:"validUntilDate"`
	TenorCalcDate      string             `json:"tenorCalculationDate"`
	IsSuccessful       bool               `json:"isSuccessful"`
	Error              any                `json:"error"`
}

type CurrencyCollection struct {
	Hedged []HedgedPair `json:"hedged"`
	Spot   []SpotPair   `json:"spot"`
}

type HedgedPair struct {
	From string  `json:"fromCurrency"`
	To   string  `json:"toCurrency"`
	Ten  []Tenor `json:"tenors"`
}

type SpotPair struct {
	From string  `json:"fromCurrency"`
	To   string  `json:"toCurrency"`
	Rate float64 `json:"rate"`
}

type Tenor struct {
	Days int     `json:"days"`
	Rate float64 `json:"rate"`
}

type PaymentTermsEnvelope struct {
	AgencyPaymentTerms       []AgencyPaymentTerm `json:"agencyPaymentTerms"`
	BaseForPaymentDueDateMap []EnumMap           `json:"baseForPaymentDueDateMap"`
	PaymentFrequencyMap      []EnumMap           `json:"paymentFrequencyMap"`
	ErrorMessage             string              `json:"errorMessage"`
}

type AgencyPaymentTerm struct {
	AgencyId              int `json:"agencyId"`
	BaseForPaymentDueDate int `json:"baseForPaymentDueDate"`
	PaymentFrequency      int `json:"paymentFrequency"`
	DaysAfter             int `json:"daysAfterPaymentPeriod"`
}

type EnumMap struct {
	Id   int    `json:"id"`
	Name string `json:"name"`
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
	redisCache  cache.Cache
	memCache    *cache.MemoryCache
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
		RedisAddr:          "localhost:6379",
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
	redisCache, err := cache.NewRedisCache(
		opts.RedisAddr,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redis cache: %w", err)
	}

	// Create in-memory cache
	memCache := cache.NewMemoryCache()
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
func WithRedisConfig(addr, password string, db int) ServiceOption {
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
	hs.log("📥 Loading existing data from Redis...")

	// Load rates from Redis and store in memory
	rates, err := hs.loadRatesFromRedis()
	if err != nil {
		hs.log("⚠️ Warning: Failed to load rates from Redis: %v", err)
	} else if rates != nil {
		hs.log("✅ Loaded %d rate pairs from Redis", len(rates))

		// Store each rate pair in memory cache
		err := hs.memCache.SetRates(rates)
		if err != nil {
			return err
		}
	}
	if rates == nil {
		hs.ForceRefresh()
	}

	// Load payment terms from Redis using your existing data structure
	terms, err := hs.loadPaymentTermsFromRedis()
	if err != nil {
		hs.log("⚠️ Warning: Failed to load payment terms from Redis: %v", err)
	} else if terms != nil {
		hs.log("✅ Loaded payment terms for %d agencies from Redis", len(terms.ByAgency))

		err := hs.memCache.SetPaymentTerms(terms)
		if err != nil {
			return err
		}
	}

	if rev, err := hs.GetRatesRevision(); err == nil && rev != 0 {
		err := hs.memCache.SetRevision(rev)
		if err != nil {
			return err
		}
	}

	if validUntil, err := hs.GetRatesValidUntil(); err == nil && !validUntil.IsZero() {
		err := hs.memCache.SetRatesValidUntil(validUntil)
		if err != nil {
			return err
		}
	}

	if tenorCalc, err := hs.GetRatesTenorCalcDate(); err == nil && !tenorCalc.IsZero() {
		err := hs.memCache.SetRatesTenorCalcDate(tenorCalc)
		if err != nil {
			return err
		}
	}
	if ratesLastRefreshed, err := hs.GetRatesLastRefreshed(); err == nil && !ratesLastRefreshed.IsZero() {
		err := hs.memCache.SetRatesLastRefreshed(ratesLastRefreshed)
		if err != nil {
			return err
		}
	}
	if termsLastRefreshed, err := hs.GetTermsLastRefreshed(); err == nil && !termsLastRefreshed.IsZero() {
		err := hs.memCache.SetTermsLastRefreshed(termsLastRefreshed)
		if err != nil {
			return err
		}
	}
	hs.log("📊 Cache initialization completed")
	return nil
}

// loadRatesFromRedis loads rates from Redis using your existing structure
func (hs *HedgingService) loadRatesFromRedis() (map[cache.PairKey]cache.RateRec, error) {
	return hs.redisCache.GetRates()
}

// loadPaymentTermsFromRedis loads payment terms from Redis using your existing structure
func (hs *HedgingService) loadPaymentTermsFromRedis() (*cache.TermsCacheData, error) {
	return hs.redisCache.GetTerms()
}

func (hs *HedgingService) GetRatesRevision() (int, error) {
	return hs.redisCache.GetRatesRevision()
}

func (hs *HedgingService) GetRatesValidUntil() (time.Time, error) {
	return hs.redisCache.GetRatesValidUntil()
}

func (hs *HedgingService) GetRatesTenorCalcDate() (time.Time, error) {
	return hs.redisCache.GetRatesTenorCalcDate()
}

func (hs *HedgingService) GetRatesLastRefreshed() (time.Time, error) {
	return hs.redisCache.GetRatesLastRefreshed()
}
func (hs *HedgingService) GetTermsLastRefreshed() (time.Time, error) {
	return hs.redisCache.GetTermsLastRefreshed()
}

// startSchedulers starts background goroutines to keep data fresh
func (hs *HedgingService) startSchedulers() {
	hs.log("⏰ Starting schedulers...")

	// Start rates refresh scheduler
	hs.wg.Add(1)
	go hs.ratesRefreshScheduler()

	// Start payment terms refresh scheduler
	hs.wg.Add(1)
	go hs.paymentTermsRefreshScheduler()

	hs.log("✅ Schedulers started")
}

func (hs *HedgingService) ratesRefreshScheduler() {
	defer hs.wg.Done()

	ticker := time.NewTicker(hs.opts.RatesRefreshInterval)
	defer ticker.Stop()

	hs.log("🔄 Rates refresh scheduler started (interval: %v)", hs.opts.RatesRefreshInterval)

	for {
		select {
		case <-hs.ctx.Done():
			hs.log("🛑 Rates refresh scheduler stopped")
			return
		case <-ticker.C:
			hs.refreshRates()
		}
	}
}

// paymentTermsRefreshScheduler periodically refreshes payment terms
func (hs *HedgingService) paymentTermsRefreshScheduler() {
	defer hs.wg.Done()

	ticker := time.NewTicker(hs.opts.TermsRefreshInterval) // Less frequent
	defer ticker.Stop()

	hs.log("🔄 Payment terms refresh scheduler started (interval: %v)", hs.opts.TermsRefreshInterval)

	for {
		select {
		case <-hs.ctx.Done():
			hs.log("🛑 Payment terms refresh scheduler stopped")
			return
		case <-ticker.C:
			hs.refreshPaymentTerms()
		}
	}
}

func (hs *HedgingService) refreshRates() {
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

func (hs *HedgingService) GiveMeRate(req cache.HedgeCalcReq) (*cache.GiveMeRateResp, error) {
	if !hs.initialized {
		return nil, fmt.Errorf("service not initialized - call Initialize() first")
	}

	term, bpddName, freqName, err := hs.redisCache.GetPaymentTermData(req)
	if err != nil {
		log.Printf("Payment Term could not be found through Redis: %v\n", err)

		term, bpddName, freqName, err = hs.memCache.GetPaymentTermData(req)
		if err != nil {
			log.Printf("Payment Term Also could not be found through memory: %v", err)
			return nil, err
		}
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

	rate := 1.00 // same
	rateType := "spot"
	var explain string
	if strings.ToUpper(req.From) != strings.ToUpper(req.To) {
		rc, ok := hs.redisCache.GetPair(pairKey)
		if !ok {
			log.Printf("Rate Pairs (from: %s-> to: %s) not be found through Redis: %v\n", req.From, req.To, err)
			log.Printf("...checking pairs on memory...")
			if rc, ok = hs.memCache.GetPair(pairKey); !ok {
				return nil, fmt.Errorf("Rate Pairs (from: %s-> to: %s) also cannnot be found through Memory\n", req.From, req.To)
			}
		}

		// Select tenor
		ten, _ := selectTenor(rc.Tenors, dth) // adapt to your struct field name (Tenors vs tenors)

		if rc.Spot != nil && (ten.Rate == 0 || ten.Days == 0) {
			rate = *rc.Spot
			rateType = "spot"
		} else {
			rate = ten.Rate
			rateType = "hedged"
		}

		explain = fmt.Sprintf("%s => Base=%s; Lead=%dd + DaysDue(%d) => DaysToHedge=%d; tenor=%dd",
			ddExplain, baseSrc, lead, dd, dth, ten.Days)
	}
	validUntil, err := hs.redisCache.GetRatesValidUntil()
	if err != nil {
		return nil, fmt.Errorf("rate validUntil could not be found through Redis")
	}

	if validUntil.IsZero() || validUntil.Before(time.Now().UTC()) {
		return nil, fmt.Errorf("rate validUntil has passed, it's not valid anymore")
	}

	revisionNumber, err := hs.redisCache.GetRatesRevision()
	if err != nil {
		log.Printf("Rate revision could not be found through Redis: %v\n", err)
		revisionNumber = hs.memCache.GetRevision()

		if revisionNumber == 0 {
			return nil, fmt.Errorf("rate revision could not be found through memory: %v", err)
		}
	}

	resp := &cache.GiveMeRateResp{
		Rate:           rate,
		Type:           rateType,
		RevisionNumber: revisionNumber,
		Explain:        explain,
	}
	return resp, nil
}

func (hs *HedgingService) ForceRefresh() {
	hs.log("🔄 Force refreshing all data...")
	hs.refreshRates()
	hs.refreshPaymentTerms()
	hs.log("✅ Force refresh completed")
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
		log.Printf("[HedgingService] "+format, args...)
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

// hydrateRates hydrates rates data using your source code logic
func (hs *HedgingService) hydrateRates(envelope *RatesEnvelope) error {
	if envelope == nil || !envelope.IsSuccessful {
		return errors.New("bad rates envelope or unsuccessful")
	}
	rec := make(map[cache.PairKey]cache.RateRec)
	for _, h := range envelope.CurrencyCollection.Hedged {
		k := cache.PairKey(strings.ToUpper(h.From) + "->" + strings.ToUpper(h.To))
		tenors := make([]cache.Tenor, len(h.Ten))
		for i, t := range h.Ten {
			tenors[i] = cache.Tenor{Days: t.Days, Rate: t.Rate}
		}
		rec[k] = cache.RateRec{Tenors: tenors}
	}
	for _, s := range envelope.CurrencyCollection.Spot {
		k := cache.PairKey(strings.ToUpper(s.From) + "->" + strings.ToUpper(s.To))
		cur := rec[k]
		cur.Spot = &[]float64{s.Rate}[0]
		rec[k] = cur
	}

	// Store in Redis cache using bulk method
	if err := hs.redisCache.SetRates(rec); err != nil {
		hs.log("Warning: failed to store rates in Redis: %v", err)
	}
	if err := hs.redisCache.SetRatesRevision(envelope.Revision); err != nil {
		log.Printf("Warning: failed to store rates revision in Redis: %v", err)
	}
	if err := hs.redisCache.SetRatesValidUntil(mustParseISODate(envelope.ValidUntilDate)); err != nil {
		log.Printf("Warning: failed to store rates valid until in Redis: %v", err)
	}
	if err := hs.redisCache.SetRatesTenorCalcDate(mustParseISODate(envelope.TenorCalcDate)); err != nil {
		log.Printf("Warning: failed to store rates tenor calc date in Redis: %v", err)
	}
	if err := hs.redisCache.SetRatesLastRefreshed(time.Now().UTC()); err != nil {
		log.Printf("Warning: failed to store last refreshed in Redis: %v", err)
	}

	err := hs.memCache.SetRates(rec)
	if err != nil {
		return err
	}

	err = hs.memCache.SetRevision(envelope.Revision)
	if err != nil {
		return err
	}

	err = hs.memCache.SetRatesValidUntil(mustParseISODate(envelope.ValidUntilDate))
	if err != nil {
		return err
	}

	err = hs.memCache.SetRatesTenorCalcDate(mustParseISODate(envelope.TenorCalcDate))
	if err != nil {
		return err
	}

	err = hs.memCache.SetRatesLastRefreshed(time.Now().UTC())
	if err != nil {
		return err
	}

	hs.log("Hydrated rates count: %d", len(rec))
	return nil
}

// hydrateTerms hydrates payment terms data using your source code logic
func (hs *HedgingService) hydrateTerms(envelope *PaymentTermsEnvelope) error {
	if envelope == nil {
		return errors.New("nil terms")
	}
	m := make(map[int]cache.AgencyPaymentTerm, len(envelope.AgencyPaymentTerms))
	for _, t := range envelope.AgencyPaymentTerms {
		m[t.AgencyId] = cache.AgencyPaymentTerm{
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

	// Store in Redis cache using bulk method
	termsData := &cache.TermsCacheData{
		ByAgency:  m,
		BpddNames: bpdd,
		FreqNames: freq,
	}
	if err := hs.redisCache.SetTerms(termsData); err != nil {
		hs.log("Warning: failed to store terms in Redis: %v", err)
	}
	now := time.Now().UTC()
	if err := hs.redisCache.SetTermsLastRefreshed(now); err != nil {
		log.Printf("Warning: failed to store last refreshed in Redis: %v", err)
	}

	err := hs.memCache.SetPaymentTerms(termsData)
	if err != nil {
		return err
	}

	err = hs.memCache.SetTermsLastRefreshed(now)
	if err != nil {
		return err
	}

	hs.log("Hydrated payment terms count: %d", len(m))
	return nil
}
