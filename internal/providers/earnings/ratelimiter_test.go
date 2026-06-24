package earnings

import (
	"context"
	"testing"
	"time"
)

func TestRateLimiterAllow(t *testing.T) {
	// Create a rate limiter that allows 60 requests per minute (1 every second)
	// but starts with burst of 60 tokens
	limiter := NewRateLimiter(60)
	ctx := context.Background()

	// First request should go through immediately (tokens available)
	if err := limiter.Allow(ctx); err != nil {
		t.Errorf("request 1: unexpected error: %v", err)
	}

	// Deplete tokens by making 59 more requests
	for i := 0; i < 59; i++ {
		if err := limiter.Allow(ctx); err != nil {
			t.Errorf("request %d: unexpected error: %v", i+2, err)
		}
	}

	// 61st request should need to wait at least ~1 second
	start := time.Now()
	if err := limiter.Allow(ctx); err != nil {
		t.Errorf("request 61: unexpected error: %v", err)
	}
	elapsed := time.Since(start)
	
	// Should have waited at least 0.9 seconds (allow some margin)
	if elapsed < 900*time.Millisecond {
		t.Errorf("61st request: expected wait >= 0.9s, got %v", elapsed)
	}
}

func TestRateLimiterDefaultRate(t *testing.T) {
	// Test that invalid rates use default
	limiter := NewRateLimiter(0)
	if limiter.maxTokens != 30 {
		t.Errorf("expected maxTokens 30 for rate 0, got %v", limiter.maxTokens)
	}

	limiter = NewRateLimiter(-5)
	if limiter.maxTokens != 30 {
		t.Errorf("expected maxTokens 30 for rate -5, got %v", limiter.maxTokens)
	}
}

func TestRateLimitedProviderImplementsProvider(t *testing.T) {
	// Compile-time check that RateLimitedProvider implements Provider
	var _ Provider = (*RateLimitedProvider)(nil)
}

func TestRateLimitedProviderClose(t *testing.T) {
	// Test with a provider that has Close
	client := &mockProviderWithClose{closed: false}
	rlp := NewRateLimitedProvider(client, 30)
	
	err := rlp.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if !client.closed {
		t.Error("expected provider.Close() to be called")
	}
}

func TestRateLimitedProviderCloseNoOp(t *testing.T) {
	// Test with a provider that doesn't have Close
	client := &mockProviderWithoutClose{}
	rlp := NewRateLimitedProvider(client, 30)
	
	err := rlp.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRateLimitedProviderEconomicCalendar(t *testing.T) {
	// Test that EconomicCalendar applies rate limiting
	client := &mockProviderWithClose{}
	rlp := NewRateLimitedProvider(client, 60) // 60 req/min = 1 per second

	ctx := context.Background()
	params := EconomicCalendarParams{
		Date:   NewCalendarDate(time.Now()),
		USMajor: true,
	}

	// First call should go through immediately
	_, err := rlp.EconomicCalendar(ctx, params)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Deplete tokens
	for i := 0; i < 59; i++ {
		_, _ = rlp.EconomicCalendar(ctx, params)
	}

	// 61st call should be rate limited
	start := time.Now()
	_, err = rlp.EconomicCalendar(ctx, params)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("expected rate limit delay >= 0.9s, got %v", elapsed)
	}
}

// mockProviderWithClose is a mock provider that implements Close
type mockProviderWithClose struct {
	closed bool
}

func (m *mockProviderWithClose) Ping(ctx context.Context) error {
	return nil
}

func (m *mockProviderWithClose) EarningsCalendar(ctx context.Context, date CalendarDate) (*EarningsCalendarResponse, error) {
	return &EarningsCalendarResponse{}, nil
}

func (m *mockProviderWithClose) EconomicCalendar(ctx context.Context, params EconomicCalendarParams) (*EconomicCalendarResponse, error) {
	return &EconomicCalendarResponse{}, nil
}

func (m *mockProviderWithClose) Close() error {
	m.closed = true
	return nil
}

// mockProviderWithoutClose is a mock provider without explicit Close
type mockProviderWithoutClose struct{}

func (m *mockProviderWithoutClose) Ping(ctx context.Context) error {
	return nil
}

func (m *mockProviderWithoutClose) EarningsCalendar(ctx context.Context, date CalendarDate) (*EarningsCalendarResponse, error) {
	return &EarningsCalendarResponse{}, nil
}

func (m *mockProviderWithoutClose) EconomicCalendar(ctx context.Context, params EconomicCalendarParams) (*EconomicCalendarResponse, error) {
	return &EconomicCalendarResponse{}, nil
}

func (m *mockProviderWithoutClose) Close() error {
	return nil
}
