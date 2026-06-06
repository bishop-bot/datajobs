package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/logging"
	ibproviders "github.com/bishop-bot/datajobs/internal/providers"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// PingHandler creates a handler that pings the IB Gateway.
// The ibProvider parameter accepts either *IBClient, *MockIBClient, or any IBProvider implementation.
func PingHandler(ibProvider ibproviders.IBProvider) worker.JobFunc {
	return func(ctx context.Context, job worker.Job) (string, error) {
		logger := logging.FromContext(ctx).With("job_id", job.ID)

		if ibProvider == nil {
			return "", fmt.Errorf("IB provider not available")
		}

		logger.Debug("pinging IB gateway")

		if err := ibProvider.Ping(ctx); err != nil {
			logger.Error("IB ping failed", "error", err)
			return "", fmt.Errorf("ping failed: %w", err)
		}

		return fmt.Sprintf("IB gateway ping successful at %s", time.Now().Format(time.RFC3339)), nil
	}
}

// PingHandlerGlobal pings the IB Gateway using the global singleton.
// Deprecated: Use PingHandler with explicit IBProvider instead.
// This function exists for backward compatibility with existing code.
func PingHandlerGlobal(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	ibClient := ibproviders.GetIB()
	if ibClient == nil {
		return "", fmt.Errorf("IB client not initialized")
	}

	logger.Debug("pinging IB gateway")

	if err := ibClient.Ping(ctx); err != nil {
		logger.Error("IB ping failed", "error", err)
		return "", fmt.Errorf("ping failed: %w", err)
	}

	return fmt.Sprintf("IB gateway ping successful at %s", time.Now().Format(time.RFC3339)), nil
}