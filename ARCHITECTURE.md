# Architecture

Enlace is a self-hosted file-sharing application built with a Go backend and Svelte frontend. The frontend is compiled and embedded into the Go binary, producing a single self-contained executable.

## Directory Structure

```
enlace/
├── cmd/enlace/          # Application entry point
├── internal/            # Core backend code
│   ├── config/          # Environment-based configuration
│   ├── database/        # SQLite setup and migrations
│   ├── handler/         # HTTP handlers and router
│   ├── middleware/       # Authentication and rate limiting
│   ├── model/           # Domain types (User, Share, File, etc.)
│   ├── otel/            # Structured logging (slog)
│   ├── repository/      # Data access layer
│   ├── service/         # Business logic layer
│   ├── storage/         # File storage abstraction (local, S3)
│   └── telemetry/       # Optional anonymous telemetry
├── frontend/            # Svelte + TypeScript SPA
│   └── src/
│       ├── routes/      # Page components
│       ├── lib/         # Shared components and API client
│       └── test/        # Unit tests
├── docs/                # Generated OpenAPI/Swagger specs
├── scripts/             # Utility scripts
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
- **Authentication**: JWT access tokens (15-min expiry) + refresh tokens (7-day expiry), bcrypt password hashing; the JWT signing secret is auto-generated on first run and persisted to `DATA_DIR/jwt_secret` (never user-configurable)
- **OIDC/SSO**: Optional OpenID Connect via [go-oidc](https://github.com/coreos/go-oidc)
- **2FA**: Optional TOTP with QR code setup and recovery codes
- **Storage**: Local filesystem or any S3-compatible backend (AWS, MinIO, RustFS)
- **Email**: SMTP notifications via [go-mail](https://github.com/wneessen/go-mail)
- **API Docs**: Auto-generated Swagger/OpenAPI via [swag](https://github.com/swaggo/swag)

### Configuration

All configuration is done through environment variables. See `.env.sample` for the full list. Key settings include storage backend selection, OIDC provider details, SMTP credentials, and 2FA enforcement. The JWT signing secret is not an environment variable — it is auto-generated and persisted in `DATA_DIR/jwt_secret` (default `./data/jwt_secret`).

## Frontend

The frontend is a single-page application built with:

- **Svelte 5** for the UI framework
- **TypeScript** for type safety
- **Vite** as the build tool and dev server
- **Tailwind CSS** for styling
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
