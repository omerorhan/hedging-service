package hedging

import (
	"github.com/omerorhan/hedging-service/internal/service"
	"github.com/omerorhan/hedging-service/internal/storage"
)

// Client provides a clean public API for the hedging service
type Client struct {
	service *service.HedgingService
}

// NewClient creates a new hedging service client
func NewClient(options ...ServiceOption) (*Client, error) {
	svc, err := service.NewHedgingService(options...)
	if err != nil {
		return nil, err
	}

	return &Client{
		service: svc,
	}, nil
}

// Initialize starts the hedging service
func (c *Client) Initialize() error {
	return c.service.Initialize()
}

// GiveMeRate calculates and returns the hedging rate for a given request
func (c *Client) GiveMeRate(req GiveMeRateReq) (*GiveMeRateResp, error) {
	// Both request and response types are aliased, so no conversion needed
	return c.service.GiveMeRate(req)
}

// Stop gracefully shuts down the service
func (c *Client) Stop() error {
	c.service.Stop()
	return nil
}

// Service options (re-exported for convenience)
type ServiceOption = service.ServiceOption

// Re-export service options for clean API
var (
	WithRateBaseUrl          = service.WithRateBaseUrl
	WithPaymentTermsBaseUrl  = service.WithPaymentTermsBaseUrl
	WithRedisConfig          = service.WithRedisConfig
	WithRatesRefreshInterval = service.WithRatesRefreshInterval
	WithTermsRefreshInterval = service.WithTermsRefreshInterval
	WithLogging              = service.WithLogging
)

// GiveMeRateReq is the clean API request type (alias to internal type)
type GiveMeRateReq = storage.GiveMeRateReq

// GiveMeRateResp is the clean API response type (alias to internal type)
type GiveMeRateResp = storage.GiveMeRateResp

// Advanced types (for advanced users who need internal types)
type (
	RatesEnvelope  = storage.RatesEnvelope
	TermsCacheData = storage.TermsCacheData
)
