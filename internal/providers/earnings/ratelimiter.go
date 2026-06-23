package earnings

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter.
type RateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new rate limiter with the specified requests per minute.
func NewRateLimiter(requestsPerMin int) *RateLimiter {
	if requestsPerMin <= 0 {
		requestsPerMin = 30 // default
	}
	return &RateLimiter{
		tokens:     float64(requestsPerMin),
		maxTokens:  float64(requestsPerMin),
		refillRate: float64(requestsPerMin) / 60.0, // tokens per second
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit.
// It blocks until a token is available.
func (r *RateLimiter) Allow(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Refill tokens based on elapsed time
	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefill = now

	// If no tokens available, wait until we can get one
	if r.tokens < 1 {
		// Calculate wait time
		tokensNeeded := 1 - r.tokens
		waitTime := time.Duration(tokensNeeded / r.refillRate * float64(time.Second))
		
		// Wait with context cancellation support
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Refill after waiting
			r.tokens += 1
			r.lastRefill = time.Now()
		}
	}

	// Consume one token
	r.tokens--
	return nil
}

// RateLimitedProvider wraps an earnings.Provider with rate limiting.
type RateLimitedProvider struct {
	provider Provider
	limiter  *RateLimiter
}

// NewRateLimitedProvider creates a new rate-limited earnings provider.
func NewRateLimitedProvider(provider Provider, requestsPerMin int) *RateLimitedProvider {
	return &RateLimitedProvider{
		provider: provider,
		limiter:  NewRateLimiter(requestsPerMin),
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
