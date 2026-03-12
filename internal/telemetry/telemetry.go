package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// installIDMu protects cachedInstallID from concurrent access.
var installIDMu sync.RWMutex

// cachedInstallID is set during SendBoot and read by SendEvent for correlation.
var cachedInstallID string

// Payload is the boot telemetry payload sent once per install.
type Payload struct {
	Application   string `json:"application"`
	InstallID     string `json:"install_id"`
	Version       string `json:"version"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	Timestamp     string `json:"timestamp"`
	UnixTimestamp int64  `json:"unix_timestamp"`
}

// EventPayload is the event telemetry payload sent when telemetry is enabled.
type EventPayload struct {
	Application   string            `json:"application"`
	InstallID     string            `json:"install_id"`
	EventType     string            `json:"event_type"`
	Version       string            `json:"version"`
	Properties    map[string]string `json:"properties,omitempty"`
	Timestamp     string            `json:"timestamp"`
	UnixTimestamp int64             `json:"unix_timestamp"`
}

func getEndpoint() string {
	endpoint := os.Getenv("TELEMETRY_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://telemetry-worker.amalgamated-tools.workers.dev"
	}
	return endpoint
}

func getInstallIDPath() string {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	return filepath.Join(dataDir, "install_id")
}

// loadOrCreateInstallID reads the install ID from disk if it exists,
// or generates a new one. Returns the ID and whether it was newly created.
func loadOrCreateInstallID(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err == nil && len(data) > 0 {
		return string(data), false
	}
	return uuid.New().String(), true
}

func sendPayload(endpoint string, body []byte) (*http.Response, error) {
	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	return client.Do(req)
}

// SendBoot submits the anonymous install telemetry payload.
// This is mandatory and cannot be opted out of. It only sends once per install.
func SendBoot(ctx context.Context, version string) {
	endpoint := getEndpoint()
	installIDPath := getInstallIDPath()
	slog.DebugContext(ctx, "Using data directory for install ID", slog.String("path", installIDPath))

	slog.InfoContext(ctx, "NOTICE: This application collects anonymous boot telemetry data (application version, OS, architecture) once per install to help improve the product.")

	id, isNew := loadOrCreateInstallID(installIDPath)
	installIDMu.Lock()
	cachedInstallID = id
	installIDMu.Unlock()

	if !isNew {
		slog.DebugContext(ctx, "Telemetry already sent for this install, skipping")
		return
	}

	slog.DebugContext(ctx, "Install ID not found, sending telemetry data")

	now := time.Now().UTC()

	payload := Payload{
		Application:   "enlace",
		InstallID:     id,
		Version:       version,
		OS:            runtime.GOOS,
		Arch:          runtime.GOARCH,
		Timestamp:     now.Format(time.RFC3339),
		UnixTimestamp: now.Unix(),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to marshal telemetry payload", slog.Any("error", err))
		return
	}

	resp, err := sendPayload(endpoint, body)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to send telemetry request", slog.Any("error", err))
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.ErrorContext(ctx, "Failed to close telemetry response body", slog.Any("error", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "Telemetry request failed", slog.Int("status", resp.StatusCode))
		return
	}

	slog.DebugContext(ctx, "Telemetry sent successfully")
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to read telemetry response", slog.Any("error", err))
		return
	}
	slog.DebugContext(ctx, "Telemetry response", slog.String("body", string(body)))

	err = os.WriteFile(installIDPath, []byte(id), 0644)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to write install ID", slog.Any("error", err))
		return
	}
}

// SendEvent submits an event telemetry payload. This is opt-in and only sends
// when TELEMETRY_ENABLED=true.
func SendEvent(ctx context.Context, version, eventType string, properties map[string]string) {
	envTelemetryEnabled, ok := os.LookupEnv("TELEMETRY_ENABLED")
	if !ok || !strings.EqualFold(envTelemetryEnabled, "true") {
		slog.DebugContext(ctx, "Event telemetry is disabled, skipping", slog.String("event_type", eventType))
		return
	}

	installIDMu.RLock()
	installID := cachedInstallID
	installIDMu.RUnlock()
	if installID == "" {
		// Try reading from file if SendBoot hasn't been called yet
		data, err := os.ReadFile(getInstallIDPath())
		if err == nil && len(data) > 0 {
			installID = string(data)
			installIDMu.Lock()
			cachedInstallID = installID
			installIDMu.Unlock()
		}
	}

	if installID == "" {
		slog.WarnContext(ctx, "Cannot send event telemetry: no install ID available", slog.String("event_type", eventType))
		return
	}

	now := time.Now().UTC()

	eventPayload := EventPayload{
		Application:   "enlace",
		InstallID:     installID,
		EventType:     eventType,
		Version:       version,
		Properties:    properties,
		Timestamp:     now.Format(time.RFC3339),
		UnixTimestamp: now.Unix(),
	}

	body, err := json.Marshal(eventPayload)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to marshal event telemetry payload", slog.Any("error", err))
		return
	}

	resp, err := sendPayload(getEndpoint(), body)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to send event telemetry request", slog.Any("error", err))
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.ErrorContext(ctx, "Failed to close event telemetry response body", slog.Any("error", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		slog.ErrorContext(ctx, "Event telemetry request failed", slog.Int("status", resp.StatusCode))
		return
	}

	slog.DebugContext(ctx, "Event telemetry sent successfully", slog.String("event_type", eventType))
}

func ReportException(ctx context.Context, err error, message string) {
	ReportExceptionWithMetadata(ctx, err, message, nil)
}

func ReportExceptionWithMetadata(ctx context.Context, err error, message string, metadata map[string]any) {
	if err == nil {
		return
	}
	// Log the error with the provided message and metadata
	slog.ErrorContext(ctx, message, slog.Any("error", err), slog.Any("metadata", metadata))
}

// GetInstallID returns the cached install ID, if available.
func GetInstallID() string {
	installIDMu.RLock()
	defer installIDMu.RUnlock()
	return cachedInstallID
}
