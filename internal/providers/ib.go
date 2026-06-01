package providers

import (
	"context"
	"sync"

	ibapi "github.com/bishop-bot/ibapi-go"
	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
)

// IBClient wraps the IB Web API client with singleton lifecycle.
type IBClient struct {
	client   *ibapi.Client
	cfg      config.IBConfig
	mu       sync.RWMutex
	closed   bool
}

// Singleton instance and lock.
var (
	ibClient     *IBClient
	ibClientOnce sync.Once
	ibClientErr  error
)

// InitIB initializes the singleton IB client.
// Safe to call multiple times; only initializes once.
func InitIB(cfg config.IBConfig) error {
	ibClientOnce.Do(func() {
		ibClient, ibClientErr = newIBClient(cfg)
	})
	return ibClientErr
}

// newIBClient creates a new IB client instance.
func newIBClient(cfg config.IBConfig) (*IBClient, error) {
	opts := []ibapi.ClientOption{
		ibapi.WithBaseURL(cfg.BaseURL),
		ibapi.WithInsecureSkipVerify(cfg.InsecureSkipVerify),
		ibapi.WithTimeout(cfg.Timeout),
	}

	client, err := ibapi.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	logging.Info("IB client initialized",
		"base_url", cfg.BaseURL,
		"insecure", cfg.InsecureSkipVerify,
	)

	return &IBClient{
		client: client,
		cfg:    cfg,
	}, nil
}

// GetIB returns the singleton IB client instance.
func GetIB() *IBClient {
	return ibClient
}

// Client returns the underlying ibapi client.
func (c *IBClient) Client() *ibapi.Client {
	return c.client
}

// Ping pings the IB Client Portal Gateway.
func (c *IBClient) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return ErrClientClosed
	}

	_, err := c.client.Session().Ping(ctx)
	return err
}

// AuthStatus returns the current authentication status.
func (c *IBClient) AuthStatus(ctx context.Context) (*ibapi.AuthStatusResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	return c.client.Session().AuthStatus(ctx)
}

// IsConnected checks if the client is connected to the gateway.
func (c *IBClient) IsConnected(ctx context.Context) bool {
	if err := c.Ping(ctx); err != nil {
		logging.Warn("IB connection check failed", "error", err)
		return false
	}
	return true
}

// Close closes the IB client and releases resources.
func (c *IBClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	logging.Info("closing IB client")
	return c.client.Close()
}

// Reconnect attempts to reconnect the IB client.
func (c *IBClient) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return ErrClientClosed
	}

	// Close existing client
	c.client.Close()

	// Create new client
	newClient, err := ibapi.NewClient(
		ibapi.WithBaseURL(c.cfg.BaseURL),
		ibapi.WithInsecureSkipVerify(c.cfg.InsecureSkipVerify),
		ibapi.WithTimeout(c.cfg.Timeout),
	)
	if err != nil {
		return err
	}

	c.client = newClient
	logging.Info("IB client reconnected", "base_url", c.cfg.BaseURL)
	return nil
}

// HistoricalData fetches historical market data for a contract.
// Parameters:
//   - conid: Contract ID (e.g., "265598" for AAPL)
//   - exchange: Exchange code (e.g., "SMART" or "NASDAQ")
//   - period: Time period (e.g., "1d", "1w", "1M")
//   - bar: Bar size (e.g., "1min", "5mins", "1hour", "1day")
//   - startTime: Optional start time in YYYYMMDD-HH:MM:SS format
//   - outsideRth: Include data outside regular trading hours
func (c *IBClient) HistoricalData(ctx context.Context, req HistoricalDataRequest) (*ibapi.HistoricalDataResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	logging.Debug("fetching historical data",
		"conid", req.Conid,
		"exchange", req.Exchange,
		"period", req.Period,
		"bar", req.Bar,
	)

	return c.client.MarketData().HistoricalData(ctx, req.ToIBRequest())
}

// HistoricalDataRequest wraps ibapi.HistoricalDataRequest with helper constructors.
type HistoricalDataRequest struct {
	// Conid is the contract ID
	Conid string
	// Exchange is the exchange code (e.g., "SMART", "NASDAQ", "XNAS")
	Exchange string
	// Period is the time period (e.g., "1d", "1w", "1M", "1y")
	Period string
	// Bar is the bar size (e.g., "1min", "5mins", "1hour", "1day")
	Bar string
	// StartTime is optional start time in YYYYMMDD-HH:MM:SS format
	StartTime string
	// OutsideRth includes data outside regular trading hours
	OutsideRth bool
	// Source is an optional data source
	Source string
}

// ToIBRequest converts to the ibapi request format.
func (r HistoricalDataRequest) ToIBRequest() ibapi.HistoricalDataRequest {
	return ibapi.HistoricalDataRequest{
		Conid:      r.Conid,
		Exchange:   r.Exchange,
		Period:     r.Period,
		Bar:        r.Bar,
		StartTime:  r.StartTime,
		OutsideRth: r.OutsideRth,
		Source:     r.Source,
	}
}

// HistoricalDataBar represents a single bar of historical data.
type HistoricalDataBar struct {
	// Open is the opening price
	Open float64
	// High is the highest price
	High float64
	// Low is the lowest price
	Low float64
	// Close is the closing price
	Close float64
	// Volume is the trading volume
	Volume float64
	// Timestamp is the bar timestamp (Unix milliseconds)
	Timestamp int64
}

// Errors
var (
	ErrClientClosed = &ClientClosedError{}
)

type ClientClosedError struct{}

func (e *ClientClosedError) Error() string {
	return "IB client is closed"
}