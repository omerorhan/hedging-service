package storage

import (
	"fmt"
	"strings"
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
func (mc *MemoryCache) DumpTerms(terms *TermsCacheData) error {
	if terms == nil {
		return nil
	}
	mc.mu.Lock()
	mc.paymentTerms.byAgency = terms.ByAgency
	mc.paymentTerms.bpddNames = terms.BpddNames
	mc.paymentTerms.freqNames = terms.FreqNames
	mc.paymentTerms.lastRefreshed = terms.LastRefresh
	mc.mu.Unlock()

	return nil
}

func (mc *MemoryCache) DumpRates(envelope *RatesEnvelope) error {
	if envelope == nil || !envelope.IsSuccessful {
		return fmt.Errorf("invalid rates envelope")
	}

	rates := make(map[PairKey]RateRec)

	// Process hedged pairs
	for _, h := range envelope.CurrencyCollection.Hedged {
		k := PairKey(strings.ToUpper(h.From) + "->" + strings.ToUpper(h.To))
		tenors := make([]Tenor, len(h.Ten))
		for i, t := range h.Ten {
			tenors[i] = Tenor{Days: t.Days, Rate: t.Rate}
		}
		rates[k] = RateRec{Tenors: tenors}
	}

	// Process spot pairs
	for _, s := range envelope.CurrencyCollection.Spot {
		k := PairKey(strings.ToUpper(s.From) + "->" + strings.ToUpper(s.To))
		cur := rates[k]
		cur.Spot = &s.Rate
		rates[k] = cur
	}

	// Parse dates
	validUntil, err := time.Parse("2006-01-02T15:04:05", envelope.ValidUntilDate)
	if err != nil {
		return fmt.Errorf("failed to parse validUntilDate: %w", err)
	}

	tenorCalcDate, err := time.Parse("2006-01-02T15:04:05", envelope.TenorCalcDate)
	if err != nil {
		return fmt.Errorf("failed to parse tenorCalcDate: %w", err)
	}

	// Store all data atomically in memory
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.rates.pairs = rates
	mc.rates.rev = envelope.Revision
	mc.rates.validUntilUTC = validUntil
	mc.rates.tenorCalcDate = tenorCalcDate
	mc.rates.lastRefreshed = time.Now().UTC()

	return nil
}

func (mc *MemoryCache) GetRates(pairKey PairKey) (RateData, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	rc, found := mc.rates.pairs[pairKey]
	if !found {
		return RateData{}, fmt.Errorf("no such rate: %s", pairKey)
	}

	return RateData{
		Pair:           rc,
		ValidUntil:     mc.rates.validUntilUTC,
		RevisionNumber: mc.rates.rev,
	}, nil
}

func (mc *MemoryCache) GetTerms(agencyId int) (*AgencyPaymentTerm, string, string, error) {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	term, ok := mc.paymentTerms.byAgency[agencyId]
	if !ok {
		return nil, "", "", fmt.Errorf("agency terms not found")
	}
	bpddName := mc.paymentTerms.bpddNames[term.BaseForPaymentDueDate]
	if bpddName == "" && term.BaseForPaymentDueDate == 0 {
		bpddName = Default // e.g., "Default"
	}
	freqName := mc.paymentTerms.freqNames[term.PaymentFrequency]
	if freqName == "" && term.PaymentFrequency == 0 {
		freqName = Monthly // e.g., "Monthly"
	}

	return &term, bpddName, freqName, nil
}
