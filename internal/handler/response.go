package handler

import (
	"encoding/json"
	"net/http"
)

// APIResponse represents a standardized API response envelope.
type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

// PaginatedResponse represents a paginated API response.
type PaginatedResponse struct {
	Success bool      `json:"success"`
	Data    any       `json:"data"`
	Meta    *PageMeta `json:"meta,omitempty"`
}

// PageMeta contains pagination metadata.
type PageMeta struct {
	Total   int `json:"total"`
	Page    int `json:"page"`
	PerPage int `json:"per_page"`
}

// JSON writes a JSON response with the given status code.
// It follows the immutable pattern by not modifying any input data.
func JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// Log encoding error but don't try to write another response
		// as headers have already been sent
		return
	}
}

// Success writes a successful JSON response.
func Success(w http.ResponseWriter, status int, data any) {
	response := APIResponse{
		Success: true,
		Data:    data,
	}
	JSON(w, status, response)
}

// Error writes an error JSON response.
func Error(w http.ResponseWriter, status int, message string) {
	response := APIResponse{
		Success: false,
		Error:   message,
	}
	JSON(w, status, response)
}

// Paginated writes a paginated JSON response.
func Paginated(w http.ResponseWriter, status int, data any, meta *PageMeta) {
	response := PaginatedResponse{
		Success: true,
		Data:    data,
		Meta:    meta,
	}
	JSON(w, status, response)
}

// DecodeJSON decodes the request body into the given interface.
// Returns an error if decoding fails.
func DecodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return ErrEmptyBody
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// ValidationErrorResponse represents a validation error response with field-specific errors.
type ValidationErrorResponse struct {
	Success bool              `json:"success" example:"false"`
	Error   string            `json:"error" example:"validation failed"`
	Fields  map[string]string `json:"fields"`
}

// ValidationError writes a validation error response with field-specific errors.
func ValidationError(w http.ResponseWriter, errors map[string]string) {
	response := struct {
		Success bool              `json:"success"`
		Error   string            `json:"error"`
		Fields  map[string]string `json:"fields"`
	}{
		Success: false,
		Error:   "validation failed",
		Fields:  errors,
	}
	JSON(w, http.StatusBadRequest, response)
}
