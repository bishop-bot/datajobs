package earnings

import "context"

// Provider defines the interface for Earnings API data access.
// This allows for mocking in tests and dependency injection.
type Provider interface {
	// Ping checks connectivity to the Earnings API.
	Ping(ctx context.Context) error

	// EarningsCalendar fetches earnings calendar data for a specific date.
	EarningsCalendar(ctx context.Context, date CalendarDate) (*EarningsCalendarResponse, error)

	// EconomicCalendar fetches economic calendar data for a specific date.
	EconomicCalendar(ctx context.Context, params EconomicCalendarParams) (*EconomicCalendarResponse, error)

	// Close releases resources.
	Close() error
}

// Ensure Client implements Provider at compile time.
var _ Provider = (*Client)(nil)