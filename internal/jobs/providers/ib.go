package providers

import (
	"context"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/providers"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// PingHandler pings the Interactive Brokers Client Portal Gateway.
func PingHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	ibClient := providers.GetIB()
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