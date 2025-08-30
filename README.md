# Hedging Service

A self-managing hedging service with Redis backup and memory caching for production scaling.

## Installation

```bash
go get github.com/omerorhan/hedging-service
```

## Quick Start

```go
package main

import (
    "log"
    "time"
    
    "github.com/omerorhan/hedging-service"
)

func main() {
    // Create client with clean root-level import
    client, err := hedging.NewClient(
        hedging.WithRedisConfig("localhost:6379", "", 0),
        hedging.WithRateBaseUrl("https://api.example.com", "user:pass"),
        hedging.WithPaymentTermsBaseUrl("https://api.example.com", "user:pass"),
        hedging.WithRatesRefreshInterval(1*time.Hour),
        hedging.WithTermsRefreshInterval(2*time.Hour),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Initialize (loads data, starts schedulers)
    if err := client.Initialize(); err != nil {
        log.Fatal(err)
    }

    // Use the service
    request := hedging.HedgeCalcReq{
        AgencyId:         123,
        From:             "USD",
        To:               "EUR", 
        BookingCreatedAt: time.Now(),
        Nonrefundable:    false,
    }

    response, err := client.GiveMeRate(request)
    if err != nil {
        log.Fatal(err)
    }

    log.Printf("Rate: %.4f, Type: %s", response.Rate, response.Type)
}
```

## Features

- ✅ **Simple Import**: `import "github.com/omerorhan/hedging-service"`
- ✅ **Memory-First Caching**: Ultra-fast in-memory access for production
- ✅ **Redis Backup**: Automatic persistence and scaling support
- ✅ **Self-Managing**: Automatic data refresh and cache management
- ✅ **Production Ready**: Built for horizontal scaling

## Configuration Options

```go
hedging.WithRedisConfig(addr, password, db)          // Redis connection
hedging.WithRateBaseUrl(url, auth)                   // Rates API endpoint
hedging.WithPaymentTermsBaseUrl(url, auth)           // Terms API endpoint  
hedging.WithRatesRefreshInterval(duration)           // Auto-refresh rates
hedging.WithTermsRefreshInterval(duration)           // Auto-refresh terms
hedging.WithLogging(enabled)                         // Enable/disable logging
```

## Architecture

```
External App
     ↓
hedging.NewClient()  ← Clean root-level import
     ↓
internal/service/    ← Business logic
     ↓  
internal/storage/    ← Memory + Redis caching
```

Perfect for microservices and production scaling! 🚀
