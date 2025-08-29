package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/omerorhan/hedging-service/pkg/service"
)

func main() {
	// Create service
	hedgingService, err := service.NewHedgingService(
		service.WithRedisConfig("localhost:6379", "", 0),
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
