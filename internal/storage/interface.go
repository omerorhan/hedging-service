package storage

import (
	"time"
)

// Cache defines the interface for caching operations
type Cache interface {
	SetRatesBackup(envelope *RatesEnvelope) error
	GetRatesBackup() (*RatesEnvelope, error)
	SetTermsBackup(terms *TermsCacheData) error
	GetTermsBackup() (*TermsCacheData, error)

	// Data versioning methods
	SetDataVersion(version *DataVersion) error
	GetDataVersion() (*DataVersion, error)

	// Leader election methods
	AcquireLeaderLock(podID string, ttl time.Duration) (bool, error)
	ReleaseLeaderLock(podID string) error

	Close() error
}

type CacheOptions struct {
	DefaultTTL time.Duration
}

func DefaultCacheOptions() *CacheOptions {
	return &CacheOptions{
		DefaultTTL: 24 * time.Hour,
	}
}
