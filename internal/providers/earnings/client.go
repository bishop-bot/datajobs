package earnings

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
	"github.com/bishop-bot/datajobs/internal/logging"
)

const (
	// DefaultBaseURL is the default base URL for the Earnings API.
	DefaultBaseURL = "https://api.earningsapi.com"

	// DefaultTimeout is the default request timeout.
	DefaultTimeout = 30 * time.Second
)

// Client provides access to the Earnings API.
type Client struct {
	baseURL   string
	apiKey    string
	timeout   time.Duration
	httpClient *http.Client
	mu        sync.RWMutex
	closed    bool
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

// NewClient creates a new Earnings API client.
// The API key is read from the EARNINGS_API_KEY environment variable by default,
// but can be overridden with the WithAPIKey option.
func NewClient(opts ...ClientOption) (*Client, error) {
	// Get API key from environment variable
	apiKey := config.GetEnv("EARNINGS_API_KEY", "")
	if apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	c := &Client{
		baseURL:   DefaultBaseURL,
		apiKey:    apiKey,
		timeout:   DefaultTimeout,
		httpClient: &http.Client{Timeout: DefaultTimeout},
	}

	// Apply options
	for _, opt := range opts {
		opt(c)
	}

	logging.Info("earnings client created",
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

// Ping checks connectivity to the Earnings API.
func (c *Client) Ping(ctx context.Context) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return ErrClientClosed
	}

	// Make a lightweight request to check connectivity
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL("/v1/calendar/earnings"), nil)
	if err != nil {
		return err
	}

	req.URL.RawQuery = url.Values{"date": {"today"}, "apikey": {c.apiKey}}.Encode()

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

// EarningsCalendar fetches earnings calendar data for a specific date.
// The date can be a specific date string (YYYY-MM-DD) or relative values like "today", "yesterday", "tomorrow".
func (c *Client) EarningsCalendar(ctx context.Context, date CalendarDate) (*EarningsCalendarResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	logging.Info("fetching earnings calendar",
		"date", date.Value,
		"baseURL", c.baseURL,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL("/v1/calendar/earnings"), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.URL.RawQuery = url.Values{
		"date":   {date.Value},
		"apikey": {c.apiKey},
	}.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.Error("earnings calendar request failed",
			"date", date.Value,
			"error", err.Error(),
			"baseURL", c.baseURL,
		)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		logging.Error("earnings calendar API error",
			"date", date.Value,
			"statusCode", resp.StatusCode,
			"baseURL", c.baseURL,
		)
		return nil, apiErr
	}

	var result EarningsCalendarResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logging.Error("failed to decode earnings calendar response",
			"date", date.Value,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	logging.Info("earnings calendar fetched",
		"date", result.Date,
		"pre", len(result.Pre),
		"after", len(result.After),
		"notSupplied", len(result.NotSupplied),
	)

	return &result, nil
}

// EconomicCalendar fetches economic calendar data for a specific date.
// The date can be a specific date string (YYYY-MM-DD) or relative values like "today", "yesterday", "tomorrow".
// If params.USMajor is true, returns only U.S. major indicators.
func (c *Client) EconomicCalendar(ctx context.Context, params EconomicCalendarParams) (*EconomicCalendarResponse, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.closed {
		return nil, ErrClientClosed
	}

	logging.Info("fetching economic calendar",
		"date", params.Date.Value,
		"usMajor", params.USMajor,
		"baseURL", c.baseURL,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.buildURL("/v1/calendar/economic"), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	queryParams := url.Values{
		"date":   {params.Date.Value},
		"apikey": {c.apiKey},
	}
	if params.USMajor {
		queryParams.Set("usmajor", "true")
	}
	req.URL.RawQuery = queryParams.Encode()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.Error("economic calendar request failed",
			"date", params.Date.Value,
			"error", err.Error(),
			"baseURL", c.baseURL,
		)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		logging.Error("economic calendar API error",
			"date", params.Date.Value,
			"statusCode", resp.StatusCode,
			"baseURL", c.baseURL,
		)
		return nil, apiErr
	}

	var events []EconomicEntry
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		logging.Error("failed to decode economic calendar response",
			"date", params.Date.Value,
			"error", err.Error(),
		)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	result := &EconomicCalendarResponse{
		Events: events,
	}

	logging.Info("economic calendar fetched",
		"date", params.Date.Value,
		"events", len(events),
	)

	return result, nil
}

// buildURL constructs the full URL for an endpoint.
func (c *Client) buildURL(path string) string {
	return c.baseURL + path
}

// Ensure Client implements interface at compile time.
var _ Provider = (*Client)(nil)