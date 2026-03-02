package telemetry

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Payload struct {
	Application string `json:"application"`
	InstallID   string `json:"install_id"`
	Version     string `json:"version"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Timestamp   string `json:"timestamp"`
}

func Send(version string) {
	// Telemetry is opt-in meaning it is disabled by default unless explicitly enabled
	envTelemetryEnabled, ok := os.LookupEnv("TELEMETRY_ENABLED")
	if ok {
		slog.Debug("Telemetry environment variable found", slog.String("TELEMETRY_ENABLED", envTelemetryEnabled))

		if !strings.EqualFold(envTelemetryEnabled, "true") {
			slog.Info("Telemetry is disabled via TELEMETRY_ENABLED environment variable")
			return
		}
	} else {
		slog.Warn("TELEMETRY_ENABLED environment variable not set, telemetry is disabled by default")
		return
	}

	endpoint := os.Getenv("TELEMETRY_ENDPOINT")
	if endpoint == "" {
		slog.Debug("Telemetry endpoint not set, using default")
		endpoint = "https://telemetry-worker.amalgamated-tools.workers.dev"
	}

	slog.Warn("NOTICE: This application collects anonymous telemetry data to help improve the product. To disable telemetry, set the environment variable TELEMETRY_ENABLED=false")

	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	installIDPath := filepath.Join(dataDir, "install_id")
	slog.Debug("Using data directory for install ID", slog.String("path", installIDPath))

	// Only send once per install
	if _, err := os.Stat(installIDPath); err == nil {
		slog.Debug("Telemetry already sent for this install, skipping")
		return
	}

	slog.Debug("Install ID not found, sending telemetry data")
	// Create install ID
	id := uuid.New().String()

	payload := Payload{
		Application: "enlace",
		InstallID:   id,
		Version:     version,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		slog.Error("Failed to marshal telemetry payload", slog.Any("error", err))
		return
	}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer(body))
	if err != nil {
		slog.Error("Failed to create telemetry request", slog.Any("error", err))
		return
	}

	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 3 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		slog.Error("Failed to send telemetry request", slog.Any("error", err))
		return
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("Failed to close telemetry response body", slog.Any("error", err))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Telemetry request failed", slog.Int("status", resp.StatusCode))
		return
	}

	// write out response to log
	slog.Debug("Telemetry sent successfully")
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("Failed to read telemetry response", slog.Any("error", err))
		return
	}
	slog.Debug("Telemetry response", slog.String("body", string(body)))

	err = os.WriteFile(installIDPath, []byte(id), 0644)
	if err != nil {
		slog.Error("Failed to write install ID", slog.Any("error", err))
		return
	}
}
