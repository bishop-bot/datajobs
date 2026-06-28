package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/bishop-bot/datajobs/internal/database"
)

// Watchlist represents a user watchlist.
type Watchlist struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Owner       string    `json:"owner"`
	IsPublic    bool      `json:"is_public"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WatchlistSymbol represents a symbol in a watchlist.
type WatchlistSymbol struct {
	WatchlistID string    `json:"watchlist_id"`
	Symbol      string    `json:"symbol"`
	Note        string    `json:"note,omitempty"`
	Position    int       `json:"position"`
	AddedAt     time.Time `json:"added_at"`
}

// WatchlistWithSymbols includes the watchlist and its symbols.
type WatchlistWithSymbols struct {
	Watchlist
	Symbols []WatchlistSymbol `json:"symbols"`
}

// CreateWatchlistInput is the input for creating a watchlist.
type CreateWatchlistInput struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Owner       string   `json:"owner"`
	IsPublic    bool     `json:"is_public"`
	Symbols     []string `json:"symbols,omitempty"`
}

// UpdateWatchlistInput is the input for updating a watchlist.
type UpdateWatchlistInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

// AddSymbolInput is the input for adding a symbol.
type AddSymbolInput struct {
	WatchlistID string `json:"watchlist_id"`
	Symbol      string `json:"symbol"`
	Note        string `json:"note,omitempty"`
	Position    int    `json:"position"`
}

// WatchlistRepository handles watchlist database operations.
type WatchlistRepository struct {
	db *database.DB
}

// NewWatchlistRepository creates a new watchlist repository.
func NewWatchlistRepository(db *database.DB) *WatchlistRepository {
	return &WatchlistRepository{db: db}
}

// Create creates a new watchlist with optional initial symbols.
func (r *WatchlistRepository) Create(ctx context.Context, input CreateWatchlistInput) (*Watchlist, error) {
	tx, err := r.db.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO watchlists (id, name, description, owner, is_public, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, input.ID, input.Name, input.Description, input.Owner, input.IsPublic, now, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert watchlist: %w", err)
	}

	for i, symbol := range input.Symbols {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO watchlist_symbols (watchlist_id, symbol, note, position, added_at)
			VALUES (?, ?, ?, ?, ?)
		`, input.ID, symbol, "", i, now)
		if err != nil {
			return nil, fmt.Errorf("failed to insert symbol %s: %w", symbol, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &Watchlist{
		ID:          input.ID,
		Name:        input.Name,
		Description: input.Description,
		Owner:       input.Owner,
		IsPublic:    input.IsPublic,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// GetByID retrieves a watchlist by ID.
func (r *WatchlistRepository) GetByID(ctx context.Context, id string) (*Watchlist, error) {
	var w Watchlist
	err := r.db.QueryRow(ctx, `
		SELECT id, name, description, owner, is_public, created_at, updated_at
		FROM watchlists WHERE id = ?
	`, id).Scan(&w.ID, &w.Name, &w.Description, &w.Owner, &w.IsPublic, &w.CreatedAt, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get watchlist: %w", err)
	}
	return &w, nil
}

// GetByIDWithSymbols retrieves a watchlist with its symbols.
func (r *WatchlistRepository) GetByIDWithSymbols(ctx context.Context, id string) (*WatchlistWithSymbols, error) {
	w, err := r.GetByID(ctx, id)
	if err != nil || w == nil {
		return nil, err
	}

	symbols, err := r.GetSymbols(ctx, id)
	if err != nil {
		return nil, err
	}

	return &WatchlistWithSymbols{Watchlist: *w, Symbols: symbols}, nil
}

// GetByOwner retrieves all watchlists for an owner.
func (r *WatchlistRepository) GetByOwner(ctx context.Context, owner string) ([]*Watchlist, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, description, owner, is_public, created_at, updated_at
		FROM watchlists WHERE owner = ? ORDER BY created_at DESC
	`, owner)
	if err != nil {
		return nil, fmt.Errorf("failed to query watchlists: %w", err)
	}
	defer rows.Close()

	var watchlists []*Watchlist
	for rows.Next() {
		var w Watchlist
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.Owner, &w.IsPublic, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan watchlist: %w", err)
		}
		watchlists = append(watchlists, &w)
	}
	return watchlists, rows.Err()
}

// GetPublic retrieves all public watchlists.
func (r *WatchlistRepository) GetPublic(ctx context.Context) ([]*Watchlist, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, name, description, owner, is_public, created_at, updated_at
		FROM watchlists WHERE is_public = true ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query watchlists: %w", err)
	}
	defer rows.Close()

	var watchlists []*Watchlist
	for rows.Next() {
		var w Watchlist
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.Owner, &w.IsPublic, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan watchlist: %w", err)
		}
		watchlists = append(watchlists, &w)
	}
	return watchlists, rows.Err()
}

// Update updates a watchlist.
func (r *WatchlistRepository) Update(ctx context.Context, id string, input UpdateWatchlistInput) (*Watchlist, error) {
	setClauses := []string{}
	args := []interface{}{}

	if input.Name != nil {
		setClauses = append(setClauses, "name = ?")
		args = append(args, *input.Name)
	}
	if input.Description != nil {
		setClauses = append(setClauses, "description = ?")
		args = append(args, *input.Description)
	}
	if input.IsPublic != nil {
		setClauses = append(setClauses, "is_public = ?")
		args = append(args, *input.IsPublic)
	}

	if len(setClauses) == 0 {
		return r.GetByID(ctx, id)
	}

	setClauses = append(setClauses, "updated_at = ?")
	args = append(args, time.Now().UTC())
	args = append(args, id)

	query := fmt.Sprintf("UPDATE watchlists SET %s WHERE id = ?", joinStrings(setClauses, ", "))
	if _, err := r.db.Exec(ctx, query, args...); err != nil {
		return nil, fmt.Errorf("failed to update watchlist: %w", err)
	}

	return r.GetByID(ctx, id)
}

// Delete deletes a watchlist and its symbols.
func (r *WatchlistRepository) Delete(ctx context.Context, id string) error {
	if _, err := r.db.Exec(ctx, "DELETE FROM watchlists WHERE id = ?", id); err != nil {
		return fmt.Errorf("failed to delete watchlist: %w", err)
	}
	return nil
}

// GetSymbols retrieves all symbols in a watchlist.
func (r *WatchlistRepository) GetSymbols(ctx context.Context, watchlistID string) ([]WatchlistSymbol, error) {
	rows, err := r.db.Query(ctx, `
		SELECT watchlist_id, symbol, note, position, added_at
		FROM watchlist_symbols WHERE watchlist_id = ? ORDER BY position ASC
	`, watchlistID)
	if err != nil {
		return nil, fmt.Errorf("failed to query symbols: %w", err)
	}
	defer rows.Close()

	var symbols []WatchlistSymbol
	for rows.Next() {
		var s WatchlistSymbol
		var note sql.NullString
		if err := rows.Scan(&s.WatchlistID, &s.Symbol, &note, &s.Position, &s.AddedAt); err != nil {
			return nil, fmt.Errorf("failed to scan symbol: %w", err)
		}
		if note.Valid {
			s.Note = note.String
		}
		symbols = append(symbols, s)
	}
	return symbols, rows.Err()
}

// AddSymbol adds a symbol to a watchlist.
func (r *WatchlistRepository) AddSymbol(ctx context.Context, input AddSymbolInput) error {
	if _, err := r.db.Exec(ctx, `
		INSERT INTO watchlist_symbols (watchlist_id, symbol, note, position, added_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(watchlist_id, symbol) DO UPDATE SET note = excluded.note, position = excluded.position
	`, input.WatchlistID, input.Symbol, input.Note, input.Position, time.Now().UTC()); err != nil {
		return fmt.Errorf("failed to add symbol: %w", err)
	}

	_, err := r.db.Exec(ctx, "UPDATE watchlists SET updated_at = ? WHERE id = ?", time.Now().UTC(), input.WatchlistID)
	return err
}

// RemoveSymbol removes a symbol from a watchlist.
func (r *WatchlistRepository) RemoveSymbol(ctx context.Context, watchlistID, symbol string) error {
	if _, err := r.db.Exec(ctx, "DELETE FROM watchlist_symbols WHERE watchlist_id = ? AND symbol = ?", watchlistID, symbol); err != nil {
		return fmt.Errorf("failed to remove symbol: %w", err)
	}

	_, err := r.db.Exec(ctx, "UPDATE watchlists SET updated_at = ? WHERE id = ?", time.Now().UTC(), watchlistID)
	return err
}

// GetWatchlistsBySymbol retrieves all watchlists containing a symbol.
func (r *WatchlistRepository) GetWatchlistsBySymbol(ctx context.Context, symbol string) ([]*Watchlist, error) {
	rows, err := r.db.Query(ctx, `
		SELECT w.id, w.name, w.description, w.owner, w.is_public, w.created_at, w.updated_at
		FROM watchlists w
		INNER JOIN watchlist_symbols ws ON w.id = ws.watchlist_id
		WHERE ws.symbol = ? AND w.is_public = true
		ORDER BY w.created_at DESC
	`, symbol)
	if err != nil {
		return nil, fmt.Errorf("failed to query watchlists: %w", err)
	}
	defer rows.Close()

	var watchlists []*Watchlist
	for rows.Next() {
		var w Watchlist
		if err := rows.Scan(&w.ID, &w.Name, &w.Description, &w.Owner, &w.IsPublic, &w.CreatedAt, &w.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan watchlist: %w", err)
		}
		watchlists = append(watchlists, &w)
	}
	return watchlists, rows.Err()
}

func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for _, s := range strs[1:] {
		result += sep + s
	}
	return result
}