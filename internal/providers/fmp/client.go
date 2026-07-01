package fmp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/bishop-bot/datajobs/internal/logging"
)

const (
	// DefaultBaseURL is the default base URL for the FMP API (stable version).
	DefaultBaseURL = "https://financialmodelingprep.com/stable"

	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second
)

// Client provides access to the FMP API.
type Client struct {
	baseURL    string
	apiKey     string
	timeout    time.Duration
	httpClient *http.Client
	mu         sync.RWMutex
	closed     bool
}

// ClientOption is a functional option for configuring the client.
type ClientOption func(*Client)

// WithBaseURL sets the base URL for the API.
func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithAPIKey sets the API key for authentication.
func WithAPIKey(apiKey string) ClientOption {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) ClientOption {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new FMP API client.
// The API key is read from the FMP_API_KEY environment variable by default,
// but can be overridden with the WithAPIKey option.
func NewClient(opts ...ClientOption) (*Client, error) {
	// Get API key from environment variable
	apiKey := os.Getenv("FMP_API_KEY")
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	c := &Client{
		baseURL:    DefaultBaseURL,
		apiKey:     apiKey,
		timeout:    DefaultTimeout,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	logging.Info("FMP client created",
		"baseURL", c.baseURL,
		"timeout", c.timeout,
	)

	return c, nil
}

// Close closes the client and releases resources.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true
	return nil
}

// Ping checks connectivity to the FMP API.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return ErrClientClosed
	}

	// Make a lightweight request to check connectivity
	// Using the ratios endpoint which is simple and fast
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL("/ratios"), nil)
	if err != nil {
		return err
	}

	req.URL.RawQuery = url.Values{
		"apikey": {c.apiKey},
		"symbol": {"AAPL"},
		"limit":  {"1"},
	}.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &APIError{StatusCode: resp.StatusCode}
	}

	return nil
}

// FinancialRatios fetches financial ratios for a symbol.
// period can be "annual", "quarter", or "ttm" (trailing twelve months).
func (c *Client) FinancialRatios(ctx context.Context, symbol string, period string) (*FinancialRatiosResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	if symbol == "" {
		return nil, &InvalidSymbolError{Symbol: symbol}
	}

	if !isValidPeriod(period) {
		return nil, &InvalidPeriodError{Period: period}
	}

	logging.Info("fetching financial ratios",
		"symbol", symbol,
		"period", period,
		"baseURL", c.baseURL,
	)

	// Build endpoint based on period
	endpoint := "/ratios/" + symbol
	if period == PeriodTTM {
		endpoint = "/ratios/ttm/" + symbol
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL(endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.Error("financial ratios request failed",
			"symbol", symbol,
			"period", period,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"Error Message"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		apiErr := &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
		logging.Error("financial ratios API error",
			"symbol", symbol,
			"period", period,
			"statusCode", resp.StatusCode,
			"message", errResp.Error,
		)
		return nil, apiErr
	}

	var result []FinancialRatiosResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logging.Error("failed to decode financial ratios response",
			"symbol", symbol,
			"period", period,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Return the first (most recent) result
	if len(result) > 0 {
		logging.Info("financial ratios fetched",
			"symbol", result[0].Symbol,
			"date", result[0].Date,
		)
		return &result[0], nil
	}

	return nil, nil
}

// KeyMetrics fetches key financial metrics for a symbol.
// period can be "annual", "quarter", or "ttm" (trailing twelve months).
func (c *Client) KeyMetrics(ctx context.Context, symbol string, period string) (*KeyMetricsResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	if symbol == "" {
		return nil, &InvalidSymbolError{Symbol: symbol}
	}

	if !isValidPeriod(period) {
		return nil, &InvalidPeriodError{Period: period}
	}

	logging.Info("fetching key metrics",
		"symbol", symbol,
		"period", period,
		"baseURL", c.baseURL,
	)

	// Build endpoint based on period
	endpoint := "/key-metrics/" + symbol
	if period == PeriodTTM {
		endpoint = "/key-metrics/ttm/" + symbol
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL(endpoint), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("apikey", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.Error("key metrics request failed",
			"symbol", symbol,
			"period", period,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp struct {
			Error string `json:"Error Message"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		apiErr := &APIError{StatusCode: resp.StatusCode, Message: errResp.Error}
		logging.Error("key metrics API error",
			"symbol", symbol,
			"period", period,
			"statusCode", resp.StatusCode,
			"message", errResp.Error,
		)
		return nil, apiErr
	}

	var result []KeyMetricsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logging.Error("failed to decode key metrics response",
			"symbol", symbol,
			"period", period,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Return the first (most recent) result
	if len(result) > 0 {
		logging.Info("key metrics fetched",
			"symbol", result[0].Symbol,
			"date", result[0].Date,
		)
		return &result[0], nil
	}

	return nil, nil
}

// buildURL constructs the full URL for an endpoint.
func (c *Client) buildURL(path string) string {
	return c.baseURL + path
}

// isValidPeriod checks if the period is valid.
func isValidPeriod(period string) bool {
	return period == PeriodAnnual || period == PeriodQuarter || period == PeriodTTM
}
