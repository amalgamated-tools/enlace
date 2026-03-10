# Architecture

Enlace is a self-hosted file-sharing application built with a Go backend and Svelte frontend. The frontend is compiled and embedded into the Go binary, producing a single self-contained executable.

## Directory Structure

```
enlace/
├── cmd/enlace/          # Application entry point
├── internal/            # Core backend code
│   ├── config/          # Environment-based configuration
│   ├── crypto/          # AES-GCM encryption helpers (used for secrets at rest)
│   ├── database/        # SQLite setup and migrations
│   ├── handler/         # HTTP handlers and router
│   ├── integration/     # Integration tests (//go:build integration)
│   ├── middleware/       # Authentication and rate limiting
│   ├── model/           # Domain types (User, Share, File, etc.)
│   ├── otel/            # Structured logging (slog)
│   ├── repository/      # Data access layer
│   ├── service/         # Business logic layer
│   ├── storage/         # File storage abstraction (local, S3)
│   └── telemetry/       # Optional anonymous telemetry
├── frontend/            # Svelte + TypeScript SPA
│   └── src/
│       ├── routes/      # Page components (top-level pages and admin sub-pages)
│       ├── lib/
│       │   ├── api/     # API client functions (one module per resource)
│       │   ├── components/ # Reusable UI components
│       │   └── stores/  # Svelte stores for auth and UI state
│       └── test/        # Test setup and shared utilities (Vitest)
├── e2e/                 # Playwright end-to-end tests
├── docs/                # Documentation and auto-generated OpenAPI/Swagger specs
├── scripts/             # Utility scripts (e.g., release.sh)
├── Makefile             # Build and dev targets
├── Dockerfile           # Multi-stage Docker build
└── embed.go             # Go embed directive for frontend assets
```

## Backend

### Layered Architecture

Requests flow through four layers:

```
HTTP Request
    ↓
Handler        →  Parses request, validates input, returns JSON responses
    ↓
Service        →  Business logic, authorization, orchestration
    ↓
Repository     →  SQL queries against SQLite
    ↓
Model          →  Domain types and methods
```

The **Storage** layer sits alongside this stack, providing an interface for file operations (Put, Get, Delete, Exists) with local filesystem and S3-compatible implementations.

### Key Technologies

- **Router**: [chi](https://github.com/go-chi/chi) with middleware for CORS, request ID, recovery, and timeouts
- **Database**: SQLite via [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) (pure Go, no CGO)
- **Authentication**: JWT access tokens (15-min expiry, `token_type: "access"`) + refresh tokens (7-day expiry, `token_type: "refresh"`); the `token_type` claim is enforced — access tokens are rejected by the refresh endpoint and refresh tokens are rejected by all other authenticated endpoints; bcrypt password hashing; the JWT signing secret is auto-generated on first run and persisted to `DATA_DIR/jwt_secret` (never user-configurable)
- **OIDC/SSO**: Optional OpenID Connect via [go-oidc](https://github.com/coreos/go-oidc)
- **2FA**: Optional TOTP with QR code setup and recovery codes
- **Storage**: Local filesystem or any S3-compatible backend (AWS, MinIO, RustFS)
- **Email**: SMTP notifications via [go-mail](https://github.com/wneessen/go-mail)
- **API Docs**: Auto-generated Swagger/OpenAPI via [swag](https://github.com/swaggo/swag)

### Configuration

All configuration is done through environment variables. See `.env.sample` for the full list. Key settings include storage backend selection, OIDC provider details, SMTP credentials, and 2FA enforcement. The JWT signing secret is not an environment variable — it is auto-generated and persisted in `DATA_DIR/jwt_secret` (default `./data/jwt_secret`).

Storage settings can also be overridden at runtime via the admin API (`GET/PUT/DELETE /api/v1/admin/storage`), which persists them to the `settings` key-value table in SQLite. DB values take precedence over environment variables on startup. The `s3_secret_key` is encrypted with AES-GCM (key derived from the JWT secret via `internal/crypto`) before being stored. See the [Configuration — Storage](configuration.md#storage) for details.

SMTP settings follow the same pattern: `GET/PUT/DELETE /api/v1/admin/smtp` persists overrides to the same `settings` table, with `smtp_pass` encrypted at rest. See the [Configuration — SMTP](configuration.md#smtp-email-notifications) for details.

### Webhooks

Enlace includes an outbound webhook system that POSTs event notifications to admin-configured HTTPS URLs when specific activities occur. The webhook system spans three layers:

- **`internal/model/webhook.go`** — `WebhookSubscription` and `WebhookDelivery` domain types.
- **`internal/repository/webhook.go`** — SQL queries for managing subscriptions and recording delivery attempts.
- **`internal/service/webhook.go`** — Business logic for creating/updating subscriptions, dispatching events (including retry scheduling), SSRF protection on target URLs, and HMAC-SHA256 request signing.
- **`internal/handler/admin_webhook.go`** — Admin HTTP handlers for subscription CRUD, delivery log access, and the `GET /api/v1/admin/webhooks/events` endpoint that returns the list of supported event types.
- **`internal/handler/webhook_emitter.go`** — Thin helpers that wire share, file, and public handlers to the webhook service, so events are emitted without coupling domain handlers to delivery logic.

Supported events: `share.created`, `file.upload.completed`, `share.viewed`, `share.downloaded`.

Every outgoing POST includes `X-Enlace-Signature` (HMAC-SHA256 over `<timestamp>.<body>`) and an `Idempotency-Key` that is stable across retries. See [Webhook verification and replay protection](api.md#webhook-verification-and-replay-protection) for the full receiver guide.

### API Keys

Enlace supports scoped, long-lived API keys for programmatic access without user credentials. Each key is limited to a declared set of permission scopes (`shares:read`, `shares:write`, `files:read`, `files:write`). Admin-only and user-profile endpoints always require a JWT access token — API keys cannot be used for them.

- **`internal/model/api_key.go`** — `APIKey` domain type.
- **`internal/repository/api_key.go`** — SQL queries for creating, listing, and revoking keys.
- **`internal/service/api_key.go`** — Business logic for key generation (token format `enl_<uuid>_<secret>`), scope validation, bcrypt-equivalent SHA-256 hashing, and authentication via `Authenticate`.
- **`internal/handler/api_key_handler.go`** — HTTP handlers for `GET/POST/DELETE /api/v1/me/api-keys`.
- **`internal/middleware/auth.go`** — Detects `enl_` prefixed tokens and routes them through the API key authentication path instead of JWT validation.

The full key value is returned only once at creation. A 14-character prefix (`key_prefix`) is stored in plaintext for display and identification; the remainder is stored as a SHA-256 hash. See [Admin API key endpoints](api.md#admin-api-key-endpoints) for the complete API reference.

## Frontend

The frontend is a single-page application built with:

- **Svelte 5** for the UI framework
- **TypeScript** for type safety
- **Vite** as the build tool and dev server
- **Tailwind CSS** for styling, with a three-way dark-mode toggle (system / light / dark) that applies `:root[data-theme="dark"]` CSS variable overrides; the preference is persisted in `localStorage` under the key `enlace.theme`
- **svelte-spa-router** for client-side routing

During development, Vite runs on `:5173` and proxies API requests to the Go backend on `:8080`. For production, the frontend is compiled to static assets and embedded into the Go binary via `go:embed`.

## Build and Deployment

### Development

```bash
make dev-setup    # Install Go and frontend dependencies
make dev          # Start backend (Air live-reload) + frontend (Vite) concurrently
```

Optional dev services via `docker-compose-dev.yml`:
- **RustFS**: Local S3-compatible storage on `:9000`
- **Mailpit**: SMTP catch-all on `:1025` with web UI on `:8025`

### Production

```bash
make build        # Build frontend + compile Go binary with embedded assets
make docker-build # Multi-stage Docker image (Alpine-based, non-root user)
```

The Dockerfile uses a three-stage build:
1. **Node stage**: Builds the Svelte frontend
2. **Go stage**: Compiles the backend with embedded frontend
3. **Runtime stage**: Minimal Alpine image with health check

### Testing

```bash
make test              # Go unit tests
make test-coverage     # Go tests with coverage report
cd frontend && pnpm test  # Frontend unit tests (Vitest)
```

The repository tracks `frontend/dist/.gitkeep` so that the `//go:embed all:frontend/dist` directive in `embed.go` is satisfied on a fresh clone. This means `go test ./...` works without a prior frontend build — useful in IDE test runners and backend-only CI jobs. The `ensure-embed-dir` Makefile target recreates the placeholder if the directory is removed (e.g., after `make clean`).

## API Response Format

All API endpoints return a consistent JSON envelope. The `data` and `error` fields are mutually exclusive — `data` appears on success, `error` on failure:

```json
// Success
{ "success": true, "data": { ... } }

// Error
{ "success": false, "error": "<message>" }

// Validation error (HTTP 400)
{ "success": false, "error": "validation failed", "fields": { "<field>": "<reason>" } }
```

Paginated list endpoints additionally include a `meta` object:

```json
{
  "success": true,
  "data": [...],
  "meta": { "total": 42, "page": 1, "per_page": 20 }
}
```
