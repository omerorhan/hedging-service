package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omerorhan/hedging-service"
)

/*
This example demonstrates how to configure the HedgingService with all available timing parameters.

Available Configuration Options:
- WithRatesRefreshInterval: How often rates are refreshed (dynamic based on ValidUntil)
- WithTermsRefreshInterval: How often terms are refreshed (static interval)
- WithRateRefreshBuffer: How long before ValidUntil to refresh rates (default: 11 minutes)
- WithNonLeaderSyncInterval: How often non-leader pods check for data updates
- WithLeaderElectionInterval: How often leader election is checked
- WithLockLeaderTTL: How long the leader lock lasts before expiring
- WithHTTPTimeout: Timeout for HTTP API calls
- WithLogging: Enable/disable logging for debugging

The service automatically handles:
- Leader election and failover
- Data synchronization between pods
- Smart refresh timing based on data validity
- Graceful shutdown and cleanup
*/

func main() {
	// Example 1: Production configuration with balanced timing
	client, err := hedging.NewClient(
		hedging.WithRedisConfig("tcp://localhost:6379"),
		hedging.WithRateBaseUrl("https://api.example.com", "user:pass"),
		hedging.WithPaymentTermsBaseUrl("https://api.example.com", "user:pass"),
		// Data refresh intervals
		hedging.WithRatesRefreshInterval(1*time.Hour),  // Rates refresh every 1 hour
		hedging.WithTermsRefreshInterval(2*time.Hour),  // Terms refresh every 2 hours
		hedging.WithRateRefreshBuffer(-11*time.Minute), // Refresh rates 15 minutes before ValidUntil
		// Distributed system timing
		hedging.WithNonLeaderSyncInterval(5*time.Second),   // Non-leader sync every 5 seconds
		hedging.WithLeaderElectionInterval(20*time.Second), // Leader election check every 20 seconds
		hedging.WithLockLeaderTTL(2*time.Minute),           // Leader lock expires in 2 minutes
		// HTTP and timeout settings
		hedging.WithHTTPTimeout(30*time.Second), // HTTP timeout 30 seconds
		// Optional: Enable logging for debugging
		hedging.WithLogging(true),
	)

	/*
		// Example 2: Development configuration with fast intervals for testing
		client, err := hedging.NewClient(
			hedging.WithRedisConfig("tcp://localhost:6379"),
			hedging.WithRateBaseUrl("https://api.example.com", "user:pass"),
			hedging.WithPaymentTermsBaseUrl("https://api.example.com", "user:pass"),
		hedging.WithRatesRefreshInterval(5*time.Minute),    // Faster rates refresh
		hedging.WithTermsRefreshInterval(30*time.Minute),   // Faster terms refresh
		hedging.WithRateRefreshBuffer(5*time.Minute),       // Shorter buffer for testing
			hedging.WithNonLeaderSyncInterval(1*time.Second),   // Very fast sync
			hedging.WithLeaderElectionInterval(5*time.Second),  // Fast leader election
			hedging.WithLockLeaderTTL(30*time.Second),          // Shorter lock TTL
			hedging.WithHTTPTimeout(10*time.Second),            // Shorter HTTP timeout
			hedging.WithLogging(true),
		)

		// Example 3: High-load production with conservative intervals
		client, err := hedging.NewClient(
			hedging.WithRedisConfig("tcp://localhost:6379"),
			hedging.WithRateBaseUrl("https://api.example.com", "user:pass"),
			hedging.WithPaymentTermsBaseUrl("https://api.example.com", "user:pass"),
		hedging.WithRatesRefreshInterval(2*time.Hour),      // Slower rates refresh
		hedging.WithTermsRefreshInterval(6*time.Hour),      // Slower terms refresh
		hedging.WithRateRefreshBuffer(30*time.Minute),      // Longer buffer for high-load
			hedging.WithNonLeaderSyncInterval(30*time.Second),  // Slower sync
			hedging.WithLeaderElectionInterval(60*time.Second), // Slower leader election
			hedging.WithLockLeaderTTL(5*time.Minute),           // Longer lock TTL
			hedging.WithHTTPTimeout(60*time.Second),            // Longer HTTP timeout
			hedging.WithLogging(false),                         // Disable logging in production
		)
	*/
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	// Initialize (loads data, starts schedulers)
	if err := client.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	req := hedging.GiveMeRateReq{
		AgencyId:             22,
		From:                 "TRY",
		To:                   "EUR",
		Nonrefundable:        true,
		CheckIn:              "2025-09-02",
		CheckOut:             "2025-09-04",
		CancellationDeadline: time.Now().UTC().Add(-11 * time.Hour * 24 * 2),
		BookingCreatedAt:     time.Now().UTC(),
	}
	rate, err := client.GiveMeRate(req)
	if err != nil {
		log.Fatalf("error : %v", err)
	}
	rateJson, _ := json.Marshal(rate)
	log.Printf("HedgingService.GiveMeRate: %s", rateJson)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("Shutting down...")
		client.Stop()
		os.Exit(0)
	}()

	// Your application logic here
	log.Println("Hedging service is running...")
	log.Println("Configuration used:")
	log.Printf("  - Rates refresh: %v", 1*time.Hour)
	log.Printf("  - Terms refresh: %v", 2*time.Hour)
	log.Printf("  - Rate refresh buffer: %v", 15*time.Minute)
	log.Printf("  - Non-leader sync: %v", 5*time.Second)
	log.Printf("  - Leader election: %v", 20*time.Second)
	log.Printf("  - Leader lock TTL: %v", 2*time.Minute)
	log.Printf("  - HTTP timeout: %v", 30*time.Second)

	// Keep the service running
	select {}
}
