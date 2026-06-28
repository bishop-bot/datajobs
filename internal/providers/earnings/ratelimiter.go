package earnings

import (
	"context"

	"github.com/bishop-bot/datajobs/internal/ratelimiter"
)

// RateLimitedProvider wraps an earnings.Provider with rate limiting.
type RateLimitedProvider struct {
	provider Provider
	limiter  *ratelimiter.TokenBucket
}

// NewRateLimitedProvider creates a new rate-limited earnings provider.
func NewRateLimitedProvider(provider Provider, requestsPerMin int) *RateLimitedProvider {
	return &RateLimitedProvider{
		provider: provider,
		limiter:  ratelimiter.NewTokenBucket(requestsPerMin),
	}
}

// EarningsCalendar fetches earnings calendar with rate limiting.
func (p *RateLimitedProvider) EarningsCalendar(ctx context.Context, date CalendarDate) (*EarningsCalendarResponse, error) {
	// Apply rate limiting (method handles its own locking)
	if err := p.limiter.Allow(ctx); err != nil {
		return nil, err
	}

	return p.provider.EarningsCalendar(ctx, date)
}

// EconomicCalendar fetches economic calendar with rate limiting.
func (p *RateLimitedProvider) EconomicCalendar(ctx context.Context, params EconomicCalendarParams) (*EconomicCalendarResponse, error) {
	// Apply rate limiting (method handles its own locking)
	if err := p.limiter.Allow(ctx); err != nil {
		return nil, err
	}

	return p.provider.EconomicCalendar(ctx, params)
}

// Close implements Provider.Close if the underlying provider supports it.
func (p *RateLimitedProvider) Close() error {
	if closer, ok := p.provider.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}

// Ping implements Provider.Ping by delegating to the underlying provider.
func (p *RateLimitedProvider) Ping(ctx context.Context) error {
	return p.provider.Ping(ctx)
}
