package ib

import (
	"context"
	"sync"

	ibapi "github.com/bishop-bot/ibapi-go"
	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
)

// Client wraps the IB Web API client with optional authentication support.
type Client struct {
	client *ibapi.Client

	// Authentication
	authenticator Authenticator
	authMu        sync.Mutex
	authenticated bool

	// Configuration
	cfg    config.IBConfig
	mu     sync.RWMutex
	closed bool
}

// NewClient creates a new IB client instance.
// This is the preferred constructor for dependency injection.
// If auth credentials are configured, an authenticator will be created.
func NewClient(cfg config.IBConfig) (*Client, error) {
	opts := []ibapi.ClientOption{
		ibapi.WithBaseURL(cfg.BaseURL),
		ibapi.WithInsecureSkipVerify(cfg.InsecureSkipVerify),
		ibapi.WithTimeout(cfg.Timeout),
	}

	client, err := ibapi.NewClient(opts...)
	if err != nil {
		return nil, err
	}

	c := &Client{
		client: client,
		cfg:    cfg,
	}

	// Initialize authenticator if credentials are provided
	if cfg.Username != "" && cfg.Password != "" {
		auth, err := NewIBGatewayAuthenticator(cfg)
		if err != nil {
			logging.Warn("failed to create IB authenticator", "error", err)
		} else if auth != nil {
			c.authenticator = auth
			logging.Info("IB authenticator configured",
				"username", cfg.Username,
				"base_url", cfg.BaseURL,
			)
		}
	}

	logging.Info("IB client created",
		"base_url", cfg.BaseURL,
		"insecure", cfg.InsecureSkipVerify,
		"auth_configured", c.authenticator != nil,
	)

	return c, nil
}

// Client returns the underlying ibapi client.
func (c *Client) Client() *ibapi.Client {
	return c.client
}

// Ping pings the IB Client Portal Gateway.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return ErrClientClosed
	}

	_, err := c.client.Session().Ping(ctx)
	return err
}

// AuthStatus returns the current authentication status.
func (c *Client) AuthStatus(ctx context.Context) (*ibapi.AuthStatusResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	return c.client.Session().AuthStatus(ctx)
}

// IsConnected checks if the client is connected to the gateway.
func (c *Client) IsConnected(ctx context.Context) bool {
	if err := c.Ping(ctx); err != nil {
		logging.Warn("IB connection check failed", "error", err)
		return false
	}
	return true
}

// IsAuthenticated checks if the client is authenticated to IB.
func (c *Client) IsAuthenticated(ctx context.Context) bool {
	if c.authenticator == nil {
		// No authenticator configured, assume not needed
		return true
	}

	status, err := c.AuthStatus(ctx)
	if err != nil {
		logging.Warn("failed to check auth status", "error", err)
		return false
	}

	return status.Authenticated
}

// EnsureAuthenticated ensures the client is authenticated.
// Returns nil if already authenticated or auth succeeds.
// Returns error if auth is required but fails.
func (c *Client) EnsureAuthenticated(ctx context.Context) error {
	if c.authenticator == nil {
		return nil
	}

	c.authMu.Lock()
	defer c.authMu.Unlock()

	// Skip the pre-check if we haven't authenticated yet
	// Just attempt authentication directly
	if !c.authenticated {
		if err := c.authenticator.Authenticate(ctx); err != nil {
			return err
		}

		// Update client with authenticated HTTP client
		c.client, _ = ibapi.NewClient(
			ibapi.WithBaseURL(c.cfg.BaseURL),
			ibapi.WithInsecureSkipVerify(c.cfg.InsecureSkipVerify),
			ibapi.WithTimeout(c.cfg.Timeout),
			ibapi.WithHTTPClient(c.authenticator.HTTPClient()),
		)
		c.authenticated = true
		return nil
	}

	// Already authenticated, verify with a quick ping
	if _, err := c.client.Session().Ping(ctx); err != nil {
		c.authenticated = false
		return c.EnsureAuthenticated(ctx)
	}

	return nil
}

// Close closes the IB client and releases resources.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	if c.authenticator != nil {
		c.authenticator.Close()
	}

	logging.Info("closing IB client")
	return c.client.Close()
}

// Reconnect attempts to reconnect the IB client.
func (c *Client) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return ErrClientClosed
	}

	// Close existing client
	c.client.Close()

	// Reset auth state
	c.authenticated = false

	// Create new client
	newClient, err := ibapi.NewClient(
		ibapi.WithBaseURL(c.cfg.BaseURL),
		ibapi.WithInsecureSkipVerify(c.cfg.InsecureSkipVerify),
		ibapi.WithTimeout(c.cfg.Timeout),
	)
	if err != nil {
		return err
	}

	// If we had an authenticator, re-authenticate
	if c.authenticator != nil {
		c.authenticator.Close()
		auth, err := NewIBGatewayAuthenticator(c.cfg)
		if err != nil {
			return err
		}
		c.authenticator = auth

		// Authenticate and update client
		if err := c.authenticator.Authenticate(context.Background()); err != nil {
			return err
		}
		newClient, _ = ibapi.NewClient(
			ibapi.WithBaseURL(c.cfg.BaseURL),
			ibapi.WithInsecureSkipVerify(c.cfg.InsecureSkipVerify),
			ibapi.WithTimeout(c.cfg.Timeout),
			ibapi.WithHTTPClient(c.authenticator.HTTPClient()),
		)
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
func (c *Client) HistoricalData(ctx context.Context, req HistoricalDataRequest) (*ibapi.HistoricalDataResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	if c.authenticator == nil {
		logging.Warn("IB HistoricalData called without authenticator - request may fail if gateway requires auth")
	}

	logging.Info("fetching historical data from IB",
		"conid", req.Conid,
		"exchange", req.Exchange,
		"period", req.Period,
		"bar", req.Bar,
		"baseURL", c.cfg.BaseURL,
	)

	resp, err := c.client.MarketData().HistoricalData(ctx, req.ToIBRequest())
	if err != nil {
		logging.Error("IB HistoricalData request failed",
			"conid", req.Conid,
			"exchange", req.Exchange,
			"period", req.Period,
			"bar", req.Bar,
			"error", err.Error(),
			"baseURL", c.cfg.BaseURL,
		)
		return nil, err
	}

	return resp, nil
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