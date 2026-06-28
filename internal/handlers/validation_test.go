package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestCreateWatchlistRequest_Validation(t *testing.T) {
	tests := []struct {
		name       string
		request    CreateWatchlistRequest
		wantErrors []string
	}{
		{
			name: "valid request",
			request: CreateWatchlistRequest{
				Name:        "My Watchlist",
				Description: "A great watchlist",
				Owner:       "user123",
				IsPublic:    true,
				Symbols:     []string{"AAPL", "GOOGL"},
			},
			wantErrors: nil,
		},
		{
			name: "valid request with ID",
			request: CreateWatchlistRequest{
				ID:          "myCustomId123",
				Name:        "My Watchlist",
				Owner:       "user123",
			},
			wantErrors: nil,
		},
		{
			name: "id too long",
			request: CreateWatchlistRequest{
				ID:   strings.Repeat("a", 101), // max is 100
				Name: "My Watchlist",
			},
			wantErrors: []string{"id must be at most 100 characters"},
		},
		{
			name: "missing required name",
			request: CreateWatchlistRequest{
				Description: "No name",
				Owner:       "user123",
			},
			wantErrors: []string{"name is required"},
		},
		{
			name: "name too long",
			request: CreateWatchlistRequest{
				Name:  strings.Repeat("a", 101), // max is 100
				Owner: "user123",
			},
			wantErrors: []string{"name must be at most 100 characters"},
		},
		{
			name: "description too long",
			request: CreateWatchlistRequest{
				Name:        "Valid Name",
				Description: strings.Repeat("a", 501), // max is 500
				Owner:       "user123",
			},
			wantErrors: []string{"description must be at most 500 characters"},
		},
		{
			name: "symbol too long in slice",
			request: CreateWatchlistRequest{
				Name:    "My Watchlist",
				Owner:   "user123",
				Symbols: []string{strings.Repeat("a", 21)}, // max is 20
			},
			wantErrors: []string{"symbols[0] must be at most 20 characters"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.request)
			if tt.wantErrors == nil {
				if err != nil {
					t.Errorf("expected no errors, got: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected errors %v, got nil", tt.wantErrors)
				return
			}

			validationErrs := ValidationErrors(err)
			for _, want := range tt.wantErrors {
				found := false
				for _, got := range validationErrs {
					if strings.Contains(got, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", want, validationErrs)
				}
			}
		})
	}
}

func TestUpdateWatchlistRequest_Validation(t *testing.T) {
	emptyStr := ""
	longStr := strings.Repeat("a", 101)

	tests := []struct {
		name       string
		request    UpdateWatchlistRequest
		wantErrors []string
	}{
		{
			name: "valid partial update",
			request: UpdateWatchlistRequest{
				Name: &emptyStr, // empty string fails min=1
			},
			wantErrors: []string{"name must be at least 1 characters"},
		},
		{
			name: "name too long",
			request: UpdateWatchlistRequest{
				Name: &longStr,
			},
			wantErrors: []string{"name must be at most 100 characters"},
		},
		{
			name: "valid with nil fields",
			request: UpdateWatchlistRequest{
				Name: nil,
			},
			wantErrors: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.request)
			if tt.wantErrors == nil {
				if err != nil {
					t.Errorf("expected no errors, got: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected errors %v, got nil", tt.wantErrors)
				return
			}

			validationErrs := ValidationErrors(err)
			for _, want := range tt.wantErrors {
				found := false
				for _, got := range validationErrs {
					if strings.Contains(got, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", want, validationErrs)
				}
			}
		})
	}
}

func TestAddSymbolRequest_Validation(t *testing.T) {
	negativePos := -1

	tests := []struct {
		name       string
		request    AddSymbolRequest
		wantErrors []string
	}{
		{
			name: "valid request",
			request: AddSymbolRequest{
				Symbol:   "AAPL",
				Note:     "Apple Inc",
				Position: 0,
			},
			wantErrors: nil,
		},
		{
			name: "missing symbol",
			request: AddSymbolRequest{
				Note: "Missing symbol",
			},
			wantErrors: []string{"symbol is required"},
		},
		{
			name: "symbol too long",
			request: AddSymbolRequest{
				Symbol: strings.Repeat("A", 21), // max is 20
			},
			wantErrors: []string{"symbol must be at most 20 characters"},
		},
		{
			name: "negative position",
			request: AddSymbolRequest{
				Symbol:   "AAPL",
				Position: negativePos,
			},
			wantErrors: []string{"position must be greater than or equal to 0"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.request)
			if tt.wantErrors == nil {
				if err != nil {
					t.Errorf("expected no errors, got: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected errors %v, got nil", tt.wantErrors)
				return
			}

			validationErrs := ValidationErrors(err)
			for _, want := range tt.wantErrors {
				found := false
				for _, got := range validationErrs {
					if strings.Contains(got, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", want, validationErrs)
				}
			}
		})
	}
}

func TestCreateJobRequest_Validation(t *testing.T) {
	tests := []struct {
		name       string
		request    CreateJobRequest
		wantErrors []string
	}{
		{
			name: "valid request",
			request: CreateJobRequest{
				ID:       "my-job",
				Name:     "My Job",
				Cron:     "0 0 * * * *",
				Type:     "scheduled",
				Handler:  "my-handler",
				Enabled:  true,
				Timeout:  3600,
				Retry:    RetryConfig{MaxAttempts: 3, DelayMs: 1000},
			},
			wantErrors: nil,
		},
		{
			name: "missing required fields",
			request:    CreateJobRequest{},
			wantErrors: []string{"id is required", "cron is required", "type is required", "handler is required"},
		},
		{
			name: "invalid type",
			request: CreateJobRequest{
				ID:      "my-job",
				Cron:    "0 0 * * * *",
				Type:    "invalid",
				Handler: "my-handler",
			},
			wantErrors: []string{"type must be one of: scheduled event batch"},
		},
		{
			name: "invalid cron",
			request: CreateJobRequest{
				ID:      "my-job",
				Cron:    "not-a-cron",
				Type:    "scheduled",
				Handler: "my-handler",
			},
			wantErrors: []string{"cron must be a valid cron expression"},
		},
		{
			name: "timeout out of range",
			request: CreateJobRequest{
				ID:      "my-job",
				Cron:    "0 0 * * * *",
				Type:    "scheduled",
				Handler: "my-handler",
				Timeout: 100000, // max is 86400
			},
			wantErrors: []string{"timeout must be less than or equal to 86400"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validate.Struct(tt.request)
			if tt.wantErrors == nil {
				if err != nil {
					t.Errorf("expected no errors, got: %v", err)
				}
				return
			}

			if err == nil {
				t.Errorf("expected errors %v, got nil", tt.wantErrors)
				return
			}

			validationErrs := ValidationErrors(err)
			for _, want := range tt.wantErrors {
				found := false
				for _, got := range validationErrs {
					if strings.Contains(got, want) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error containing %q, got: %v", want, validationErrs)
				}
			}
		})
	}
}

func TestValidationErrors_Integration(t *testing.T) {
	// Test that validation errors are properly formatted in HTTP responses
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req CreateWatchlistRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondJSON(w, http.StatusBadRequest, Response{
				Success: false,
				Error:   "invalid request body",
			})
			return
		}

		if err := validate.Struct(req); err != nil {
			respondValidationError(w, err)
			return
		}

		respondJSON(w, http.StatusOK, Response{Success: true})
	})

	// Test with invalid request (missing required fields)
	t.Run("missing fields returns 400", func(t *testing.T) {
		body := `{"description": "only description"}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400, got %d", w.Code)
		}

		var resp Response
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to unmarshal response: %v", err)
		}

		if resp.Success {
			t.Error("expected success=false")
		}

		if !strings.Contains(resp.Error, "validation failed") {
			t.Errorf("expected error to contain 'validation failed', got: %s", resp.Error)
		}

		if !strings.Contains(resp.Error, "name is required") {
			t.Errorf("expected error to contain 'name is required', got: %s", resp.Error)
		}
	})

	// Test with valid request
	t.Run("valid request returns 200", func(t *testing.T) {
		body := `{"name": "Test", "owner": "user1"}`
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		w := httptest.NewRecorder()

		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})
}

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Watchlist", "myWatchlist"},
		{"simple", "simple"},
		{"multiple words here", "multipleWordsHere"},
		{"ALLCAPS", "allcaps"},
		{"with123numbers", "with123numbers"},
		{"spaces   and    tabs", "spacesAndTabs"},
		{"", ""},
		{"a", "a"},
		{"A", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toCamelCase(tt.input)
			if result != tt.expected {
				t.Errorf("toCamelCase(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
