package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bishop-bot/datajobs/internal/repository"
)

// WatchlistHandler handles watchlist API requests.
type WatchlistHandler struct {
	repo *repository.WatchlistRepository
}

// NewWatchlistHandler creates a new watchlist handler.
func NewWatchlistHandler(repo *repository.WatchlistRepository) *WatchlistHandler {
	return &WatchlistHandler{repo: repo}
}

// RegisterRoutes registers watchlist routes on the router.
func (h *WatchlistHandler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.ListWatchlists)
	r.Post("/", h.CreateWatchlist)
	r.Get("/{id}", h.GetWatchlist)
	r.Put("/{id}", h.UpdateWatchlist)
	r.Delete("/{id}", h.DeleteWatchlist)

	// Symbol operations
	r.Get("/{id}/symbols", h.GetSymbols)
	r.Post("/{id}/symbols", h.AddSymbol)
	r.Delete("/{id}/symbols/{symbol}", h.RemoveSymbol)

	// Lookup
	r.Get("/symbol/{symbol}", h.GetWatchlistsBySymbol)
}

// ListWatchlists returns all public watchlists or user's own watchlists.
func (h *WatchlistHandler) ListWatchlists(w http.ResponseWriter, r *http.Request) {
	owner := r.URL.Query().Get("owner")

	var watchlists []*repository.Watchlist
	var err error

	if owner != "" {
		watchlists, err = h.repo.GetByOwner(r.Context(), owner)
	} else {
		watchlists, err = h.repo.GetPublic(r.Context())
	}

	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to fetch watchlists: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"watchlists": watchlists,
			"count":      len(watchlists),
		},
	})
}

// CreateWatchlist creates a new watchlist.
func (h *WatchlistHandler) CreateWatchlist(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Name        string   `json:"name"`
		Description string   `json:"description"`
		Owner       string   `json:"owner"`
		IsPublic    bool     `json:"is_public"`
		Symbols     []string `json:"symbols"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
		return
	}

	if input.Name == "" || input.Owner == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "name and owner are required",
		})
		return
	}

	watchlist, err := h.repo.Create(r.Context(), repository.CreateWatchlistInput{
		ID:          uuid.New().String(),
		Name:        input.Name,
		Description: input.Description,
		Owner:       input.Owner,
		IsPublic:    input.IsPublic,
		Symbols:     input.Symbols,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to create watchlist: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Data:    watchlist,
	})
}

// GetWatchlist returns a watchlist with its symbols.
func (h *WatchlistHandler) GetWatchlist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// Check if full details requested
	includeSymbols := r.URL.Query().Get("include") == "symbols"

	if includeSymbols {
		watchlist, err := h.repo.GetByIDWithSymbols(r.Context(), id)
		if err != nil {
			respondJSON(w, http.StatusInternalServerError, Response{
				Success: false,
				Error:   "failed to fetch watchlist: " + err.Error(),
			})
			return
		}
		if watchlist == nil {
			respondJSON(w, http.StatusNotFound, Response{
				Success: false,
				Error:   "watchlist not found",
			})
			return
		}
		respondJSON(w, http.StatusOK, Response{
			Success: true,
			Data:    watchlist,
		})
		return
	}

	watchlist, err := h.repo.GetByID(r.Context(), id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to fetch watchlist: " + err.Error(),
		})
		return
	}
	if watchlist == nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "watchlist not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    watchlist,
	})
}

// UpdateWatchlist updates a watchlist.
func (h *WatchlistHandler) UpdateWatchlist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input struct {
		Name        *string `json:"name,omitempty"`
		Description *string `json:"description,omitempty"`
		IsPublic    *bool   `json:"is_public,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
		return
	}

	watchlist, err := h.repo.Update(r.Context(), id, repository.UpdateWatchlistInput{
		Name:        input.Name,
		Description: input.Description,
		IsPublic:    input.IsPublic,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to update watchlist: " + err.Error(),
		})
		return
	}
	if watchlist == nil {
		respondJSON(w, http.StatusNotFound, Response{
			Success: false,
			Error:   "watchlist not found",
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data:    watchlist,
	})
}

// DeleteWatchlist deletes a watchlist.
func (h *WatchlistHandler) DeleteWatchlist(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	err := h.repo.Delete(r.Context(), id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to delete watchlist: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "watchlist deleted",
	})
}

// GetSymbols returns all symbols in a watchlist.
func (h *WatchlistHandler) GetSymbols(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	symbols, err := h.repo.GetSymbols(r.Context(), id)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to fetch symbols: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"symbols": symbols,
			"count":   len(symbols),
		},
	})
}

// AddSymbol adds a symbol to a watchlist.
func (h *WatchlistHandler) AddSymbol(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var input struct {
		Symbol   string `json:"symbol"`
		Note     string `json:"note"`
		Position int    `json:"position"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "invalid request body: " + err.Error(),
		})
		return
	}

	if input.Symbol == "" {
		respondJSON(w, http.StatusBadRequest, Response{
			Success: false,
			Error:   "symbol is required",
		})
		return
	}

	err := h.repo.AddSymbol(r.Context(), repository.AddSymbolInput{
		WatchlistID: id,
		Symbol:      input.Symbol,
		Note:        input.Note,
		Position:    input.Position,
	})
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to add symbol: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusCreated, Response{
		Success: true,
		Message: "symbol added",
	})
}

// RemoveSymbol removes a symbol from a watchlist.
func (h *WatchlistHandler) RemoveSymbol(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	symbol := chi.URLParam(r, "symbol")

	err := h.repo.RemoveSymbol(r.Context(), id, symbol)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to remove symbol: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Message: "symbol removed",
	})
}

// GetWatchlistsBySymbol returns all watchlists containing a symbol.
func (h *WatchlistHandler) GetWatchlistsBySymbol(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")

	watchlists, err := h.repo.GetWatchlistsBySymbol(r.Context(), symbol)
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, Response{
			Success: false,
			Error:   "failed to fetch watchlists: " + err.Error(),
		})
		return
	}

	respondJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"symbol":     symbol,
			"watchlists": watchlists,
			"count":      len(watchlists),
		},
	})
}
