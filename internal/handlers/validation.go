package handlers

import (
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator is a global validator instance.
var validate *validator.Validate

func init() {
	validate = validator.New()

	// Register a custom tag name function to use JSON field names in error messages
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})
}

// ValidationErrors converts validator.ValidationErrors to user-friendly messages.
func ValidationErrors(err error) []string {
	var errors []string
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			errors = append(errors, formatValidationError(e))
		}
	}
	return errors
}

// formatValidationError formats a single validation error into a human-readable message.
func formatValidationError(e validator.FieldError) string {
	field := e.Field()
	switch e.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "min":
		if e.Type().Kind() == reflect.Slice {
			return fmt.Sprintf("%s must contain at least %s items", field, e.Param())
		}
		return fmt.Sprintf("%s must be at least %s characters", field, e.Param())
	case "max":
		if e.Type().Kind() == reflect.Slice {
			return fmt.Sprintf("%s must contain at most %s items", field, e.Param())
		}
		return fmt.Sprintf("%s must be at most %s characters", field, e.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, e.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, e.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, e.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, e.Param())
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, e.Param())
	case "cron":
		return fmt.Sprintf("%s must be a valid cron expression", field)
	default:
		return fmt.Sprintf("%s failed validation: %s", field, e.Tag())
	}
}

// respondValidationError sends a 400 Bad Request with validation error details.
func respondValidationError(w http.ResponseWriter, err error) {
	errors := ValidationErrors(err)
	respondJSON(w, http.StatusBadRequest, Response{
		Success: false,
		Error:   "validation failed: " + strings.Join(errors, "; "),
	})
}

// ---- Request Structs with Validation Tags ----

// CreateWatchlistRequest is the request body for creating a watchlist.
type CreateWatchlistRequest struct {
	ID          string   `json:"id"           validate:"omitempty,min=1,max=100"` // optional, camelCase of name if not provided
	Name        string   `json:"name"         validate:"required,min=1,max=100"`
	Description string   `json:"description"  validate:"max=500"`
	Owner       string   `json:"owner"` // optional
	IsPublic    bool     `json:"is_public"`
	Symbols     []string `json:"symbols"      validate:"max=500,dive,required,min=1,max=20"`
}

// UpdateWatchlistRequest is the request body for updating a watchlist.
type UpdateWatchlistRequest struct {
	Name        *string `json:"name"         validate:"omitempty,min=1,max=100"`
	Description *string `json:"description"  validate:"omitempty,max=500"`
	IsPublic    *bool   `json:"is_public"`
}

// AddSymbolRequest is the request body for adding a symbol to a watchlist.
type AddSymbolRequest struct {
	Symbol   string `json:"symbol"   validate:"required,min=1,max=20"`
	Note     string `json:"note"     validate:"omitempty,max=500"`
	Position *int   `json:"position" validate:"omitempty,gte=0"` // optional, nil means auto-assign at end
}

// CreateJobRequest is the request body for creating a job.
type CreateJobRequest struct {
	ID       string                 `json:"id"        validate:"required,min=1,max=100"`
	Name     string                 `json:"name"      validate:"max=200"`
	Cron     string                 `json:"cron"      validate:"required,cron"`
	Type     string                 `json:"type"      validate:"required,oneof=scheduled event batch"`
	Handler  string                 `json:"handler"   validate:"required,max=100"`
	Enabled  bool                   `json:"enabled"`
	Timeout  int                    `json:"timeout"   validate:"gte=0,lte=86400"`
	Retry    RetryConfig            `json:"retry"`
	Metadata map[string]interface{} `json:"metadata"`
}

// RetryConfig is the retry configuration for a job.
type RetryConfig struct {
	MaxAttempts int `json:"max_attempts" validate:"gte=0,lte=100"`
	DelayMs     int `json:"delay_ms"     validate:"gte=0,lte=60000"`
}

// UpdateJobRequest is the request body for updating a job.
type UpdateJobRequest struct {
	Name     *string               `json:"name,omitempty"     validate:"omitempty,max=200"`
	Cron     *string               `json:"cron,omitempty"     validate:"omitempty,cron"`
	Handler  *string               `json:"handler,omitempty"  validate:"omitempty,max=100"`
	Enabled  *bool                 `json:"enabled,omitempty"`
	Timeout  *int                  `json:"timeout,omitempty"  validate:"omitempty,gte=0,lte=86400"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}
