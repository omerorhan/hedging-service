package cache

import (
	"time"
)

// Data structures from source code for bulk loading
type PairKey string
type RateRec struct {
	Spot   *float64 `json:"spot"`
	Tenors []Tenor  `json:"tenors"`
}

type Tenor struct {
	Days int     `json:"days"`
	Rate float64 `json:"rate"`
}

type AgencyPaymentTerm struct {
	AgencyId              int `json:"agencyId"`
	BaseForPaymentDueDate int `json:"baseForPaymentDueDate"`
	PaymentFrequency      int `json:"paymentFrequency"`
	DaysAfter             int `json:"daysAfterPaymentPeriod"`
}

type TermsCacheData struct {
	ByAgency    map[int]AgencyPaymentTerm `json:"by_agency"`
	BpddNames   map[int]string            `json:"bpdd_names"`
	FreqNames   map[int]string            `json:"freq_names"`
	LastRefresh time.Time                 `json:"last_refresh"`
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
	DefaultTTL time.Duration `json:"defaultTTL"`
}

func DefaultCacheOptions() *CacheOptions {
	return &CacheOptions{
		DefaultTTL: 24 * time.Hour,
	}
}
