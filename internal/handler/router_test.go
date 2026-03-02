package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRouter(t *testing.T) {
	// Create router with empty config (all dependencies nil for this test)
	cfg := RouterConfig{}
	router := NewRouter(cfg)

	if router == nil {
		t.Fatal("NewRouter() returned nil")
	}
}

func TestHealthHandler(t *testing.T) {
	cfg := RouterConfig{}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("health endpoint status = %v, want %v", w.Code, http.StatusOK)
	}

	var response APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if !response.Success {
		t.Error("health endpoint response.Success = false, want true")
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("health endpoint response.Data is not a map")
	}

	if data["status"] != "ok" {
		t.Errorf("health endpoint data.status = %v, want 'ok'", data["status"])
	}
}

func TestHealthHandler_MethodNotAllowed(t *testing.T) {
	cfg := RouterConfig{}
	router := NewRouter(cfg)

	// POST to health endpoint should be method not allowed
	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST to health endpoint status = %v, want %v", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestRouter_NotFoundHandler(t *testing.T) {
	cfg := RouterConfig{}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("nonexistent route status = %v, want %v", w.Code, http.StatusNotFound)
	}
}

func TestRouter_HasRequestIDMiddleware(t *testing.T) {
	cfg := RouterConfig{}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// The request ID middleware should process requests successfully
	// We verify the health endpoint still works with middleware applied
	if w.Code != http.StatusOK {
		t.Errorf("router with middleware status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestRouter_ContentTypeJSON(t *testing.T) {
	cfg := RouterConfig{}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("health endpoint Content-Type = %v, want application/json", contentType)
	}
}

func TestRouter_SwaggerEnabled(t *testing.T) {
	cfg := RouterConfig{SwaggerEnabled: true}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("swagger endpoint with SwaggerEnabled=true status = %v, want %v", w.Code, http.StatusOK)
	}
}

func TestRouter_SwaggerDisabled(t *testing.T) {
	cfg := RouterConfig{SwaggerEnabled: false}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/swagger/index.html", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		t.Errorf("swagger endpoint with SwaggerEnabled=false status = %v, want non-200", w.Code)
	}
}
