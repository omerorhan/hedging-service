package main

import (
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omerorhan/hedging-service/pkg/cache"
	"github.com/omerorhan/hedging-service/pkg/service"
)

func main() {
	// Create service
	hedgingService, err := service.NewHedgingService(
		service.WithRedisConfig("tcp://localhost:6379/0", "", 0),
		service.WithRateBaseUrl("https://api.example.com", "user:pass"),
		service.WithPaymentTermsBaseUrl("https://api.example.com", "user:pass"),
		service.WithRatesRefreshInterval(1*time.Hour),
		service.WithTermsRefreshInterval(2*time.Hour),
	)
	if err != nil {
		log.Fatalf("Failed to create service: %v", err)
	}

	// Initialize (loads data, starts schedulers)
	if err := hedgingService.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	req := cache.HedgeCalcReq{
		AgencyId:             22,
		From:                 "AED",
		To:                   "EUR",
		Nonrefundable:        true,
		CheckIn:              "2025-09-02",
		CheckOut:             "2025-09-04",
		CancellationDeadline: time.Now().UTC().Add(-11 * time.Hour * 24 * 2),
		BookingCreatedAt:     time.Now().UTC(),
	}
	rate, err := hedgingService.GiveMeRate(req)
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
		hedgingService.Stop()
		os.Exit(0)
	}()

	// Your application logic here
	log.Println("Hedging service is running...")

	// Keep the service running
	select {}
}
