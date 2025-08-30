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

func main() {
	// Create service using new clean API
	client, err := hedging.NewClient(
		hedging.WithRedisConfig("tcp://localhost:6379", "", 0),
		hedging.WithRateBaseUrl("https://api.example.com", "user:pass"),
		hedging.WithPaymentTermsBaseUrl("https://api.example.com", "user:pass"),
		hedging.WithRatesRefreshInterval(1*time.Hour),
		hedging.WithTermsRefreshInterval(2*time.Hour),
	)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	// Initialize (loads data, starts schedulers)
	if err := client.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	req := hedging.HedgeCalcReq{
		AgencyId:             22,
		From:                 "AED",
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

	// Keep the service running
	select {}
}
