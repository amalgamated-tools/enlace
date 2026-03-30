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
# Go unit tests
make test

# Go tests with HTML coverage report
make test-coverage

# Go integration tests (requires a running server; gated by //go:build integration)
make test-integration

# Playwright end-to-end tests (builds the app first)
make test-e2e

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

## CI pipeline

GitHub Actions runs four main automated workflows for this repository:

- **Test** (`.github/workflows/test.yml`) runs on pushes and pull requests targeting `main` and `develop`.
  - **Frontend Build & Check** installs frontend dependencies, runs `pnpm check`, `pnpm format --check`, `pnpm test`, and `pnpm build`, then uploads the built `frontend/dist` artifact.
  - **Go Tests** downloads the frontend artifact, runs `golangci-lint`, executes `go test -v ./...`, and checks `gofmt` output.
  - **Integration Tests** downloads the frontend artifact and runs `go test -tags integration -v -count=1 ./internal/integration/...`.
- **E2E Tests** (`.github/workflows/e2etest.yml`) runs on pushes and pull requests targeting `main` and `develop`. It builds the frontend and Go binary, then runs the Playwright suite and uploads the test report.
- **Build Container** (`.github/workflows/docker-build.yml`) runs on pushes to `main`, on published GitHub releases, and on manual dispatch. It builds and publishes the multi-architecture container image to GHCR.
- **Deploy documentation to Pages** (`.github/workflows/static.yml`) runs on pushes to `main` and on manual dispatch. It builds the MkDocs site and deploys it to GitHub Pages.

## Creating a release

Use the helper script to create and push a version tag:

```bash
./scripts/release.sh v1.2.3
```

The script:

1. Validates that the version matches the `vX.Y.Z` format.
2. Verifies that the tag does not already exist.
3. Requires a clean working tree (including no untracked files).
4. Creates an annotated git tag.
5. Pushes the tag to `origin`.

After pushing the tag, publish the corresponding GitHub release to run the release automation in `.github/workflows/docker-build.yml`, which builds and publishes the container image.

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
