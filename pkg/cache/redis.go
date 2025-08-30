package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

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

	u, err := url.Parse(addr)
	if err != nil {
		panic("can't parse url for redis " + err.Error())
	}
	var passwd string
	if u.User != nil {
		passwd, _ = u.User.Password()
	}
	db := 0
	if 1 < len(u.Path) {
		db, err = strconv.Atoi(u.Path[1:])
		if err != nil {
			panic(fmt.Sprintf("can't convert string into int for redis db; %s; %s", err.Error(), string(addr)))
		}
	}

	client := redis.NewClient(&redis.Options{
		Network:  u.Scheme,
		Addr:     u.Host,
		Password: passwd,
		DB:       db,
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

func (rc *RedisCache) SetRatesBackup(envelope *RatesEnvelope) error {
	if envelope == nil {
		return fmt.Errorf("rates envelope is nil")
	}

	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal rates envelope: %w", err)
	}

	return rc.client.Set(rc.ctx, ratesBackupKey, data, rc.opts.DefaultTTL).Err()
}

func (rc *RedisCache) GetRatesBackup() (*RatesEnvelope, error) {
	data, err := rc.client.Get(rc.ctx, ratesBackupKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No backup found
		}
		return nil, fmt.Errorf("failed to get backup from Redis: %w", err)
	}

	var envelope RatesEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("failed to unmarshal backup: %w", err)
	}

	return &envelope, nil
}

func (rc *RedisCache) SetTermsBackup(terms *TermsCacheData) error {
	data, err := json.Marshal(terms)
	if err != nil {
		return fmt.Errorf("failed to marshal terms: %w", err)
	}

	// Set with expiration (24 hours)
	return rc.client.Set(rc.ctx, termsBackupKey, data, rc.opts.DefaultTTL).Err()
}

func (rc *RedisCache) GetTermsBackup() (*TermsCacheData, error) {
	data, err := rc.client.Get(rc.ctx, termsBackupKey).Bytes()
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

func (rc *RedisCache) Close() error {
	return rc.client.Close()
}
