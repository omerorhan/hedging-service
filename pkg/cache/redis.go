package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCache implements the Cache interface using Redis
type RedisCache struct {
	client *redis.Client
	opts   *CacheOptions
	ctx    context.Context
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(addr string, options ...RedisOption) (*RedisCache, error) {
	opts := DefaultCacheOptions()

	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	// Test connection
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	cache := &RedisCache{
		client: client,
		opts:   opts,
		ctx:    ctx,
	}

	// Apply options
	for _, option := range options {
		option(cache)
	}

	return cache, nil
}

// RedisOption is a function that configures Redis cache options
type RedisOption func(*RedisCache)

// WithRedisOptions sets cache options
func WithRedisOptions(opts *CacheOptions) RedisOption {
	return func(rc *RedisCache) {
		rc.opts = opts
	}
}

// WithContext sets the context for cache operations
func WithContext(ctx context.Context) RedisOption {
	return func(rc *RedisCache) {
		rc.ctx = ctx
	}
}

// ===== Rates Cache Methods =====

func (rc *RedisCache) GetRates() (map[PairKey]RateRec, error) {
	data, err := rc.client.Get(rc.ctx, ratesCacheKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get rates from Redis: %w", err)
	}

	log.Printf("Retrieved data from Redis: %d bytes", len(data))

	var rates map[PairKey]RateRec
	if err := json.Unmarshal(data, &rates); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rates: %w", err)
	}

	return rates, nil
}

func (rc *RedisCache) SetRates(rates map[PairKey]RateRec) error {
	data, err := json.Marshal(rates)
	if err != nil {
		return fmt.Errorf("failed to marshal rates: %w", err)
	}

	log.Printf("Marshaled Rates data size: %d bytes", len(data))

	// Set with expiration (24 hours)
	return rc.client.Set(rc.ctx, ratesCacheKey, data, 24*time.Hour).Err()
}

func (rc *RedisCache) GetRatesRevision() (int, error) {
	rev, err := rc.client.Get(rc.ctx, ratesRevKey).Int()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get rates revision: %w", err)
	}
	return rev, nil
}

func (rc *RedisCache) SetRatesRevision(rev int) error {
	return rc.client.Set(rc.ctx, ratesRevKey, rev, 24*time.Hour).Err()
}

func (rc *RedisCache) GetRatesValidUntil() (time.Time, error) {
	data, err := rc.client.Get(rc.ctx, ratesValidUntilKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to get rates valid until: %w", err)
	}

	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return time.Time{}, fmt.Errorf("failed to unmarshal valid until: %w", err)
	}

	return t, nil
}

func (rc *RedisCache) SetRatesValidUntil(t time.Time) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal valid until: %w", err)
	}

	return rc.client.Set(rc.ctx, ratesValidUntilKey, data, 24*time.Hour).Err()
}

func (rc *RedisCache) GetRatesTenorCalcDate() (time.Time, error) {
	data, err := rc.client.Get(rc.ctx, ratesTenorCalcKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to get tenor calc date: %w", err)
	}

	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return time.Time{}, fmt.Errorf("failed to unmarshal tenor calc date: %w", err)
	}

	return t, nil
}

func (rc *RedisCache) SetRatesTenorCalcDate(t time.Time) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal tenor calc date: %w", err)
	}

	return rc.client.Set(rc.ctx, ratesTenorCalcKey, data, 24*time.Hour).Err()
}

// ===== Terms Cache Methods =====

func (rc *RedisCache) GetPair(pairKey PairKey) (RateRec, bool) {
	rates, err := rc.GetRates()
	if err != nil {
		return RateRec{}, false
	}
	ratePair, ok := rates[pairKey]

	return ratePair, ok
}

func (rc *RedisCache) GetPaymentTermData(req HedgeCalcReq) (*AgencyPaymentTerm, string, string, error) {
	if rc == nil {
		return nil, "", "", fmt.Errorf("service not initialized - call Initialize() first")
	}
	allTerms, err := rc.GetTerms()
	if err != nil {
		return nil, "", "", err
	}
	if allTerms == nil {
		return nil, "", "", fmt.Errorf("Payment terms are not available")
	}

	agencyTerm, ok := allTerms.ByAgency[req.AgencyId]
	if !ok {
		return nil, "", "", fmt.Errorf("agency terms not found")
	}

	bpddName := allTerms.BpddNames[agencyTerm.BaseForPaymentDueDate]
	if bpddName == "" && agencyTerm.BaseForPaymentDueDate == 0 {
		bpddName = Default // e.g., "Default"
	}
	freqName := allTerms.FreqNames[agencyTerm.PaymentFrequency]
	if freqName == "" && agencyTerm.PaymentFrequency == 0 {
		freqName = Monthly // e.g., "Monthly"
	}

	return &agencyTerm, bpddName, freqName, nil
}

func (rc *RedisCache) GetTerms() (*TermsCacheData, error) {
	data, err := rc.client.Get(rc.ctx, termsCacheKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		return nil, fmt.Errorf("failed to get terms from Redis: %w", err)
	}

	var terms TermsCacheData
	if err := json.Unmarshal(data, &terms); err != nil {
		return nil, fmt.Errorf("failed to unmarshal terms: %w", err)
	}

	return &terms, nil
}

func (rc *RedisCache) SetTerms(terms *TermsCacheData) error {
	data, err := json.Marshal(terms)
	if err != nil {
		return fmt.Errorf("failed to marshal terms: %w", err)
	}

	log.Printf("Marshaled Terms data size: %d bytes", len(data))
	// Set with expiration (24 hours)
	return rc.client.Set(rc.ctx, termsCacheKey, data, 24*time.Hour).Err()
}

func (rc *RedisCache) GetRatesLastRefreshed() (time.Time, error) {
	data, err := rc.client.Get(rc.ctx, ratesLastRefreshKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to get rate last refreshed: %w", err)
	}

	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return time.Time{}, fmt.Errorf("failed to unmarshal rates last refreshed time: %w", err)
	}

	return t, nil
}

func (rc *RedisCache) SetRatesLastRefreshed(t time.Time) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal last refreshed: %w", err)
	}

	return rc.client.Set(rc.ctx, ratesLastRefreshKey, data, 24*time.Hour).Err()
}

func (rc *RedisCache) GetTermsLastRefreshed() (time.Time, error) {
	data, err := rc.client.Get(rc.ctx, termsLastRefreshKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to get terms last refreshed: %w", err)
	}

	var t time.Time
	if err := json.Unmarshal(data, &t); err != nil {
		return time.Time{}, fmt.Errorf("failed to unmarshal terms last refreshed time: %w", err)
	}

	return t, nil
}

func (rc *RedisCache) SetTermsLastRefreshed(t time.Time) error {
	data, err := json.Marshal(t)
	if err != nil {
		return fmt.Errorf("failed to marshal last refreshed: %w", err)
	}

	return rc.client.Set(rc.ctx, termsLastRefreshKey, data, 24*time.Hour).Err()
}

func (rc *RedisCache) FlushAll() error {
	return rc.client.FlushAll(rc.ctx).Err()
}

func (rc *RedisCache) Flush(ctx context.Context) error {
	return rc.client.FlushDB(ctx).Err()
}

func (rc *RedisCache) Close() error {
	return rc.client.Close()
}

func (rc *RedisCache) GetStats() (map[string]interface{}, error) {
	raw, err := rc.client.Info(rc.ctx, "memory").Result()
	if err != nil {
		return nil, err
	}

	// Get cache sizes
	ratesSize, _ := rc.client.MemoryUsage(rc.ctx, ratesCacheKey).Result()
	termsSize, _ := rc.client.MemoryUsage(rc.ctx, termsCacheKey).Result()

	return map[string]interface{}{
		"redis_info":       parseRedisInfo(raw),
		"rates_cache_size": ratesSize,
		"terms_cache_size": termsSize,
	}, nil
}

func parseRedisInfo(raw string) map[string]string {
	lines := strings.Split(raw, "\n")
	out := make(map[string]string)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}
