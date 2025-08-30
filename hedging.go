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

// GiveMeRate is an alias for GetRate for backward compatibility
func (c *Client) GiveMeRate(req GiveMeRateReq) (*GiveMeRateResp, error) {
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

// Re-export common types for convenience
type (
	GiveMeRateReq  = service.GiveMeRateReq
	GiveMeRateResp = service.GiveMeRateResp
	RatesEnvelope  = storage.RatesEnvelope
	TermsCacheData = storage.TermsCacheData
)
