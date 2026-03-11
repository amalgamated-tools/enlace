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

// setCachedInstallID is a test helper to safely set the cached install ID.
func setCachedInstallID(id string) {
	installIDMu.Lock()
	cachedInstallID = id
	installIDMu.Unlock()
}

// getCachedInstallID is a test helper to safely read the cached install ID.
func getCachedInstallID() string {
	installIDMu.RLock()
	defer installIDMu.RUnlock()
	return cachedInstallID
}

func TestSendBoot_AlwaysSends(t *testing.T) {
	// Boot telemetry should send even without TELEMETRY_ENABLED
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	os.Unsetenv("TELEMETRY_ENABLED")
	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	t.Setenv("DATA_DIR", tmpDir)
	setCachedInstallID("")

	SendBoot("1.0.0")

	if !called.Load() {
		t.Error("boot telemetry should be sent even when TELEMETRY_ENABLED is not set")
	}
}

func TestSendBoot_AlwaysSendsWhenDisabled(t *testing.T) {
	// Boot telemetry should send even when TELEMETRY_ENABLED=false
	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	t.Setenv("TELEMETRY_ENABLED", "false")
	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	t.Setenv("DATA_DIR", tmpDir)
	setCachedInstallID("")

	SendBoot("1.0.0")

	if !called.Load() {
		t.Error("boot telemetry should be sent even when TELEMETRY_ENABLED=false")
	}
}

func TestSendBoot_SuccessfulPayload(t *testing.T) {
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
	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	t.Setenv("DATA_DIR", tmpDir)
	setCachedInstallID("")

	SendBoot("2.5.0")

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
	installIDPath := filepath.Join(tmpDir, "install_id")
	data, err := os.ReadFile(installIDPath)
	if err != nil {
		t.Fatalf("install_id file should have been created: %v", err)
	}
	if string(data) != received.InstallID {
		t.Errorf("install_id file content %q doesn't match payload %q", string(data), received.InstallID)
	}

	// Verify cachedInstallID is set
	if got := getCachedInstallID(); got != received.InstallID {
		t.Errorf("cachedInstallID %q doesn't match payload %q", got, received.InstallID)
	}
}

func TestSendBoot_OnlyOnce(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	t.Setenv("DATA_DIR", tmpDir)
	setCachedInstallID("")

	// First call should send
	SendBoot("1.0.0")
	if callCount.Load() != 1 {
		t.Fatalf("expected 1 call after first send, got %d", callCount.Load())
	}

	// Second call should skip because install_id exists
	SendBoot("1.0.0")
	if callCount.Load() != 1 {
		t.Errorf("expected 1 call after second send (should skip), got %d", callCount.Load())
	}
}

func TestSendBoot_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	t.Setenv("DATA_DIR", tmpDir)
	setCachedInstallID("")

	SendBoot("1.0.0")

	installIDPath := filepath.Join(tmpDir, "install_id")
	if _, err := os.Stat(installIDPath); err == nil {
		t.Error("install_id should not be written when server returns an error")
	}
}

func TestSendBoot_ConnectionFailure(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("TELEMETRY_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("DATA_DIR", tmpDir)
	setCachedInstallID("")

	SendBoot("1.0.0")

	installIDPath := filepath.Join(tmpDir, "install_id")
	if _, err := os.Stat(installIDPath); err == nil {
		t.Error("install_id should not be written when connection fails")
	}
}

func TestSendBoot_WriteFailure(t *testing.T) {
	var callCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	t.Setenv("DATA_DIR", filepath.Join(t.TempDir(), "nonexistent", "subdir"))
	setCachedInstallID("")

	SendBoot("1.0.0")

	if callCount.Load() != 1 {
		t.Errorf("expected 1 HTTP call, got %d", callCount.Load())
	}
}

// Event telemetry tests

func TestSendEvent_DisabledByDefault(t *testing.T) {
	os.Unsetenv("TELEMETRY_ENABLED")

	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	setCachedInstallID("test-id")

	SendEvent("1.0.0", "test.event", nil)

	if called.Load() {
		t.Error("event telemetry should not be sent when TELEMETRY_ENABLED is not set")
	}
}

func TestSendEvent_DisabledExplicitly(t *testing.T) {
	t.Setenv("TELEMETRY_ENABLED", "false")

	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	setCachedInstallID("test-id")

	SendEvent("1.0.0", "test.event", nil)

	if called.Load() {
		t.Error("event telemetry should not be sent when TELEMETRY_ENABLED=false")
	}
}

func TestSendEvent_Success(t *testing.T) {
	var received EventPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("failed to decode event payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	t.Setenv("TELEMETRY_ENABLED", "true")
	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	t.Setenv("DATA_DIR", tmpDir)
	setCachedInstallID("")

	// Boot first to establish install ID
	SendBoot("1.0.0")
	bootID := getCachedInstallID()

	// Now send event
	props := map[string]string{"key": "value"}
	SendEvent("1.0.0", "share.created", props)

	if received.Application != "enlace" {
		t.Errorf("expected application 'enlace', got %q", received.Application)
	}
	if received.InstallID != bootID {
		t.Errorf("expected install_id %q, got %q", bootID, received.InstallID)
	}
	if received.EventType != "share.created" {
		t.Errorf("expected event_type 'share.created', got %q", received.EventType)
	}
	if received.Version != "1.0.0" {
		t.Errorf("expected version '1.0.0', got %q", received.Version)
	}
	if received.Properties["key"] != "value" {
		t.Errorf("expected property key=value, got %v", received.Properties)
	}
}

func TestSendEvent_NoInstallID(t *testing.T) {
	t.Setenv("TELEMETRY_ENABLED", "true")

	var called atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	t.Setenv("DATA_DIR", t.TempDir()) // empty dir, no install_id file
	setCachedInstallID("")

	SendEvent("1.0.0", "test.event", nil)

	if called.Load() {
		t.Error("event telemetry should not be sent when no install ID is available")
	}
}

func TestSendEvent_ReadsInstallIDFromFile(t *testing.T) {
	var received EventPayload
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("failed to decode event payload: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	t.Setenv("TELEMETRY_ENABLED", "true")
	t.Setenv("TELEMETRY_ENDPOINT", srv.URL)
	t.Setenv("DATA_DIR", tmpDir)
	setCachedInstallID("") // not set via SendBoot

	// Write install_id file manually
	expectedID := "file-based-install-id"
	err := os.WriteFile(filepath.Join(tmpDir, "install_id"), []byte(expectedID), 0644)
	if err != nil {
		t.Fatalf("failed to write install_id file: %v", err)
	}

	SendEvent("1.0.0", "test.event", nil)

	if received.InstallID != expectedID {
		t.Errorf("expected install_id %q from file, got %q", expectedID, received.InstallID)
	}

	// cachedInstallID should now be populated
	if got := getCachedInstallID(); got != expectedID {
		t.Errorf("cachedInstallID should be %q, got %q", expectedID, got)
	}
}
