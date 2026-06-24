package earnings

import "time"

// EarningsCalendarResponse represents the API response for the earnings calendar endpoint.
type EarningsCalendarResponse struct {
	// Date is the calendar date in YYYY-MM-DD format
	Date string `json:"date"`
	// Pre contains companies reporting earnings before market open
	Pre []EarningsEntry `json:"pre"`
	// After contains companies reporting earnings after market close
	After []EarningsEntry `json:"after"`
	// NotSupplied contains companies with unspecified reporting times
	NotSupplied []EarningsEntry `json:"notSupplied"`
}

// EarningsEntry represents a single company's earnings data.
type EarningsEntry struct {
	// Symbol is the ticker symbol
	Symbol string `json:"symbol"`
	// Name is the company name
	Name string `json:"name"`
	// EpsEstimate is the analyst EPS estimate
	EpsEstimate float64 `json:"epsEstimate"`
	// Eps is the actual/reported EPS
	Eps float64 `json:"eps"`
	// Revenue is the actual/reported revenue in cents
	Revenue int64 `json:"revenue"`
	// RevenueEstimate is the analyst revenue estimate in cents
	RevenueEstimate int64 `json:"revenueEstimate"`
}

// CalendarDate represents a date for calendar queries.
// Can be a specific date string (YYYY-MM-DD) or special values like "today", "yesterday", "tomorrow".
type CalendarDate struct {
	// Value is the date string or special value
	Value string
	// IsRelative indicates if this is a relative date (today, yesterday, tomorrow)
	IsRelative bool
}

// NewCalendarDate creates a CalendarDate from a time.Time.
func NewCalendarDate(t time.Time) CalendarDate {
	return CalendarDate{
		Value:       t.Format("2006-01-02"),
		IsRelative:  false,
	}
}

// NewRelativeCalendarDate creates a CalendarDate for a relative date.
func NewRelativeCalendarDate(value string) CalendarDate {
	return CalendarDate{
		Value:       value,
		IsRelative:  true,
	}
}

// Today returns a CalendarDate for today.
func Today() CalendarDate {
	return NewRelativeCalendarDate("today")
}

// Yesterday returns a CalendarDate for yesterday.
func Yesterday() CalendarDate {
	return NewRelativeCalendarDate("yesterday")
}

// Tomorrow returns a CalendarDate for tomorrow.
func Tomorrow() CalendarDate {
	return NewRelativeCalendarDate("tomorrow")
}

// DateRange represents a range of dates for batch queries.
type DateRange struct {
	Start time.Time
	End   time.Time
}

// EconomicCalendarResponse represents the API response for the economic calendar endpoint.
type EconomicCalendarResponse struct {
	// Events contains economic events for the queried date
	Events []EconomicEntry `json:"events"`
}

// EconomicEntry represents a single economic event.
type EconomicEntry struct {
	// Country is the country or region name (e.g., "United States")
	Country string `json:"country"`
	// EventName is the economic event name (e.g., "PPI")
	EventName string `json:"eventName"`
	// Date is the event date in YYYY-MM-DD format
	Date string `json:"date"`
	// Time is the event time in ET (e.g., "08:30") or "24H"
	Time string `json:"time"`
	// Actual is the actual value (null if not released yet)
	Actual *string `json:"actual"`
	// Consensus is the market consensus/forecast (may be null)
	Consensus *string `json:"consensus"`
	// Previous is the previous value (may be null)
	Previous *string `json:"previous"`
}

// EconomicCalendarParams contains parameters for economic calendar queries.
type EconomicCalendarParams struct {
	// Date is the date to query (YYYY-MM-DD or "today", "yesterday", "tomorrow")
	Date CalendarDate
	// USMajor if true, returns U.S. major indicators only
	USMajor bool
}