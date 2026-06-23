package earnings

import (
	"errors"
	"fmt"
)

// Errors specific to the Earnings API provider.
var (
	// ErrClientClosed is returned when an operation is attempted on a closed client.
	ErrClientClosed = &ClientClosedError{}

	// ErrMissingAPIKey is returned when the API key is not configured.
	ErrMissingAPIKey = &MissingAPIKeyError{}

	// ErrInvalidDate is returned when the date format is invalid.
	ErrInvalidDate = &InvalidDateError{}

	// ErrAPIError is returned when the API returns an error response.
	ErrAPIError = &APIError{}
)

// ClientClosedError is returned when an operation is attempted on a closed client.
type ClientClosedError struct{}

func (e *ClientClosedError) Error() string {
	return "earnings client is closed"
}

// MissingAPIKeyError is returned when the API key is not configured.
type MissingAPIKeyError struct{}

func (e *MissingAPIKeyError) Error() string {
	return "EARNINGS_API_KEY environment variable is not set"
}

// InvalidDateError is returned when the date format is invalid.
type InvalidDateError struct {
	Date string
}

func (e *InvalidDateError) Error() string {
	return "invalid date format: " + e.Date + " (expected YYYY-MM-DD or today/yesterday/tomorrow)"
}

// APIError represents an error response from the API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return "earnings API error: " + e.Message
	}
	return "earnings API error: status " + fmt.Sprintf("%d", e.StatusCode)
}

// IsAPIError checks if the error is an API error with a specific status code.
func IsAPIError(err error, statusCode int) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == statusCode
	}
	return false
}