package telemetry

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
)

func TestSend_DisabledByDefault(t *testing.T) {
	t.Setenv("TELEMETRY_ENABLED", "")
	os.Unsetenv("TELEMETRY_ENABLED")

	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)

	Send("1.0.0")

	if called.Load() {
		t.Error("telemetry should not be sent when TELEMETRY_ENABLED is not set")
	}
}

func TestSend_DisabledExplicitly(t *testing.T) {
	t.Setenv("TELEMETRY_ENABLED", "false")

	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)

	Send("1.0.0")

	if called.Load() {
		t.Error("telemetry should not be sent when TELEMETRY_ENABLED=false")
	}
}

func TestSend_SuccessfulPayload(t *testing.T) {
	var received Payload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	installIDPath := filepath.Join(tmpDir, "install_id")

	send("2.5.0", srv.URL, installIDPath)

	if received.Application != "enlace" {
		t.Errorf("expected application 'enlace', got %q", received.Application)
	}
	if received.Version != "2.5.0" {
		t.Errorf("expected version '2.5.0', got %q", received.Version)
	}
	if received.OS != runtime.GOOS {
		t.Errorf("expected OS %q, got %q", runtime.GOOS, received.OS)
	}
	if received.Arch != runtime.GOARCH {
		t.Errorf("expected Arch %q, got %q", runtime.GOARCH, received.Arch)
	}
	if received.InstallID == "" {
		t.Error("expected non-empty install_id")
	}
	if received.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}

	// Verify install_id file was written
	data, err := os.ReadFile(installIDPath)
	if err != nil {
		t.Fatalf("install_id file should have been created: %v", err)
	}
	if string(data) != received.InstallID {
		t.Errorf("install_id file content %q doesn't match payload %q", string(data), received.InstallID)
	}
}

func TestSend_OnlyOnce(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	installIDPath := filepath.Join(tmpDir, "install_id")

	// First call should send
	send("1.0.0", srv.URL, installIDPath)
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 call after first send, got %d", callCount.Load())
	}

	// Second call should skip because install_id exists
	send("1.0.0", srv.URL, installIDPath)
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call after second send (should skip), got %d", callCount.Load())
	}
}

func TestSend_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	installIDPath := filepath.Join(tmpDir, "install_id")

	// Should not panic or write install_id on server error
	send("1.0.0", srv.URL, installIDPath)

	if _, err := os.Stat(installIDPath); err == nil {
		t.Error("install_id should not be written when server returns an error")
	}
}

func TestSend_ConnectionFailure(t *testing.T) {
	tmpDir := t.TempDir()
	installIDPath := filepath.Join(tmpDir, "install_id")

	// Point at a closed server to simulate connection failure
	send("1.0.0", "http://127.0.0.1:1", installIDPath)

	if _, err := os.Stat(installIDPath); err == nil {
		t.Error("install_id should not be written when connection fails")
	}
}

func TestSend_WriteFailure(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// Use a path in a non-existent directory so WriteFile fails
	installIDPath := filepath.Join(t.TempDir(), "nonexistent", "subdir", "install_id")

	send("1.0.0", srv.URL, installIDPath)

	// HTTP call should still have been made
	if callCount.Load() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", callCount.Load())
	}

	// install_id should not exist since the write failed
	if _, err := os.Stat(installIDPath); err == nil {
		t.Error("install_id should not exist when write to non-existent directory fails")
	}
}
