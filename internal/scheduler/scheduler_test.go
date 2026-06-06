package scheduler

import (
	"testing"
	"time"

	"github.com/bishop-bot/datajobs/internal/config"
)

func TestSanitizeRetryConfig(t *testing.T) {
	tests := []struct {
		name     string
		input    config.RetryConfig
		expected config.RetryConfig
	}{
		{
			name:  "zero values get defaults",
			input: config.RetryConfig{},
			expected: config.RetryConfig{
				MaxAttempts:  3,
				InitialDelay: 1000,
				MaxDelay:     60000,
				Multiplier:   2.0,
			},
		},
		{
			name: "negative values get defaults",
			input: config.RetryConfig{
				MaxAttempts:  -1,
				InitialDelay: -100,
				MaxDelay:     -500,
				Multiplier:   -1.0,
			},
			expected: config.RetryConfig{
				MaxAttempts:  3,
				InitialDelay: 1000,
				MaxDelay:     60000,
				Multiplier:   2.0,
			},
		},
		{
			name: "partial zero values get defaults for zeros",
			input: config.RetryConfig{
				MaxAttempts:  5,
				InitialDelay: 0,
				MaxDelay:     30000,
				Multiplier:   1.5,
			},
			expected: config.RetryConfig{
				MaxAttempts:  5,
				InitialDelay: 1000,
				MaxDelay:     30000,
				Multiplier:   1.5,
			},
		},
		{
			name: "all valid values preserved",
			input: config.RetryConfig{
				MaxAttempts:  5,
				InitialDelay: 2000,
				MaxDelay:     120000,
				Multiplier:   3.0,
			},
			expected: config.RetryConfig{
				MaxAttempts:  5,
				InitialDelay: 2000,
				MaxDelay:     120000,
				Multiplier:   3.0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeRetryConfig(tt.input)
			if result.MaxAttempts != tt.expected.MaxAttempts {
				t.Errorf("MaxAttempts: expected %d, got %d", tt.expected.MaxAttempts, result.MaxAttempts)
			}
			if result.InitialDelay != tt.expected.InitialDelay {
				t.Errorf("InitialDelay: expected %d, got %d", tt.expected.InitialDelay, result.InitialDelay)
			}
			if result.MaxDelay != tt.expected.MaxDelay {
				t.Errorf("MaxDelay: expected %d, got %d", tt.expected.MaxDelay, result.MaxDelay)
			}
			if result.Multiplier != tt.expected.Multiplier {
				t.Errorf("Multiplier: expected %f, got %f", tt.expected.Multiplier, result.Multiplier)
			}
		})
	}
}

func TestSanitizeRetryConfigBackoffCalculation(t *testing.T) {
	// Test that sanitized config produces non-zero backoffs
	cfg := sanitizeRetryConfig(config.RetryConfig{})

	// Simulate backoff calculation (same formula as worker pool)
	calcBackoff := func(retry config.RetryConfig, attempt int) time.Duration {
		delay := float64(retry.InitialDelay)
		for i := 0; i < attempt; i++ {
			delay *= retry.Multiplier
		}
		if delay > float64(retry.MaxDelay) {
			delay = float64(retry.MaxDelay)
		}
		return time.Duration(delay) * time.Millisecond
	}

	// First attempt should have non-zero backoff
	backoff := calcBackoff(cfg, 0)
	if backoff <= 0 {
		t.Errorf("expected positive backoff for attempt 0, got %v", backoff)
	}

	// Subsequent attempts should also have non-zero backoff
	for attempt := 1; attempt < 5; attempt++ {
		backoff := calcBackoff(cfg, attempt)
		if backoff <= 0 {
			t.Errorf("expected positive backoff for attempt %d, got %v", attempt, backoff)
		}
	}
}
