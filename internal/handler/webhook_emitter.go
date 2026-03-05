package handler

import (
	"context"

	"github.com/amalgamated-tools/enlace/internal/service"
)

// WebhookEmitter emits integration events.
type WebhookEmitter interface {
	Emit(ctx context.Context, event service.WebhookEvent) error
}
