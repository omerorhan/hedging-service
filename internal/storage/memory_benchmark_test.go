package storage

import (
	"testing"
	"time"
)

func BenchmarkMemoryCache_GetRates(b *testing.B) {
	cache := NewMemoryCache()

	// Pre-populate with some test data
	testEnvelope := &RatesEnvelope{
		IsSuccessful:   true,
		Revision:       123,
		ValidUntilDate: "2025-12-31T23:59:59",
		TenorCalcDate:  "2025-01-01T00:00:00",
		CurrencyCollection: CurrencyCollection{
			Hedged: []HedgedPair{
				{
					From: "USD",
					To:   "EUR",
					Ten: []Tenor{
						{Days: 30, Rate: 0.85},
						{Days: 60, Rate: 0.84},
					},
				},
			},
			Spot: []SpotPair{
				{
					From: "USD",
					To:   "EUR",
					Rate: 0.86,
				},
			},
		},
	}

	cache.DumpRates(testEnvelope)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = cache.GetRates("USD->EUR")
		}
	})
}

func BenchmarkMemoryCache_GetTerms(b *testing.B) {
	cache := NewMemoryCache()

	// Pre-populate with test data
	testTerms := &TermsCacheData{
		ByAgency: map[int]AgencyPaymentTerm{
			123: {
				AgencyId:              123,
				BaseForPaymentDueDate: 1,
				PaymentFrequency:      1,
				DaysAfter:             5,
			},
		},
		BpddNames: map[int]string{
			1: "CheckIn",
		},
		FreqNames: map[int]string{
			1: "Monthly",
		},
		LastRefresh: time.Now().UTC(),
	}

	cache.DumpTerms(testTerms)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, _, _ = cache.GetTerms(123)
		}
	})
}

func BenchmarkMemoryCache_ConcurrentRatesAndTerms(b *testing.B) {
	cache := NewMemoryCache()

	// Pre-populate both caches
	testEnvelope := &RatesEnvelope{
		IsSuccessful:   true,
		Revision:       123,
		ValidUntilDate: "2025-12-31T23:59:59",
		TenorCalcDate:  "2025-01-01T00:00:00",
		CurrencyCollection: CurrencyCollection{
			Hedged: []HedgedPair{
				{
					From: "USD",
					To:   "EUR",
					Ten: []Tenor{
						{Days: 30, Rate: 0.85},
					},
				},
			},
		},
	}

	testTerms := &TermsCacheData{
		ByAgency: map[int]AgencyPaymentTerm{
			123: {
				AgencyId:              123,
				BaseForPaymentDueDate: 1,
				PaymentFrequency:      1,
				DaysAfter:             5,
			},
		},
		BpddNames:   map[int]string{1: "CheckIn"},
		FreqNames:   map[int]string{1: "Monthly"},
		LastRefresh: time.Now().UTC(),
	}

	cache.DumpRates(testEnvelope)
	cache.DumpTerms(testTerms)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				_, _ = cache.GetRates("USD->EUR")
			} else {
				_, _, _, _ = cache.GetTerms(123)
			}
			i++
		}
	})
}

func BenchmarkMemoryCache_GetLastRefresh(b *testing.B) {
	cache := NewMemoryCache()

	// Pre-populate with test data
	testEnvelope := &RatesEnvelope{
		IsSuccessful:   true,
		Revision:       123,
		ValidUntilDate: "2025-12-31T23:59:59",
		TenorCalcDate:  "2025-01-01T00:00:00",
		CurrencyCollection: CurrencyCollection{
			Hedged: []HedgedPair{
				{
					From: "USD",
					To:   "EUR",
					Ten: []Tenor{
						{Days: 30, Rate: 0.85},
					},
				},
			},
		},
	}

	testTerms := &TermsCacheData{
		ByAgency: map[int]AgencyPaymentTerm{
			123: {AgencyId: 123},
		},
		LastRefresh: time.Now().UTC(),
	}

	cache.DumpRates(testEnvelope)
	cache.DumpTerms(testTerms)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = cache.GetRatesLastRefresh()
			_ = cache.GetTermsLastRefresh()
		}
	})
}

func BenchmarkMemoryCache_GetRatesMetadata(b *testing.B) {
	cache := NewMemoryCache()

	// Pre-populate with test data
	testEnvelope := &RatesEnvelope{
		IsSuccessful:   true,
		Revision:       123,
		ValidUntilDate: "2025-12-31T23:59:59",
		TenorCalcDate:  "2025-01-01T00:00:00",
		CurrencyCollection: CurrencyCollection{
			Hedged: []HedgedPair{
				{
					From: "USD",
					To:   "EUR",
					Ten: []Tenor{
						{Days: 30, Rate: 0.85},
					},
				},
			},
		},
	}

	cache.DumpRates(testEnvelope)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _, _ = cache.GetRatesMetadata()
		}
	})
}
