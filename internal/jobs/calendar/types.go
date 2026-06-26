package calendar

import "time"

// CalendarEconomic represents an economic calendar event entity.
type CalendarEconomic struct {
	ID         int64
	Country    string
	EventName  string
	Date       string
	Time       string
	Actual     *string
	Consensus  *string
	Previous   *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
