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

	// Cleanup function to remove test data from Redis
	cleanup := func() {
		redisCache.CleanupTestData()
	}
	defer cleanup()

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

	// Cleanup function to remove test data from Redis
	cleanup := func() {
		redisCache.CleanupTestData()
	}
	defer cleanup()

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

	// Cleanup function to remove test data from Redis
	cleanup := func() {
		redisCache.CleanupTestData()
	}
	defer cleanup()

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

func TestDistributedDataManager_DataSync(t *testing.T) {
	redisCache, err := storage.NewRedisCache("tcp://localhost:6379")
	if err != nil {
		t.Skipf("Skipping test - Redis not available: %v", err)
	}
	defer redisCache.Close()

	// Cleanup function to remove test data from Redis
	cleanup := func() {
		redisCache.CleanupTestData()
	}
	defer cleanup() // Ensure cleanup runs even if test fails

	memCache := storage.NewMemoryCache()
	opts := DefaultServiceOptions()
	opts.SyncInterval = 1 * time.Second // Fast sync for testing

	// Create distributed manager
	manager := NewDistributedDataManager(redisCache, memCache, opts)

	// Test needsDataSync when no data exists
	if !manager.needsDataSync() {
		t.Error("Should need sync when no local data exists")
	}

	// Set some data in Redis
	version := &storage.DataVersion{
		RatesRevision: 100,
		LastUpdated:   time.Now().UTC(),
		LastUpdatedBy: "test-pod",
	}
	if err := redisCache.SetDataVersion(version); err != nil {
		t.Fatalf("Failed to set data version: %v", err)
	}

	// Test needsDataSync when local data is older
	if !manager.needsDataSync() {
		t.Error("Should need sync when local data is older than Redis")
	}

	// Simulate local data with same revision
	_, _, hasData := memCache.GetRatesMetadata()
	if hasData {
		// If there's already data, we need to test differently
		t.Log("Local data already exists, testing with newer Redis version")

		// Set newer version in Redis
		newerVersion := &storage.DataVersion{
			RatesRevision: 200,
			LastUpdated:   time.Now().UTC(),
			LastUpdatedBy: "test-pod-2",
		}
		if err := redisCache.SetDataVersion(newerVersion); err != nil {
			t.Fatalf("Failed to set newer data version: %v", err)
		}

		if !manager.needsDataSync() {
			t.Error("Should need sync when Redis has newer data")
		}
	}

	t.Log("Data sync test completed")
}

func TestDistributedDataManager_RealDataSync(t *testing.T) {
	redisCache, err := storage.NewRedisCache("tcp://localhost:6379")
	if err != nil {
		t.Skipf("Skipping test - Redis not available: %v", err)
	}
	defer redisCache.Close()

	// Cleanup function to remove test data from Redis
	cleanup := func() {
		redisCache.CleanupTestData()
	}
	defer cleanup()

	// Create two distributed managers to simulate leader and follower pods
	memCache1 := storage.NewMemoryCache()
	memCache2 := storage.NewMemoryCache()
	opts := DefaultServiceOptions()
	opts.SyncInterval = 1 * time.Second // Fast sync for testing

	manager1 := NewDistributedDataManager(redisCache, memCache1, opts)
	manager2 := NewDistributedDataManager(redisCache, memCache2, opts)

	// Start both managers
	if err := manager1.Start(); err != nil {
		t.Fatalf("Failed to start manager1: %v", err)
	}
	defer manager1.Stop()

	if err := manager2.Start(); err != nil {
		t.Fatalf("Failed to start manager2: %v", err)
	}
	defer manager2.Stop()

	// Wait for leader election
	time.Sleep(2 * time.Second)

	// Determine which is leader and which is follower
	var leader *DistributedDataManager
	var leaderMemCache, followerMemCache *storage.MemoryCache

	if manager1.IsLeader() {
		leader = manager1
		leaderMemCache = memCache1
		followerMemCache = memCache2
		t.Log("Manager1 is leader, Manager2 is follower")
	} else if manager2.IsLeader() {
		leader = manager2
		leaderMemCache = memCache2
		followerMemCache = memCache1
		t.Log("Manager2 is leader, Manager1 is follower")
	} else {
		t.Fatal("No leader elected")
	}

	// Simulate leader updating data by directly setting data in Redis
	// (This simulates what happens when leader fetches from external API)
	t.Log("🔄 Simulating leader updating data...")

	// Create mock rates data
	mockRates := &storage.RatesEnvelope{
		Revision:       100,
		ValidUntilDate: time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02T15:04:05"),
		TenorCalcDate:  time.Now().UTC().Format("2006-01-02T15:04:05"),
		IsSuccessful:   true,
		CurrencyCollection: storage.CurrencyCollection{
			Hedged: []storage.HedgedPair{
				{
					From: "USD",
					To:   "EUR",
					Ten: []storage.Tenor{
						{Days: 30, Rate: 0.84},
						{Days: 60, Rate: 0.83},
					},
				},
			},
			Spot: []storage.SpotPair{
				{
					From: "USD",
					To:   "EUR",
					Rate: 0.85,
				},
			},
		},
	}

	// Leader stores data in Redis (simulating what happens in refreshRatesFromAPI)
	if err := redisCache.SetRatesBackup(mockRates); err != nil {
		t.Fatalf("Failed to set rates in Redis: %v", err)
	}

	// Update data version (simulating what happens in refreshRatesFromAPI)
	version := &storage.DataVersion{
		RatesRevision: 100,
		LastUpdated:   time.Now().UTC(),
		LastUpdatedBy: leader.GetPodID(),
	}
	if err := redisCache.SetDataVersion(version); err != nil {
		t.Fatalf("Failed to set data version: %v", err)
	}

	// Leader also updates its local cache
	if err := leaderMemCache.DumpRates(mockRates); err != nil {
		t.Fatalf("Failed to update leader's local cache: %v", err)
	}

	t.Log("✅ Leader has updated data (revision 100)")

	// Check that follower doesn't have the data yet
	_, followerRevision, followerHasData := followerMemCache.GetRatesMetadata()
	if followerHasData {
		t.Logf("⚠️ Follower already has data (revision %d) - this might be from previous test", followerRevision)
	} else {
		t.Log("📝 Follower has no data yet (expected)")
	}

	// Wait for follower to sync (should happen within syncInterval)
	t.Log("⏳ Waiting for follower to sync...")
	time.Sleep(3 * time.Second)

	// Check if follower synced the data
	_, followerRevisionAfter, followerHasDataAfter := followerMemCache.GetRatesMetadata()
	if !followerHasDataAfter {
		t.Error("❌ Follower failed to sync data from Redis")
	} else {
		t.Logf("✅ Follower synced data (revision %d)", followerRevisionAfter)
	}

	// Verify the data is the same
	if followerRevisionAfter != 100 {
		t.Errorf("Expected follower to have revision 100, got %d", followerRevisionAfter)
	}

	// Test that follower can now serve requests with the synced data
	// (This would be tested through the actual service, but we can verify the cache has data)
	if followerRevisionAfter == 100 {
		t.Log("✅ Follower is ready to serve requests with synced data")
	}

	// Test updating data again to ensure continuous sync
	t.Log("🔄 Testing continuous sync - updating data again...")

	// Update with new revision
	mockRates2 := &storage.RatesEnvelope{
		Revision:       200,
		ValidUntilDate: time.Now().UTC().Add(24 * time.Hour).Format("2006-01-02T15:04:05"),
		TenorCalcDate:  time.Now().UTC().Format("2006-01-02T15:04:05"),
		IsSuccessful:   true,
		CurrencyCollection: storage.CurrencyCollection{
			Hedged: []storage.HedgedPair{
				{
					From: "USD",
					To:   "EUR",
					Ten: []storage.Tenor{
						{Days: 30, Rate: 0.86},
						{Days: 60, Rate: 0.85},
					},
				},
			},
			Spot: []storage.SpotPair{
				{
					From: "USD",
					To:   "EUR",
					Rate: 0.87, // Updated rate
				},
			},
		},
	}

	// Update Redis with new data
	if err := redisCache.SetRatesBackup(mockRates2); err != nil {
		t.Fatalf("Failed to update rates in Redis: %v", err)
	}

	version2 := &storage.DataVersion{
		RatesRevision: 200,
		LastUpdated:   time.Now().UTC(),
		LastUpdatedBy: leader.GetPodID(),
	}
	if err := redisCache.SetDataVersion(version2); err != nil {
		t.Fatalf("Failed to update data version: %v", err)
	}

	// Update leader's cache
	if err := leaderMemCache.DumpRates(mockRates2); err != nil {
		t.Fatalf("Failed to update leader's local cache: %v", err)
	}

	t.Log("✅ Leader updated data to revision 200")

	// Wait for follower to sync again
	time.Sleep(3 * time.Second)

	// Check if follower synced the new data
	_, followerRevisionFinal, followerHasDataFinal := followerMemCache.GetRatesMetadata()
	if !followerHasDataFinal {
		t.Error("❌ Follower failed to sync updated data")
	} else if followerRevisionFinal != 200 {
		t.Errorf("Expected follower to have revision 200, got %d", followerRevisionFinal)
	} else {
		t.Logf("✅ Follower synced updated data (revision %d)", followerRevisionFinal)
	}

	t.Log("🎉 Real data sync test completed successfully!")
}
