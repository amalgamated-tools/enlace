package otel

import (
	"log/slog"
	"os"
	"runtime/debug"

	"github.com/amalgamated-tools/enlace/internal/telemetry"
)

var Version = "dev"

func SetupLogger() {
	format := "json"
	level := slog.LevelInfo
	addSource := false

	logFormat, ok := os.LookupEnv("LOG_FORMAT")
	if ok {
		format = logFormat
	}

	logLevel, ok := os.LookupEnv("LOG_LEVEL")
	if ok {
		switch logLevel {
		case "debug":
			level = slog.LevelDebug
			addSource = true
		case "info":
			level = slog.LevelInfo
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		default:
			level = slog.LevelInfo
		}
	}

	var logger *slog.Logger
	if format == "json" {
		logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{AddSource: addSource, Level: level}))
	} else {
		logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{AddSource: addSource, Level: level}))
	}

	// Try to get version from build info
	info, ok := debug.ReadBuildInfo()
	if ok {
		if info.Main.Version != "" {
			Version = info.Main.Version
		}
	}

	logger = logger.With(slog.String("version", Version))
	logger.Info("Logger initialized", slog.String("format", format), slog.String("level", level.String()))
	slog.SetDefault(logger)

	telemetry.Send(Version)
}
