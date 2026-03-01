# Copilot Instructions

## Project Overview

Sharer is a self-hosted file sharing and collaboration application. It is a monorepo with a **Go backend** and a **Svelte + TypeScript frontend**. The Go binary embeds the built frontend assets from `frontend/dist` via `//go:embed` (see `embed.go`).

## Tech Stack

### Backend

- **Language:** Go (module `github.com/amalgamated-tools/sharer`)
- **Router:** chi/v5
- **Database:** SQLite via `modernc.org/sqlite`
- **Auth:** JWT (`golang-jwt/jwt/v5`) with optional OIDC SSO (`go-oidc/v3`)
- **Storage:** Local filesystem or S3-compatible (`aws-sdk-go-v2`)

### Frontend

- **Framework:** Svelte 5 with TypeScript
- **Build tool:** Vite
- **Styling:** Tailwind CSS 4 with PostCSS
- **Routing:** svelte-spa-router (client-side SPA)
- **Testing:** Vitest with @testing-library/svelte
- **Package manager:** pnpm
- **Formatting:** Prettier with prettier-plugin-svelte

## Architecture

### Backend (`internal/`)

The backend follows a layered architecture:

- `cmd/sharer/` — Application entry point
- `internal/config/` — Configuration management (environment variables)
- `internal/database/` — Database initialization and migrations
- `internal/handler/` — HTTP route handlers (auth, files, shares, admin, public, OIDC)
- `internal/middleware/` — Auth and rate limiting middleware
- `internal/model/` — Domain models (User, File, Share)
- `internal/repository/` — Data access layer (SQLite queries)
- `internal/service/` — Business logic
- `internal/storage/` — Storage abstraction (local and S3)
- `embed.go` — Embeds the built frontend for serving from the Go binary

### Frontend (`frontend/src/`)

- `routes/` — Page-level Svelte components (Login, Dashboard, Shares, etc.)
- `lib/components/` — Reusable UI components
- `lib/api/` — API client functions
- `lib/stores/` — Svelte stores for state management
- `test/` — Test setup and utilities

## Coding Conventions

### Go

- Use `gofmt` for formatting (no custom formatter config).
- Use `golangci-lint` v2 for linting (default configuration).
- Define errors as package-level sentinel variables (e.g., `var ErrNotFound = errors.New("...")`).
- Use the `APIResponse` envelope (`{success, data, error}`) for all JSON API responses via the helpers in `internal/handler/response.go`: `Success()`, `Error()`, `Paginated()`, `ValidationError()`.
- Decode JSON requests with `DecodeJSON()` which calls `DisallowUnknownFields()`.
- Follow the existing layered pattern: handlers call services, services call repositories.
- Write tests in `*_test.go` files alongside the code they test.

### Frontend (Svelte/TypeScript)

- Format code with Prettier (config in `frontend/.prettierrc`, uses `prettier-plugin-svelte`).
- Use Svelte 5 syntax.
- Use Tailwind CSS utility classes for styling.
- Use svelte-spa-router for client-side routing (routes defined in `src/routes.ts`).
- Write tests using Vitest and @testing-library/svelte.

## Build & Development Commands

```bash
# Development (hot reload backend + frontend)
make dev

# Build everything (frontend + Go binary)
make build

# Run tests
make test                    # Go tests
cd frontend && pnpm test     # Frontend tests

# Linting & formatting
make lint                    # Go vet
make fmt                     # Go format
cd frontend && pnpm check    # TypeScript + Svelte type checking
cd frontend && pnpm format   # Prettier formatting

# Docker
make docker-build
make docker-up
```

## CI Pipeline

The CI workflow (`.github/workflows/test.yml`) runs on pushes to `main`/`develop` and on PRs:

1. **Frontend job:** installs pnpm, runs TypeScript checks (`pnpm check`), formatting checks (`pnpm format --check`), tests (`pnpm test`), and builds the frontend (`pnpm build`).
2. **Go job** (depends on frontend): downloads the built frontend artifact, runs `golangci-lint`, Go tests (`go test -v ./...`), and checks formatting (`gofmt`).

## Configuration

The application is configured via environment variables. See `.env.sample` for all available options including server port, database path, JWT secret, storage settings, SMTP, and OIDC configuration.
