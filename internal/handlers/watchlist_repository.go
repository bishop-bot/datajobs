package handlers

import (
	"github.com/bishop-bot/datajobs/internal/database"
	"github.com/bishop-bot/datajobs/internal/repository"
)

// NewWatchlistRepository creates a new watchlist repository.
func NewWatchlistRepository(db *database.DB) *repository.WatchlistRepository {
	return repository.NewWatchlistRepository(db)
}
