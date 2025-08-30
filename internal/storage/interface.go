package storage

import (
	"time"
)

type PairKey string
type RateRec struct {
	Spot   *float64
	Tenors []Tenor
}

type Tenor struct {
	Days int
	Rate float64
}

type AgencyPaymentTerm struct {
	AgencyId              int
	BaseForPaymentDueDate int
	PaymentFrequency      int
	DaysAfter             int
}

type TermsCacheData struct {
	ByAgency    map[int]AgencyPaymentTerm
	BpddNames   map[int]string
	FreqNames   map[int]string
	LastRefresh time.Time
}

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
