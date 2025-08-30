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
