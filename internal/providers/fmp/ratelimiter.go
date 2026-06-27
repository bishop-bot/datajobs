package fmp

import (
	"context"

	"github.com/bishop-bot/datajobs/internal/ratelimiter"
)

// RateLimitedProvider wraps an fmp.Provider with rate limiting.
type RateLimitedProvider struct {
	provider Provider
	limiter  *ratelimiter.TokenBucket
}

// NewRateLimitedProvider creates a new rate-limited FMP provider.
func NewRateLimitedProvider(provider Provider, requestsPerMin int) *RateLimitedProvider {
	return &RateLimitedProvider{
		provider: provider,
		limiter:  ratelimiter.NewTokenBucket(requestsPerMin),
	}
}

// FinancialRatios fetches financial ratios with rate limiting.
func (p *RateLimitedProvider) FinancialRatios(ctx context.Context, symbol string, period string) (*FinancialRatiosResponse, error) {
	if err := p.limiter.Allow(ctx); err != nil {
		return nil, err
	}
	return p.provider.FinancialRatios(ctx, symbol, period)
}

// KeyMetrics fetches key metrics with rate limiting.
func (p *RateLimitedProvider) KeyMetrics(ctx context.Context, symbol string, period string) (*KeyMetricsResponse, error) {
	if err := p.limiter.Allow(ctx); err != nil {
		return nil, err
	}
	return p.provider.KeyMetrics(ctx, symbol, period)
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
