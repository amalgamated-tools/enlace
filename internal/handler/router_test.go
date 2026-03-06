package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/amalgamated-tools/enlace/internal/service"
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

	// When no email service is configured, email_configured should be false
	if data["email_configured"] != false {
		t.Errorf("health endpoint data.email_configured = %v, want false", data["email_configured"])
	}
}

func TestHealthHandler_EmailConfigured(t *testing.T) {
	emailSvc := service.NewEmailService(service.SMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
		From: "test@example.com",
	}, nil, "http://localhost")

	cfg := RouterConfig{EmailService: emailSvc}
	router := NewRouter(cfg)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("health endpoint status = %v, want %v", w.Code, http.StatusOK)
	}

	var response APIResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	data, ok := response.Data.(map[string]interface{})
	if !ok {
		t.Fatal("health endpoint response.Data is not a map")
	}

	// NewEmailService with a valid host/port/from creates a mail client,
	// so email_configured should be true
	emailConfigured, _ := data["email_configured"].(bool)
	if !emailConfigured {
		t.Errorf("health endpoint data.email_configured = %v, want true", data["email_configured"])
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

func TestRouter_LoginRateLimited(t *testing.T) {
	cfg := RouterConfig{}
	router := NewRouter(cfg)

	// The login rate limiter allows a burst of 5; sending more requests from the
	// same IP should eventually trigger a 429 Too Many Requests response.
	var rateLimited bool
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "192.0.2.1:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			rateLimited = true
			break
		}
	}

	if !rateLimited {
		t.Error("expected login endpoint to be rate-limited after repeated requests, but it was not")
	}
}

func TestRouter_RegisterRateLimited(t *testing.T) {
	cfg := RouterConfig{}
	router := NewRouter(cfg)

	// The register rate limiter allows a burst of 3; sending more requests from the
	// same IP should eventually trigger a 429 Too Many Requests response.
	var rateLimited bool
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", nil)
		req.RemoteAddr = "192.0.2.2:12345"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code == http.StatusTooManyRequests {
			rateLimited = true
			break
		}
	}

	if !rateLimited {
		t.Error("expected register endpoint to be rate-limited after repeated requests, but it was not")
	}
}
