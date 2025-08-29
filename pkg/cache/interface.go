package cache

import (
	"context"
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
	ByAgency  map[int]AgencyPaymentTerm `json:"by_agency"`
	BpddNames map[int]string            `json:"bpdd_names"`
	FreqNames map[int]string            `json:"freq_names"`
}

// Cache defines the interface for caching operations
type Cache interface {
	GetPair(pairKey PairKey) (RateRec, bool)
	GetPaymentTermData(req HedgeCalcReq) (*AgencyPaymentTerm, string, string, error)
	SetRates(rates map[PairKey]RateRec) error
	GetRates() (map[PairKey]RateRec, error)
	GetRatesRevision() (int, error)
	SetRatesRevision(rev int) error
	GetRatesValidUntil() (time.Time, error)
	SetRatesValidUntil(t time.Time) error
	GetRatesTenorCalcDate() (time.Time, error)
	SetRatesTenorCalcDate(t time.Time) error
	GetTerms() (*TermsCacheData, error)
	SetTerms(terms *TermsCacheData) error
	GetRatesLastRefreshed() (time.Time, error)
	SetRatesLastRefreshed(t time.Time) error
	GetTermsLastRefreshed() (time.Time, error)
	SetTermsLastRefreshed(t time.Time) error
	FlushAll() error
	Flush(ctx context.Context) error
	GetStats() (map[string]interface{}, error)
	Close() error
}

type CacheOptions struct {
	DefaultTTL      time.Duration `json:"defaultTTL"`
	MaxMemory       int64         `json:"maxMemory"` // in bytes
	MaxKeys         int64         `json:"maxKeys"`
	CleanupInterval time.Duration `json:"cleanupInterval"`
	Compression     bool          `json:"compression"`
}

func DefaultCacheOptions() *CacheOptions {
	return &CacheOptions{
		DefaultTTL:      24 * time.Hour,
		MaxMemory:       100 * 1024 * 1024, // 100MB
		MaxKeys:         10000,
		CleanupInterval: 1 * time.Hour,
		Compression:     false,
	}
}
