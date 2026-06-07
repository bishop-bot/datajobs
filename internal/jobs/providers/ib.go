package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/logging"
	ibproviders "github.com/bishop-bot/datajobs/internal/providers/ib"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// PingHandler creates a handler that pings the IB Gateway.
// It checks authentication status and re-authenticates if needed.
// The ibProvider parameter accepts either *ib.Client, *ib.MockClient, or any Provider implementation.
func PingHandler(ibProvider ibproviders.Provider) worker.JobFunc {
	return func(ctx context.Context, job worker.Job) (string, error) {
		logger := logging.FromContext(ctx).With("job_id", job.ID)

		if ibProvider == nil {
			return "", fmt.Errorf("IB provider not available")
		}

		// Check if provider supports authentication
		if authProvider, ok := ibProvider.(ibproviders.AuthAwareProvider); ok {
			// Check and ensure authentication
			if err := authProvider.EnsureAuthenticated(ctx); err != nil {
				logger.Error("IB authentication failed", "error", err)
				return "", fmt.Errorf("authentication failed: %w", err)
			}
			logger.Debug("IB authentication verified")
		}

		logger.Debug("pinging IB gateway")

		if err := ibProvider.Ping(ctx); err != nil {
			logger.Error("IB ping failed", "error", err)
			return "", fmt.Errorf("ping failed: %w", err)
		}

		return fmt.Sprintf("IB gateway ping successful at %s", time.Now().Format(time.RFC3339)), nil
	}
}