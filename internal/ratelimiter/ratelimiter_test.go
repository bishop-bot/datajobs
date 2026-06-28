package ratelimiter

import (
	"context"
	"testing"
	"time"
)

func TestNewTokenBucket(t *testing.T) {
	bucket := NewTokenBucket(60)
	if bucket.maxTokens != 60 {
		t.Errorf("maxTokens = %v, want 60", bucket.maxTokens)
	}
	if bucket.refillRate != 1.0 {
		t.Errorf("refillRate = %v, want 1.0", bucket.refillRate)
	}
}

func TestNewTokenBucketDefault(t *testing.T) {
	bucket := NewTokenBucket(0)
	if bucket.maxTokens != 30 {
		t.Errorf("maxTokens = %v, want 30 (default)", bucket.maxTokens)
	}
}

func TestNewTokenBucketNegative(t *testing.T) {
	bucket := NewTokenBucket(-5)
	if bucket.maxTokens != 30 {
		t.Errorf("maxTokens = %v, want 30 (default)", bucket.maxTokens)
	}
}

func TestTokenBucketAllow(t *testing.T) {
	bucket := NewTokenBucket(60) // 1 per second

	ctx := context.Background()

	// First request should succeed immediately
	err := bucket.Allow(ctx)
	if err != nil {
		t.Errorf("Allow() failed: %v", err)
	}

	// Second request might need to wait (depends on timing)
	err = bucket.Allow(ctx)
	if err != nil {
		t.Errorf("Allow() failed: %v", err)
	}
}

func TestTokenBucketAllowWithContextCancellation(t *testing.T) {
	bucket := NewTokenBucket(1) // Very slow rate
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Consume the token
	bucket.Allow(context.Background())

	// Now try to get another - should be cancelled by context
	err := bucket.Allow(ctx)
	if err != context.DeadlineExceeded && err != context.Canceled {
		t.Errorf("Allow() returned unexpected error: %v", err)
	}
}

func TestTokenBucketAllowImmediate(t *testing.T) {
	bucket := NewTokenBucket(60)

	// Should be able to get a token immediately
	if !bucket.AllowImmediate() {
		t.Error("AllowImmediate() returned false, want true")
	}

	// Should still have tokens
	if bucket.Tokens() <= 0 {
		t.Error("Expected tokens after AllowImmediate")
	}
}

func TestTokenBucketReset(t *testing.T) {
	bucket := NewTokenBucket(60)

	// Consume some tokens
	bucket.Allow(context.Background())
	bucket.Allow(context.Background())

	// Reset
	bucket.Reset()

	if bucket.Tokens() != 60 {
		t.Errorf("Tokens() after reset = %v, want 60", bucket.Tokens())
	}
}

func TestTokenBucketRefill(t *testing.T) {
	bucket := NewTokenBucket(60) // 1 per second

	// Consume all tokens
	for i := 0; i < 60; i++ {
		bucket.Allow(context.Background())
	}

	// Should be empty or nearly empty
	tokens := bucket.Tokens()
	if tokens > 1 {
		t.Errorf("Expected ~0 tokens after exhausting, got %v", tokens)
	}

	// Wait a bit and check refill
	time.Sleep(100 * time.Millisecond)
	tokens = bucket.Tokens()
	if tokens < 0.05 || tokens > 0.15 {
		t.Errorf("Expected ~0.1 tokens after 100ms, got %v", tokens)
	}
}

func TestTokenBucketConcurrency(t *testing.T) {
	bucket := NewTokenBucket(100) // Fast rate for testing

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				bucket.Allow(context.Background())
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// All 100 requests should have been processed
	// (Some tokens may have been refilled, but no panics should occur)
}
