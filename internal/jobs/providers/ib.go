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