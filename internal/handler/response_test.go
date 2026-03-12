package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestJSON(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		data       any
		wantStatus int
		wantBody   string
	}{
		{
			name:       "simple map",
			status:     http.StatusOK,
			data:       map[string]string{"key": "value"},
			wantStatus: http.StatusOK,
			wantBody:   `{"key":"value"}`,
		},
		{
			name:       "struct",
			status:     http.StatusCreated,
			data:       struct{ Name string }{"test"},
			wantStatus: http.StatusCreated,
			wantBody:   `{"Name":"test"}`,
		},
		{
			name:       "nil data",
			status:     http.StatusNoContent,
			data:       nil,
			wantStatus: http.StatusNoContent,
			wantBody:   "null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			JSON(w, tt.status, tt.data)

			if w.Code != tt.wantStatus {
				t.Errorf("JSON() status = %v, want %v", w.Code, tt.wantStatus)
			}

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("JSON() Content-Type = %v, want application/json", contentType)
			}

			got := strings.TrimSpace(w.Body.String())
			if got != tt.wantBody {
				t.Errorf("JSON() body = %v, want %v", got, tt.wantBody)
			}
		})
	}
}

func TestSuccess(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"message": "hello"}

	Success(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Success() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Success() response.Success = false, want true")
	}

	if response.Error != "" {
		t.Errorf("Success() response.Error = %v, want empty", response.Error)
	}

	// Check data field contains our map
	dataMap, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatal("Success() response.Data is not a map")
	}

	if dataMap["message"] != "hello" {
		t.Errorf("Success() data.message = %v, want hello", dataMap["message"])
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		message    string
		wantStatus int
	}{
		{
			name:       "bad request",
			status:     http.StatusBadRequest,
			message:    "invalid input",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "internal error",
			status:     http.StatusInternalServerError,
			message:    "something went wrong",
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "not found",
			status:     http.StatusNotFound,
			message:    "resource not found",
			wantStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()

			Error(w, tt.status, tt.message)

			if w.Code != tt.wantStatus {
				t.Errorf("Error() status = %v, want %v", w.Code, tt.wantStatus)
			}

			var response APIResponse
			if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
				t.Fatalf("Failed to decode response: %v", err)
			}

			if response.Success {
				t.Error("Error() response.Success = true, want false")
			}

			if response.Error != tt.message {
				t.Errorf("Error() response.Error = %v, want %v", response.Error, tt.message)
			}

			if response.Data != nil {
				t.Errorf("Error() response.Data = %v, want nil", response.Data)
			}
		})
	}
}

func TestPaginated(t *testing.T) {
	w := httptest.NewRecorder()
	data := []string{"item1", "item2"}
	meta := &PageMeta{
		Total:   100,
		Page:    1,
		PerPage: 10,
	}

	Paginated(w, http.StatusOK, data, meta)

	if w.Code != http.StatusOK {
		t.Errorf("Paginated() status = %v, want %v", w.Code, http.StatusOK)
	}

	var response PaginatedResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("Paginated() response.Success = false, want true")
	}

	if response.Meta == nil {
		t.Fatal("Paginated() response.Meta is nil")
	}

	if response.Meta.Total != 100 {
		t.Errorf("Paginated() meta.Total = %v, want 100", response.Meta.Total)
	}

	if response.Meta.Page != 1 {
		t.Errorf("Paginated() meta.Page = %v, want 1", response.Meta.Page)
	}

	if response.Meta.PerPage != 10 {
		t.Errorf("Paginated() meta.PerPage = %v, want 10", response.Meta.PerPage)
	}
}

func TestDecodeJSON(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		target  any
		wantErr bool
	}{
		{
			name:    "valid json",
			body:    `{"Name": "test"}`,
			target:  &struct{ Name string }{},
			wantErr: false,
		},
		{
			name:    "invalid json",
			body:    `{invalid`,
			target:  &struct{}{},
			wantErr: true,
		},
		{
			name:    "unknown field",
			body:    `{"unknown_field": "value"}`,
			target:  &struct{ Name string }{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(tt.body))
			err := DecodeJSON(req, tt.target)

			if (err != nil) != tt.wantErr {
				t.Errorf("DecodeJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestDecodeJSON_NilBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Body = nil

	var target struct{}
	err := DecodeJSON(req, &target)

	if err != ErrEmptyBody {
		t.Errorf("DecodeJSON() with nil body error = %v, want %v", err, ErrEmptyBody)
	}
}

func TestValidationError(t *testing.T) {
	w := httptest.NewRecorder()
	errors := map[string]string{
		"email":    "invalid email format",
		"password": "must be at least 8 characters",
	}

	ValidationError(w, errors)

	if w.Code != http.StatusBadRequest {
		t.Errorf("ValidationError() status = %v, want %v", w.Code, http.StatusBadRequest)
	}

	var response struct {
		Success bool              `json:"success"`
		Error   string            `json:"error"`
		Fields  map[string]string `json:"fields"`
	}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Success {
		t.Error("ValidationError() response.Success = true, want false")
	}

	if response.Error != "validation failed" {
		t.Errorf("ValidationError() response.Error = %v, want 'validation failed'", response.Error)
	}

	if len(response.Fields) != 2 {
		t.Errorf("ValidationError() response.Fields length = %v, want 2", len(response.Fields))
	}

	if response.Fields["email"] != "invalid email format" {
		t.Errorf("ValidationError() fields.email = %v, want 'invalid email format'", response.Fields["email"])
	}
}
