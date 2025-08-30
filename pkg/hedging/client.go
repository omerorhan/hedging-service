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
func NewClient(options ...service.ServiceOption) (*Client, error) {
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

// GetRate calculates and returns the hedging rate for a given request
func (c *Client) GetRate(req storage.HedgeCalcReq) (*storage.GiveMeRateResp, error) {
	return c.service.GiveMeRate(req)
}

// GiveMeRate is an alias for GetRate for backward compatibility
func (c *Client) GiveMeRate(req storage.HedgeCalcReq) (*storage.GiveMeRateResp, error) {
	return c.service.GiveMeRate(req)
}

// Stop gracefully shuts down the service
func (c *Client) Stop() error {
	c.service.Stop()
	return nil
}

// Service options (re-exported for convenience)
type ServiceOption = service.ServiceOption

var (
	WithRateBaseUrl          = service.WithRateBaseUrl
	WithPaymentTermsBaseUrl  = service.WithPaymentTermsBaseUrl
	WithRedisConfig          = service.WithRedisConfig
	WithRatesRefreshInterval = service.WithRatesRefreshInterval
	WithTermsRefreshInterval = service.WithTermsRefreshInterval
	WithLogging              = service.WithLogging
)
