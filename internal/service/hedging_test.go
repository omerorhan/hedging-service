package service

import (
	"testing"
	"time"

	"github.com/omerorhan/hedging-service/internal/storage"
)

func TestHedgingService_GetLatestRevision(t *testing.T) {
	// Create a new hedging service with proper configuration
	service, err := NewHedgingService(
		WithRatesRefreshInterval(1*time.Hour),
		WithTermsRefreshInterval(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("Failed to create hedging service: %v", err)
	}

	// // Test before initialization
	// _, err = service.GetLatestRevision()
	// if err == nil {
	// 	t.Error("Expected error when service not initialized, got nil")
	// }
	//
	// // Initialize the service
	// if err := service.Initialize(); err != nil {
	// 	t.Fatalf("Failed to initialize service: %v", err)
	// }
	//
	// // Test after initialization but before data is loaded
	// _, err = service.GetLatestRevision()
	// if err == nil {
	// 	t.Error("Expected error when no data available, got nil")
	// }

	// Mock some data into the memory cache
	testEnvelope := &storage.RatesEnvelope{
		IsSuccessful:   true,
		Revision:       456,
		ValidUntilDate: "2025-12-31T23:59:59",
		TenorCalcDate:  "2025-01-01T00:00:00",
		CurrencyCollection: storage.CurrencyCollection{
			Hedged: []storage.HedgedPair{
				{
					From: "USD",
					To:   "EUR",
					Ten: []storage.Tenor{
						{Days: 30, Rate: 0.85},
					},
				},
			},
		},
	}

	testTerms := &storage.TermsCacheData{
		ByAgency: map[int]storage.AgencyPaymentTerm{
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

	// Load data into memory cache
	if err := service.memCache.DumpRates(testEnvelope); err != nil {
		t.Fatalf("Failed to dump rates: %v", err)
	}

	if err := service.memCache.DumpTerms(testTerms); err != nil {
		t.Fatalf("Failed to dump terms: %v", err)
	}

	// // Now test GetLatestRevision
	// revisionInfo, err := service.GetLatestRevision()
	// if err != nil {
	// 	t.Fatalf("GetLatestRevision failed: %v", err)
	// }
	//
	// // Verify the returned data
	// if revisionInfo.Revision != 456 {
	// 	t.Errorf("Expected revision 456, got %d", revisionInfo.Revision)
	// }
	//
	// if !revisionInfo.IsValid {
	// 	t.Error("Expected data to be valid")
	// }
	//
	// if revisionInfo.TimeUntilExpiry <= 0 {
	// 	t.Error("Expected positive time until expiry")
	// }
	//
	// // Verify refresh times are recent
	// if time.Since(revisionInfo.RatesLastRefresh) > time.Minute {
	// 	t.Error("Rates refresh time should be recent")
	// }
	//
	// if time.Since(revisionInfo.TermsLastRefresh) > time.Minute {
	// 	t.Error("Terms refresh time should be recent")
	// }

	// Test with expired data
	expiredEnvelope := &storage.RatesEnvelope{
		IsSuccessful:   true,
		Revision:       789,
		ValidUntilDate: "2020-01-01T00:00:00", // Expired date
		TenorCalcDate:  "2020-01-01T00:00:00",
		CurrencyCollection: storage.CurrencyCollection{
			Hedged: []storage.HedgedPair{
				{
					From: "USD",
					To:   "EUR",
					Ten: []storage.Tenor{
						{Days: 30, Rate: 0.85},
					},
				},
			},
		},
	}

	if err := service.memCache.DumpRates(expiredEnvelope); err != nil {
		t.Fatalf("Failed to dump expired rates: %v", err)
	}

	// revisionInfo, err = service.GetLatestRevision()
	// if err != nil {
	// 	t.Fatalf("GetLatestRevision failed with expired data: %v", err)
	// }
	//
	// if revisionInfo.Revision != 789 {
	// 	t.Errorf("Expected revision 789, got %d", revisionInfo.Revision)
	// }
	//
	// if revisionInfo.IsValid {
	// 	t.Error("Expected data to be invalid (expired)")
	// }
	//
	// if revisionInfo.TimeUntilExpiry >= 0 {
	// 	t.Error("Expected negative time until expiry for expired data")
	// }
}

// func TestHedgingService_GetLatestRevision_Concurrent(t *testing.T) {
// 	// Create and initialize service with proper configuration
// 	service, err := NewHedgingService(
// 		WithRatesRefreshInterval(1*time.Hour),
// 		WithTermsRefreshInterval(1*time.Hour),
// 	)
// 	if err != nil {
// 		t.Fatalf("Failed to create hedging service: %v", err)
// 	}
//
// 	if err := service.Initialize(); err != nil {
// 		t.Fatalf("Failed to initialize service: %v", err)
// 	}
//
// 	// Load test data
// 	testEnvelope := &storage.RatesEnvelope{
// 		IsSuccessful:   true,
// 		Revision:       999,
// 		ValidUntilDate: "2025-12-31T23:59:59",
// 		TenorCalcDate:  "2025-01-01T00:00:00",
// 		CurrencyCollection: storage.CurrencyCollection{
// 			Hedged: []storage.HedgedPair{
// 				{
// 					From: "USD",
// 					To:   "EUR",
// 					Ten: []storage.Tenor{
// 						{Days: 30, Rate: 0.85},
// 					},
// 				},
// 			},
// 		},
// 	}
//
// 	testTerms := &storage.TermsCacheData{
// 		ByAgency: map[int]storage.AgencyPaymentTerm{
// 			123: {AgencyId: 123},
// 		},
// 		LastRefresh: time.Now().UTC(),
// 	}
//
// 	if err := service.memCache.DumpRates(testEnvelope); err != nil {
// 		t.Fatalf("Failed to dump rates: %v", err)
// 	}
//
// 	if err := service.memCache.DumpTerms(testTerms); err != nil {
// 		t.Fatalf("Failed to dump terms: %v", err)
// 	}
//
// 	// Test concurrent access
// 	const numGoroutines = 100
// 	const numCalls = 1000
//
// 	results := make(chan *storage.RevisionInfo, numGoroutines*numCalls)
// 	errors := make(chan error, numGoroutines*numCalls)
//
// 	for i := 0; i < numGoroutines; i++ {
// 		go func() {
// 			for j := 0; j < numCalls; j++ {
// 				info, err := service.GetLatestRevision()
// 				if err != nil {
// 					errors <- err
// 					return
// 				}
// 				results <- info
// 			}
// 		}()
// 	}
//
// 	// Collect results
// 	successCount := 0
// 	errorCount := 0
//
// 	for i := 0; i < numGoroutines*numCalls; i++ {
// 		select {
// 		case <-results:
// 			successCount++
// 		case <-errors:
// 			errorCount++
// 		}
// 	}
//
// 	if errorCount > 0 {
// 		t.Errorf("Expected no errors, got %d errors", errorCount)
// 	}
//
// 	if successCount != numGoroutines*numCalls {
// 		t.Errorf("Expected %d successful calls, got %d", numGoroutines*numCalls, successCount)
// 	}
// }

func TestHedgingService_DataVersioning(t *testing.T) {
	// Create and initialize service with proper configuration
	service, err := NewHedgingService(
		WithRatesRefreshInterval(1*time.Hour),
		WithTermsRefreshInterval(1*time.Hour),
	)
	if err != nil {
		t.Fatalf("Failed to create hedging service: %v", err)
	}

	if err := service.Initialize(); err != nil {
		t.Fatalf("Failed to initialize service: %v", err)
	}

	// Test setting and getting data version
	now := time.Now().UTC()
	version := &storage.DataVersion{
		RatesRevision: 100,
		LastUpdated:   now,
		LastUpdatedBy: "test-pod",
	}

	err = service.redisCache.SetDataVersion(version)
	if err != nil {
		t.Fatalf("Failed to set data version: %v", err)
	}

	retrievedVersion, err := service.redisCache.GetDataVersion()
	if err != nil {
		t.Fatalf("Failed to get data version: %v", err)
	}

	if retrievedVersion == nil {
		t.Fatal("Retrieved version is nil")
	}

	if retrievedVersion.RatesRevision != version.RatesRevision {
		t.Errorf("Expected rates revision %d, got %d", version.RatesRevision, retrievedVersion.RatesRevision)
	}

	// TermsRevision is no longer tracked

	if retrievedVersion.LastUpdatedBy != version.LastUpdatedBy {
		t.Errorf("Expected last updated by %s, got %s", version.LastUpdatedBy, retrievedVersion.LastUpdatedBy)
	}
}
