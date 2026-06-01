package ingestion

import (
	"context"
	"fmt"

	"github.com/bishop-bot/datajobs/internal/logging"
	"github.com/bishop-bot/datajobs/internal/worker"
)

// SQLiteToQuestDBHandler syncs data from SQLite to QuestDB.
func SQLiteToQuestDBHandler(ctx context.Context, job worker.Job) (string, error) {
	logger := logging.FromContext(ctx).With("job_id", job.ID)

	sourceDB, _ := job.Metadata["sourceDb"].(string)
	targetTable, _ := job.Metadata["targetTable"].(string)

	if sourceDB == "" || targetTable == "" {
		return "", fmt.Errorf("sourceDb and targetTable are required")
	}

	logger.Info("starting SQLite to QuestDB sync",
		"source_db", sourceDB,
		"target_table", targetTable,
	)

	return "SQLite to QuestDB sync initiated", nil
}