package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
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

// Data versioning methods
func (rc *RedisCache) SetDataVersion(version *DataVersion) error {
	data, err := json.Marshal(version)
	if err != nil {
		return fmt.Errorf("failed to marshal data version: %w", err)
	}
	return rc.client.Set(rc.ctx, dataVersionKey, data, rc.opts.DefaultTTL).Err()
}

func (rc *RedisCache) GetDataVersion() (*DataVersion, error) {
	data, err := rc.client.Get(rc.ctx, dataVersionKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // No version found
		}
		return nil, fmt.Errorf("failed to get data version: %w", err)
	}

	var version DataVersion
	if err := json.Unmarshal(data, &version); err != nil {
		return nil, fmt.Errorf("failed to unmarshal data version: %w", err)
	}

	return &version, nil
}

// Leader election methods
func (rc *RedisCache) AcquireLeaderLock(podID string, ttl time.Duration) (bool, error) {
	// Use SET with NX (only if not exists) and EX (expiration) for atomic leader election
	result := rc.client.SetNX(rc.ctx, leaderLockKey, podID, ttl)
	if result.Err() != nil {
		return false, fmt.Errorf("failed to acquire leader lock: %w", result.Err())
	}
	return result.Val(), nil
}

func (rc *RedisCache) ReleaseLeaderLock(podID string) error {
	// Only release if we're the current leader
	currentLeader, err := rc.client.Get(rc.ctx, leaderLockKey).Result()
	if err != nil {
		if err == redis.Nil {
			return nil // No leader to release
		}
		return fmt.Errorf("failed to check current leader: %w", err)
	}

	if currentLeader == podID {
		return rc.client.Del(rc.ctx, leaderLockKey).Err()
	}
	return nil
}

func (rc *RedisCache) Close() error {
	return rc.client.Close()
}
