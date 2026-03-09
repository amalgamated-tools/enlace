# Enlace

A self-hosted file-sharing application with a Go backend and Svelte frontend. Create password-protected, expiring shares, set download or view limits, and let others upload files to you via reverse shares.

## Features

- **File shares** — upload files and generate a public link
- **Reverse shares** — let others upload files to a link you control
- **Access controls** — optional password protection, expiry date, download limit, and view limit per share
- **Authentication** — local email/password accounts with JWT; optional OpenID Connect (OIDC/SSO)
- **Two-factor authentication** — per-user TOTP 2FA with QR-code setup, recovery codes, and optional admin-enforced enrollment (`REQUIRE_2FA`); mutually exclusive with SSO/OIDC
- **Storage backends** — local filesystem or any S3-compatible object store; storage settings can be overridden at runtime via the admin API without redeploying (changes take effect after restart)
- **Admin panel** — manage users, storage, SMTP, webhooks, API keys, and file upload restrictions from the UI or admin API; changes to storage and SMTP take effect on the next restart
- **API keys** — create scoped, long-lived API keys for programmatic access without user credentials; each key is limited to a set of permission scopes
- **Webhooks** — subscribe to server-side events (`share.created`, `file.upload.completed`, etc.) with HMAC-SHA256 signed deliveries, retry logic, and a delivery log
- **Rate limiting** — IP-based rate limiting middleware. `LoginRateLimiter` (5 req/min) is applied to `POST /auth/login`, `RegisterRateLimiter` (3 req/min) to `POST /auth/register`, and `TFAVerifyRateLimiter` (5 req/min) to the 2FA verification endpoints. The `APIRateLimiter` (60 req/min) helper is available in `internal/middleware/ratelimit.go` but not wired up by default. When running behind a reverse proxy, configure `TRUSTED_PROXIES` so that forwarded client IPs are used for rate limiting instead of the proxy's address — see [Networking / Reverse Proxy](docs/configuration.md#networking--reverse-proxy).
- **Email notifications** — optionally email share links to recipients via SMTP; resend from the share detail page
- **Dark mode** — three-way theme toggle (system, light, dark) with preference persisted in the browser
- **Embeds frontend** — single binary ships the compiled Svelte app

## Quick Start

```bash
docker run -d \
  -p 8080:8080 \
  -v enlace-data:/app/data \
  -v enlace-uploads:/app/uploads \
  -e BASE_URL=http://localhost:8080 \
  ghcr.io/amalgamated-tools/enlace:latest
```

Open <http://localhost:8080> and register your first user.

> **First admin bootstrap:** The first user to register on a fresh instance is automatically granted admin privileges. This also applies to the first user created via OIDC sign-in when SSO is enabled. Subsequent registrations create regular users. Once an admin account exists, additional admins can be created or promoted via the admin panel or `POST /api/v1/admin/users`.

For Docker Compose setup and production builds, see the [Deployment guide](docs/deployment.md).

## Documentation

| Topic | Link |
|---|---|
| Configuration | [docs/configuration.md](docs/configuration.md) |
| API Reference | [docs/api.md](docs/api.md) |
| Deployment | [docs/deployment.md](docs/deployment.md) |
| Development | [docs/development.md](docs/development.md) |
| OIDC / SSO | [docs/oidc.md](docs/oidc.md) |
| Architecture | [docs/architecture.md](docs/architecture.md) |
| Contributing | [CONTRIBUTING.md](CONTRIBUTING.md) |
| Code of Conduct | [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) |

## Project Layout

```
cmd/enlace/        # main entrypoint
internal/
  config/          # environment-based configuration
  crypto/          # AES-GCM encryption helpers (secrets at rest)
  database/        # SQLite helpers & migrations
  handler/         # HTTP handlers and router (chi)
  integration/     # integration tests (//go:build integration)
  middleware/       # auth and rate-limiting middleware
  model/           # domain types (Share, File, User)
  otel/            # structured logging setup (slog)
  repository/      # data-access layer
  service/         # business logic
  storage/         # Storage interface + local & S3 implementations
  telemetry/       # anonymous opt-in telemetry
frontend/          # Svelte + TypeScript + Vite app
e2e/               # Playwright end-to-end tests
docs/              # documentation and auto-generated OpenAPI specs
scripts/           # release and utility scripts
```

## License

Enlace is released under the [MIT License](LICENSE).
