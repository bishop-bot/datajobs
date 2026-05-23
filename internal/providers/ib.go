package providers

import (
	"context"
	"sync"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"

	ibapi "github.com/bishop-bot/ibapi-go"
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

// Errors
var (
	ErrClientClosed = &ClientClosedError{}
)

type ClientClosedError struct{}

func (e *ClientClosedError) Error() string {
	return "IB client is closed"
}