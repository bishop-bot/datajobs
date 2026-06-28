package fmp

import (
	"errors"
	"fmt"
)

// Errors specific to the FMP API provider.
var (
	// ErrClientClosed is returned when an operation is attempted on a closed client.
	ErrClientClosed = &ClientClosedError{}

	// ErrMissingAPIKey is returned when the API key is not configured.
	ErrMissingAPIKey = &MissingAPIKeyError{}

	// ErrInvalidSymbol is returned when the symbol format is invalid.
	ErrInvalidSymbol = &InvalidSymbolError{}

	// ErrInvalidPeriod is returned when the period is invalid.
	ErrInvalidPeriod = &InvalidPeriodError{}
)

// ClientClosedError is returned when an operation is attempted on a closed client.
type ClientClosedError struct{}

func (e *ClientClosedError) Error() string {
	return "FMP client is closed"
}

// MissingAPIKeyError is returned when the API key is not configured.
type MissingAPIKeyError struct{}

func (e *MissingAPIKeyError) Error() string {
	return "FMP_API_KEY environment variable is not set"
}

// InvalidSymbolError is returned when the symbol format is invalid.
type InvalidSymbolError struct {
	Symbol string
}

func (e *InvalidSymbolError) Error() string {
	return "invalid symbol: " + e.Symbol
}

// InvalidPeriodError is returned when the period is invalid.
type InvalidPeriodError struct {
	Period string
}

func (e *InvalidPeriodError) Error() string {
	return "invalid period: " + e.Period + " (expected: annual, quarter, or ttm)"
}

// APIError represents an error response from the API.
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return "FMP API error: " + e.Message
	}
	return "FMP API error: status " + fmt.Sprintf("%d", e.StatusCode)
}

// IsAPIError checks if the error is an API error with a specific status code.
func IsAPIError(err error, statusCode int) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == statusCode
	}
	return false
}
