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

### Automated CI screenshot updates

The `.github/workflows/screenshots.yml` workflow runs automatically on every push to `main`. It builds the app from source, starts the server, runs `scripts/take-screenshots.mjs`, and then checks whether any PNG files in `screenshots/` changed.

- **If screenshots changed**, the workflow opens a draft pull request from the `auto/screenshots` branch with the updated images. Review and merge the PR to commit the new screenshots.
- **If screenshots are unchanged**, the workflow exits without creating a PR.

Only one screenshot workflow runs at a time — the `concurrency: screenshots` group cancels any in-progress run when a new push arrives.

## Documentation site (GitHub Pages)

The `.github/workflows/static.yml` workflow deploys the repository contents to **GitHub Pages** on every push to `main`. The published site is available at:

```
https://amalgamated-tools.github.io/enlace/
```

The workflow uploads the entire repository tree as the Pages artifact; GitHub Pages serves the `docs/` Markdown files alongside the project source. No build step is required — Markdown is rendered directly by GitHub Pages.

Only one Pages deployment runs at a time — the `concurrency: pages` group skips any queued runs but never cancels an in-progress deployment.

> **Note:** You cannot trigger this workflow manually from the command line. Use the **Actions → Deploy static content to Pages → Run workflow** button in the GitHub UI, or push a commit to `main`.

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
