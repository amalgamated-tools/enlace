# Development

For contribution workflow, code style, and PR guidelines, see [CONTRIBUTING.md](../CONTRIBUTING.md).

## Prerequisites

- Go 1.26+
- Node.js 22+ with [pnpm](https://pnpm.io/)
- [Air](https://github.com/air-verse/air) for live reload of the Go backend
- [goreman](https://github.com/mattn/goreman) or [overmind](https://github.com/DarthSim/overmind) to run the `Procfile.dev` (optional; only needed for `make dev`)
- [swag](https://github.com/swaggo/swag) (optional; only needed to regenerate the OpenAPI/Swagger docs with `make swagger`)

## Getting started

```bash
# Install Go and frontend dependencies
make dev-setup

# Start backend and frontend dev servers with live reload
make dev
```

The backend defaults to <http://localhost:8080> and the Vite dev server proxies API calls from <http://localhost:5173>.

## Common targets

```bash
make build          # production binary (frontend embedded)
make build-backend  # backend only, faster iteration
make run            # build then run the production binary
make run-backend    # run backend without rebuilding (go run ./cmd/enlace)
make test               # go test ./... -v
make test-coverage      # test + HTML coverage report
make test-integration   # integration tests (//go:build integration tag)
make test-e2e           # Playwright end-to-end tests (builds app first)
make lint           # go vet ./... (CI also runs golangci-lint v2)
make fmt            # gofmt + Prettier (formats Go and frontend code)
make clean          # remove build artifacts
make swagger        # regenerate OpenAPI/Swagger docs (requires swag)
make swagger-fmt    # format swag annotations in Go source
```

## Generating screenshots

The `screenshots/` directory contains Playwright-generated PNG images (light and dark, @2× resolution) of every page. To regenerate them locally:

```bash
make screenshots
```

This target cleans previous build artifacts, builds the frontend, starts the Go backend and Vite dev server, runs `scripts/take-screenshots.mjs` via Playwright, then shuts down the servers. Requires [goreman](https://github.com/mattn/goreman) or [overmind](https://github.com/DarthSim/overmind) to be installed.

You can also point the script at an already-running instance:

```bash
cd e2e
BASE_URL=http://localhost:5173 node ../scripts/take-screenshots.mjs
```

## S3-compatible storage (local dev)

The dev compose file ships [RustFS](https://rustfs.com/), an S3-compatible server:

```bash
make rustfs         # start RustFS in Docker
make rustfs-stop    # stop it
make rustfs-logs    # tail logs
```

Then set `STORAGE_TYPE=s3` and point `S3_ENDPOINT` at `http://localhost:9000`.

## Email (local dev)

The dev compose file ships [Mailpit](https://mailpit.axllent.org/), a local SMTP catch-all:

```bash
docker compose -f docker-compose-dev.yml up mailpit
```

| Setting | Value |
|---|---|
| SMTP host | `localhost` |
| SMTP port | `1025` |
| Mailpit UI | <http://localhost:8025> |

Set `SMTP_HOST=localhost`, `SMTP_PORT=1025`, and `SMTP_TLS_POLICY=none` in your `.env` to route all outgoing mail to Mailpit.
