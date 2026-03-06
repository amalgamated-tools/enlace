# Contributing to Enlace

Thank you for your interest in contributing! This document covers how to set up your development environment, run tests, and submit changes.

## Prerequisites

- Go 1.26+
- Node.js 22+ with [pnpm](https://pnpm.io/)
- [Air](https://github.com/air-verse/air) for live reload
- [goreman](https://github.com/mattn/goreman) or [overmind](https://github.com/DarthSim/overmind) for `make dev`
- [swag](https://github.com/swaggo/swag) (optional; only needed to regenerate the OpenAPI/Swagger docs)

For detailed development environment setup including local S3, email testing, and all Make targets, see [docs/development.md](docs/development.md).

## Development setup

```bash
# Install frontend dependencies and download Go modules
make dev-setup

# Start both backend (with live reload via Air) and frontend (Vite HMR)
make dev
```

The Go backend listens on `http://localhost:8080`. The Vite dev server runs on `http://localhost:5173` and proxies all `/api` requests to the backend.

## Running tests

```bash
# Go tests
make test

# Go tests with HTML coverage report
make test-coverage

# Frontend unit tests (Vitest)
cd frontend && pnpm test
```

> **Tip:** `go test ./...` also works directly without building the frontend first. A committed `frontend/dist/.gitkeep` placeholder satisfies the `//go:embed all:frontend/dist` directive in `embed.go`, so a full frontend build is not required just to run Go tests. The `make test` target calls `ensure-embed-dir` to recreate it if it was removed (e.g., after `make clean`).

## Code style

### Go

- Run `make fmt` before committing (`gofmt` + Prettier for the frontend).
- Run `make lint` to catch issues with `go vet`. CI additionally runs `golangci-lint v2`.
- Follow the existing layered pattern: **handler → service → repository**.
- Use the `APIResponse` envelope (`{success, data, error}`) for all JSON responses via helpers in `internal/handler/response.go`.
- Define errors as package-level sentinel variables (e.g., `var ErrNotFound = errors.New("...")`).
- If you add or modify API handler annotations, regenerate the OpenAPI spec with `make swagger` (requires [swag](https://github.com/swaggo/swag)) and format annotations with `make swagger-fmt`.

### Frontend

- Format with Prettier (`cd frontend && pnpm format`).
- Run TypeScript + Svelte type checks with `cd frontend && pnpm check`.
- Use Svelte 5 syntax and Tailwind CSS utility classes.
- When deriving values from svelte-spa-router's `$location` store, use reactive declarations (`$:`) rather than plain functions. Plain functions are not re-evaluated when the store changes, so Svelte will not update the DOM. For example:

  ```svelte
  // ✅ Reactive — Svelte tracks the $location dependency
  $: dashboardActive = $location === "/";
  $: sharesActive = $location.startsWith("/shares");

  // ❌ Not reactive — $location is read once and never re-evaluated
  function isActive(path: string) { return $location === path; }
  ```

## Adding a feature

1. **Fork** the repository and create a descriptive branch (e.g., `feat/share-notifications`).
2. Write or update tests alongside your code.
3. Run the full test suite and linters.
4. Open a pull request against `main` with a clear description of the change.

## Project layout

```
cmd/enlace/        # application entry point
internal/
  config/          # environment variable loading
  database/        # SQLite init & migrations
  handler/         # HTTP route handlers (chi) + router
  middleware/      # auth and rate-limiting middleware
  model/           # domain types (Share, File, User)
  otel/            # structured logging (slog)
  repository/      # data-access layer (SQLite queries)
  service/         # business logic
  storage/         # Storage interface + local & S3 backends
  telemetry/       # opt-in anonymous telemetry
frontend/          # Svelte + TypeScript + Vite SPA
```

## Reporting issues

Please open a GitHub issue and include:

- A clear description of the problem or feature request.
- Steps to reproduce (for bugs).
- Relevant logs or error messages.
- Your environment (OS, Go version, Docker version if applicable).
