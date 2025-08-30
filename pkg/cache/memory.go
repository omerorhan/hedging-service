package cache

import (
	"fmt"
	"log"
	"sync"
	"time"
)

type (
	ratesCache struct {
		// mu            sync.RWMutex
		rev           int
		validUntilUTC time.Time
		tenorCalcDate time.Time
		pairs         map[PairKey]RateRec
		lastRefreshed time.Time
	}
	termsCache struct {
		// mu            sync.RWMutex
		byAgency      map[int]AgencyPaymentTerm
		bpddNames     map[int]string
		freqNames     map[int]string
		lastRefreshed time.Time
	}
)

// MemoryCache implements the Cache interface using in-memory storage
type MemoryCache struct {
	mu           sync.RWMutex
	rates        ratesCache
	paymentTerms termsCache
}

// NewMemoryCache creates a new in-memory cache instance
func NewMemoryCache() *MemoryCache {
	cache := &MemoryCache{
		rates: ratesCache{pairs: make(map[PairKey]RateRec)},
		paymentTerms: termsCache{
			byAgency:  make(map[int]AgencyPaymentTerm),
			bpddNames: make(map[int]string),
			freqNames: make(map[int]string),
		},
	}
	return cache
}

func (mc *MemoryCache) SetRates(rates map[PairKey]RateRec) error {
	mc.mu.Lock()
	mc.rates.pairs = rates
	mc.mu.Unlock()

	log.Printf("Set rates from Redis into memory")
	return nil
}

func (mc *MemoryCache) GetPair(pairKey PairKey) (RateRec, bool) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	rc, ok := mc.rates.pairs[pairKey]
	return rc, ok
}

// SetPaymentTerms caches payment terms
func (mc *MemoryCache) SetPaymentTerms(terms *TermsCacheData) error {
	if terms == nil {
		return nil
	}
	mc.mu.Lock()
	mc.paymentTerms.byAgency = terms.ByAgency
	mc.paymentTerms.bpddNames = terms.BpddNames
	mc.paymentTerms.freqNames = terms.FreqNames
	mc.mu.Unlock()
	log.Printf("Set payment terms from Redis into memory")
	return nil
}

func (mc *MemoryCache) SetRevision(rev int) error {
	mc.mu.Lock()
	mc.rates.rev = rev
	mc.mu.Unlock()

	return nil
}

func (mc *MemoryCache) GetRevision() int {
	mc.mu.RLock()
	mc.mu.RUnlock()

	return mc.rates.rev
}

func (mc *MemoryCache) GetRatesValidUntil() time.Time {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	return mc.rates.validUntilUTC
}
func (mc *MemoryCache) SetRatesValidUntil(validUntil time.Time) error {
	mc.mu.Lock()
	mc.rates.validUntilUTC = validUntil
	mc.mu.Unlock()

	return nil
}

func (mc *MemoryCache) SetRatesTenorCalcDate(tenorCalc time.Time) error {
	mc.mu.Lock()
	mc.rates.tenorCalcDate = tenorCalc
	mc.mu.Unlock()

	return nil
}

func (mc *MemoryCache) SetRatesLastRefreshed(lastRefreshed time.Time) error {
	mc.mu.Lock()
	mc.rates.lastRefreshed = lastRefreshed
	mc.mu.Unlock()

	return nil
}

func (mc *MemoryCache) SetTermsLastRefreshed(lastRefreshed time.Time) error {
	mc.mu.Lock()
	mc.paymentTerms.lastRefreshed = lastRefreshed
	mc.mu.Unlock()

	return nil
}

func (mc *MemoryCache) GetPaymentTermData(req HedgeCalcReq) (*AgencyPaymentTerm, string, string, error) {
	mc.mu.RLock()
	term, ok := mc.paymentTerms.byAgency[req.AgencyId]
	bpddName := mc.paymentTerms.bpddNames[term.BaseForPaymentDueDate]
	if bpddName == "" && term.BaseForPaymentDueDate == 0 {
		bpddName = Default // e.g., "Default"
	}
	freqName := mc.paymentTerms.freqNames[term.PaymentFrequency]
	if freqName == "" && term.PaymentFrequency == 0 {
		freqName = Monthly // e.g., "Monthly"
	}
	mc.mu.RUnlock()
	if !ok {
		return nil, "", "", fmt.Errorf("agency terms not found")
	}

	return &term, bpddName, freqName, nil
}
