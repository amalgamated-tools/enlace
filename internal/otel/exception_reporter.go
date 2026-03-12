package otel

import (
	"context"
	"log/slog"
)

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
