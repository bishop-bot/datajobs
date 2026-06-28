// Package ratelimiter provides a generic token bucket rate limiter for API requests.
package ratelimiter

import (
	"context"
	"sync"
	"time"
)

// TokenBucket implements a token bucket rate limiter.
type TokenBucket struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

// NewTokenBucket creates a new token bucket with the specified requests per minute.
func NewTokenBucket(requestsPerMin int) *TokenBucket {
	if requestsPerMin <= 0 {
		requestsPerMin = 30 // default
	}
	return &TokenBucket{
		tokens:     float64(requestsPerMin),
		maxTokens:  float64(requestsPerMin),
		refillRate: float64(requestsPerMin) / 60.0, // tokens per second
		lastRefill: time.Now(),
	}
}

// Allow checks if a request is allowed under the rate limit.
// It blocks until a token is available.
func (r *TokenBucket) Allow(ctx context.Context) error {
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

// AllowImmediate checks if a request can be made immediately without blocking.
// Returns true if a token is available, false otherwise.
func (r *TokenBucket) AllowImmediate() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefill = now

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// Reset resets the token bucket to full capacity.
func (r *TokenBucket) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens = r.maxTokens
	r.lastRefill = time.Now()
}

// Tokens returns the current number of available tokens.
func (r *TokenBucket) Tokens() float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	tokens := r.tokens + elapsed*r.refillRate
	if tokens > r.maxTokens {
		tokens = r.maxTokens
	}
	return tokens
}
