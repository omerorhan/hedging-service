package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/omerorhan/hedging-service/internal/storage"
)

func TestDistributedDataManager_LeaderElection(t *testing.T) {
	// Create Redis cache (this will use the actual Redis instance)
	redisCache, err := storage.NewRedisCache("tcp://localhost:6379")
	if err != nil {
		t.Skipf("Skipping test - Redis not available: %v", err)
	}
	defer redisCache.Close()

	memCache := storage.NewMemoryCache()
	opts := DefaultServiceOptions()

	// Create two distributed managers to simulate multiple pods
	manager1 := NewDistributedDataManager(redisCache, memCache, opts)
	manager2 := NewDistributedDataManager(redisCache, memCache, opts)

	// Start both managers
	if err := manager1.Start(); err != nil {
		t.Fatalf("Failed to start manager1: %v", err)
	}
	defer manager1.Stop()

	if err := manager2.Start(); err != nil {
		t.Fatalf("Failed to start manager2: %v", err)
	}
	defer manager2.Stop()

	// Wait for leader election to complete
	time.Sleep(5 * time.Second)

	// Check that exactly one is the leader
	manager1IsLeader := manager1.IsLeader()
	manager2IsLeader := manager2.IsLeader()

	if manager1IsLeader && manager2IsLeader {
		t.Error("Both managers are leaders - should only be one")
	}

	if !manager1IsLeader && !manager2IsLeader {
		t.Error("No manager is the leader - should have one")
	}

	t.Logf("Manager1 is leader: %v, Manager2 is leader: %v", manager1IsLeader, manager2IsLeader)
}

func TestDistributedDataManager_DataVersioning(t *testing.T) {
	redisCache, err := storage.NewRedisCache("tcp://localhost:6379")
	if err != nil {
		t.Skipf("Skipping test - Redis not available: %v", err)
	}
	defer redisCache.Close()

	// Test data versioning
	version1 := &storage.DataVersion{
		RatesRevision: 100,
		LastUpdated:   time.Now().UTC(),
		LastUpdatedBy: "test-pod-1",
	}

	// Set version
	if err := redisCache.SetDataVersion(version1); err != nil {
		t.Fatalf("Failed to set data version: %v", err)
	}

	// Get version
	retrievedVersion, err := redisCache.GetDataVersion()
	if err != nil {
		t.Fatalf("Failed to get data version: %v", err)
	}

	if retrievedVersion == nil {
		t.Fatal("Retrieved version is nil")
	}

	if retrievedVersion.RatesRevision != version1.RatesRevision {
		t.Errorf("Expected rates revision %d, got %d", version1.RatesRevision, retrievedVersion.RatesRevision)
	}

	// TermsRevision is no longer tracked

	if retrievedVersion.LastUpdatedBy != version1.LastUpdatedBy {
		t.Errorf("Expected last updated by %s, got %s", version1.LastUpdatedBy, retrievedVersion.LastUpdatedBy)
	}
}

func TestDistributedDataManager_LeaderLock(t *testing.T) {
	redisCache, err := storage.NewRedisCache("tcp://localhost:6379")
	if err != nil {
		t.Skipf("Skipping test - Redis not available: %v", err)
	}
	defer redisCache.Close()

	podID := "test-pod-123"
	ttl := 30 * time.Second

	// Test acquiring leadership
	acquired, err := redisCache.AcquireLeaderLock(podID, ttl)
	if err != nil {
		t.Fatalf("Failed to acquire leader lock: %v", err)
	}

	if !acquired {
		t.Error("Failed to acquire leader lock")
	}

	// Try to acquire again (should fail)
	acquired2, err := redisCache.AcquireLeaderLock("another-pod", ttl)
	if err != nil {
		t.Fatalf("Failed to acquire leader lock (second attempt): %v", err)
	}

	if acquired2 {
		t.Error("Second pod should not have acquired leadership")
	}

	// Release leadership
	if err := redisCache.ReleaseLeaderLock(podID); err != nil {
		t.Fatalf("Failed to release leader lock: %v", err)
	}

	// Now another pod should be able to acquire
	acquired3, err := redisCache.AcquireLeaderLock("another-pod", ttl)
	if err != nil {
		t.Fatalf("Failed to acquire leader lock (after release): %v", err)
	}

	if !acquired3 {
		t.Error("Should have acquired leadership after previous release")
	}

	// Cleanup
	redisCache.ReleaseLeaderLock("another-pod")
}

func TestHedgingService_WithDistributedManager(t *testing.T) {
	// Create service with distributed manager
	service, err := NewHedgingService(
		WithRedisConfig("tcp://localhost:6379"),
		WithRatesRefreshInterval(1*time.Minute),
		WithTermsRefreshInterval(1*time.Minute),
		WithLogging(true),
	)
	if err != nil {
		t.Skipf("Skipping test - Redis not available: %v", err)
	}
	defer service.Stop()

	// Initialize service
	if err := service.Initialize(); err != nil {
		t.Fatalf("Failed to initialize service: %v", err)
	}

	// Wait a bit for initialization
	time.Sleep(2 * time.Second)

	// Check if service is working
	isLeader := service.IsLeader()
	t.Logf("Service is leader: %v", isLeader)

	// Get revision info
	revisionInfo, err := service.GetLatestRevision()
	if err != nil {
		t.Logf("No revision info available yet: %v", err)
	} else {
		t.Logf("Revision info: %+v", revisionInfo)
	}

	// Test GiveMeRate (should work regardless of leader status)
	req := storage.GiveMeRateReq{
		AgencyId:         1,
		From:             "USD",
		To:               "EUR",
		BookingCreatedAt: time.Now().UTC(),
		CheckIn:          "2025-01-01",
		CheckOut:         "2025-01-02",
		Nonrefundable:    false,
	}

	resp, err := service.GiveMeRate(req)
	if err != nil {
		t.Logf("GiveMeRate failed (expected if no data): %v", err)
	} else {
		t.Logf("GiveMeRate succeeded: %+v", resp)
	}
}

func TestDistributedDataManager_ConcurrentLeadership(t *testing.T) {
	redisCache, err := storage.NewRedisCache("tcp://localhost:6379")
	if err != nil {
		t.Skipf("Skipping test - Redis not available: %v", err)
	}
	defer redisCache.Close()

	// Test concurrent leadership acquisition
	const numGoroutines = 10
	results := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			podID := fmt.Sprintf("concurrent-test-%d", id)
			acquired, err := redisCache.AcquireLeaderLock(podID, 10*time.Second)
			if err != nil {
				t.Errorf("Goroutine %d failed to acquire leadership: %v", id, err)
				results <- false
				return
			}
			results <- acquired
		}(i)
	}

	// Collect results
	acquiredCount := 0
	for i := 0; i < numGoroutines; i++ {
		if <-results {
			acquiredCount++
		}
	}

	// Only one should have acquired leadership
	if acquiredCount != 1 {
		t.Errorf("Expected exactly 1 leadership acquisition, got %d", acquiredCount)
	}

	// Cleanup
	redisCache.ReleaseLeaderLock("concurrent-test-0")
}
