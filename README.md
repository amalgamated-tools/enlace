# Enlace

A self-hosted file-sharing application with a Go backend and Svelte frontend. Create password-protected, expiring shares, set download or view limits, and let others upload files to you via reverse shares.

## Features

- **File shares** — upload files and generate a public link
- **Reverse shares** — let others upload files to a link you control
- **Access controls** — optional password protection, expiry date, download limit, and view limit per share
- **Authentication** — local email/password accounts with JWT; optional OpenID Connect (OIDC/SSO)
- **Storage backends** — local filesystem or any S3-compatible object store
- **Admin panel** — manage users from the UI
- **Rate limiting** — IP-based rate limiting middleware included (not applied by default). Pre-built helpers in `internal/middleware/ratelimit.go`: `LoginRateLimiter` (5 req/min), `RegisterRateLimiter` (3 req/min), and `APIRateLimiter` (60 req/min).
- **Embeds frontend** — single binary ships the compiled Svelte app

## Quick Start (Docker)

```bash
docker run -d \
  -p 8080:8080 \
  -e JWT_SECRET=change-me \
  -v enlace-db:/app/data \
  -v enlace-uploads:/app/uploads \
  enlace:latest
```

Open <http://localhost:8080> and register your first user.

### Docker Compose

```bash
cp .env.sample .env   # edit values as needed
docker-compose up -d
```

## Configuration

All settings are read from environment variables (or a `.env` file when running locally).

### Core

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port the server listens on |
| `DATABASE_PATH` | `./enlace.db` | Path to the SQLite database file |
| `JWT_SECRET` | *(required)* | Secret used to sign JWT tokens |
| `BASE_URL` | `http://localhost:8080` | Public base URL (used in share links) |

### Storage

| Variable | Default | Description |
|---|---|---|
| `STORAGE_TYPE` | `local` | `local` or `s3` |
| `STORAGE_LOCAL_PATH` | `./uploads` | Directory for local file storage |
| `S3_ENDPOINT` | — | S3-compatible endpoint URL |
| `S3_BUCKET` | — | Bucket name |
| `S3_ACCESS_KEY` | — | Access key ID |
| `S3_SECRET_KEY` | — | Secret access key |
| `S3_REGION` | — | AWS/compatible region |
| `S3_PATH_PREFIX` | — | Optional key prefix inside the bucket |

### SMTP (reserved for future use)

The following variables are accepted by the configuration loader and are reserved for upcoming email-notification features. No emails are sent in the current release.

| Variable | Default | Description |
|---|---|---|
| `SMTP_HOST` | — | SMTP server hostname |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USER` | — | SMTP username |
| `SMTP_PASS` | — | SMTP password |
| `SMTP_FROM` | `noreply@example.com` | Sender address |

### API & CORS

| Variable | Default | Description |
|---|---|---|
| `SWAGGER_ENABLED` | `false` | Set to `true` to serve the Swagger UI at `/swagger/` and the OpenAPI spec at `/swagger/doc.json` |
| `CORS_ORIGINS` | *(BaseURL)* | Comma-separated list of allowed CORS origins. Defaults to `BASE_URL` when empty |

### Logging

| Variable | Default | Description |
|---|---|---|
| `LOG_FORMAT` | `json` | Log output format: `json` or `text`; any other value is treated as `text` |
| `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, or `error`. Setting `debug` also adds source location to each log line |

### Telemetry

Enlace collects **opt-in, anonymous** telemetry to help improve the project. Telemetry is **disabled by default** and only activates when `TELEMETRY_ENABLED=true` is explicitly set. When enabled, Enlace attempts to send a lightweight telemetry ping on startup; after a successful send, it writes an install ID file in `DATA_DIR` and will not send additional pings for that installation. If the request fails or the install ID file cannot be written, the ping will be retried on subsequent startups. Clearing or changing `DATA_DIR` causes Enlace to generate a new install ID and send telemetry again. The payload contains only: application name, a random install ID, version, OS, architecture, and timestamp — no user data, files, or IP addresses.

| Variable | Default | Description |
|---|---|---|
| `TELEMETRY_ENABLED` | `false` | Set to `true` to enable anonymous telemetry |
| `TELEMETRY_ENDPOINT` | `https://telemetry-worker.amalgamated-tools.workers.dev` | Endpoint that receives the telemetry ping (override for self-hosted collection) |
| `DATA_DIR` | `./data` | Directory used to store the install ID file that prevents duplicate telemetry pings |

### API & CORS

| Variable | Default | Description |
|---|---|---|
| `SWAGGER_ENABLED` | `false` | Set to `true` to serve the Swagger UI at `/swagger/` and the OpenAPI spec at `/swagger/doc.json` |
| `CORS_ORIGINS` | *(equals `BASE_URL`)* | Comma-separated list of allowed CORS origins. Defaults to the value of `BASE_URL` when not set |

### OIDC / SSO (optional)

| Variable | Default | Description |
|---|---|---|
| `OIDC_ENABLED` | `false` | Set to `true` to enable OIDC |
| `OIDC_ISSUER_URL` | — | Provider issuer URL (must expose `/.well-known/openid-configuration`) |
| `OIDC_CLIENT_ID` | — | OAuth 2.0 client ID |
| `OIDC_CLIENT_SECRET` | — | OAuth 2.0 client secret |
| `OIDC_REDIRECT_URL` | — | Callback URL: `https://<host>/api/v1/auth/oidc/callback` |
| `OIDC_SCOPES` | `openid email profile` | Space-separated scope list |

See [OIDC.md](OIDC.md) for provider-specific setup guides.

## API

All authenticated endpoints require an `Authorization: Bearer <access_token>` header.

### Response Format

Every endpoint returns a JSON object with the following envelope:

```json
// Success
{ "success": true, "data": <payload> }

// Error
{ "success": false, "error": "<message>" }

// Validation error (HTTP 400)
{ "success": false, "error": "validation failed", "fields": { "<field>": "<reason>" } }
```

List endpoints that support pagination include a `meta` object:

```json
{
  "success": true,
  "data": [...],
  "meta": { "total": 42, "page": 1, "per_page": 20 }
}
```

Full validation error example:

```json
{
  "success": false,
  "error": "validation failed",
  "fields": {
    "email": "email is required",
    "password": "password must be at least 8 characters"
  }
}
```

### Auth endpoints

**`POST /api/v1/auth/register`**

```json
{ "email": "user@example.com", "password": "secret", "display_name": "Alice" }
```

**`POST /api/v1/auth/login`** — returns `access_token`, `refresh_token`, and `user`.

```json
{ "email": "user@example.com", "password": "secret" }
```

Response:

```json
{
  "success": true,
  "data": {
    "access_token": "<jwt>",
    "refresh_token": "<token>",
    "user": { "id": "<uuid>", "email": "user@example.com", "display_name": "Alice" }
  }
}
```

**`POST /api/v1/auth/refresh`** — returns new `access_token` and `refresh_token`.

```json
{ "refresh_token": "<token>" }
```

### User profile endpoints

**`GET /api/v1/me`** — returns the current user's profile.

Response `data` fields:

| Field | Type | Description |
|---|---|---|
| `id` | string | User UUID |
| `email` | string | Email address |
| `display_name` | string | Display name |
| `is_admin` | bool | Whether the user has admin privileges |
| `oidc_linked` | bool | Whether an OIDC identity is linked |
| `has_password` | bool | Whether the account has a local password set |

**`PATCH /api/v1/me`** — update the current user's profile (all fields optional).

| Field | Type | Description |
|---|---|---|
| `display_name` | string | New display name |
| `email` | string | New email address |

**`PUT /api/v1/me/password`** — change the current user's password.

| Field | Type | Required | Description |
|---|---|---|---|
| `old_password` | string | ✔ | Current password |
| `new_password` | string | ✔ | New password (min 8 characters) |

### Admin user endpoints

All admin endpoints require authentication with an account that has `is_admin: true`.

**`POST /api/v1/admin/users`** — create a user.

| Field | Type | Required | Description |
|---|---|---|---|
| `email` | string | ✔ | Email address |
| `password` | string | ✔ | Initial password |
| `display_name` | string | ✔ | Display name |
| `is_admin` | bool | | Grant admin privileges |

**`PATCH /api/v1/admin/users/{id}`** accepts the same fields (all optional). Omitted fields are unchanged.

Admin user responses include `id`, `email`, `display_name`, `is_admin`, `created_at`, and `updated_at`.

### Share endpoints

**`POST /api/v1/shares`**

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | ✔ | Display name (max 255 chars) |
| `description` | string | | Optional description |
| `slug` | string | | Custom URL slug (3–50 chars, `[a-z0-9-]`); auto-generated if omitted |
| `password` | string | | Password-protect the share |
| `expires_at` | string (RFC3339) | | Expiry timestamp |
| `max_downloads` | int | | Download limit (≥ 0) |
| `max_views` | int | | View limit (≥ 0) |
| `is_reverse_share` | bool | | Allow others to upload files to this share |

**`PATCH /api/v1/shares/{id}`** accepts the same fields (all optional). Use `"clear_password": true` or `"clear_expiry": true` to remove those constraints.

Share responses include the following fields:

| Field | Type | Description |
|---|---|---|
| `id` | string | Share UUID |
| `slug` | string | URL slug used in public links |
| `name` | string | Display name |
| `description` | string | Optional description |
| `has_password` | bool | Whether the share requires a password |
| `expires_at` | string (RFC3339) | Expiry timestamp, omitted if not set |
| `max_downloads` | int | Download limit, omitted if not set |
| `download_count` | int | Number of times files have been downloaded |
| `max_views` | int | View limit, omitted if not set |
| `view_count` | int | Number of times the share has been viewed |
| `is_reverse_share` | bool | Whether others can upload to this share |
| `created_at` | string (RFC3339) | Creation timestamp |
| `updated_at` | string (RFC3339) | Last-updated timestamp |

### File object

File responses (e.g., from `GET /api/v1/shares/{id}/files`) include:

| Field | Type | Description |
|---|---|---|
| `id` | string | File UUID |
| `name` | string | Original filename |
| `size` | int | File size in bytes |
| `mime_type` | string | Detected MIME type |

### Endpoint reference

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/health` | — | Health check |
| `GET` | `/swagger/*` | — | Swagger UI (requires `SWAGGER_ENABLED=true`) |
| `POST` | `/api/v1/auth/register` | — | Create account |
| `POST` | `/api/v1/auth/login` | — | Obtain JWT tokens |
| `POST` | `/api/v1/auth/refresh` | — | Refresh access token |
| `POST` | `/api/v1/auth/logout` | — | Revoke refresh token |
| `GET` | `/api/v1/auth/oidc/config` | — | OIDC feature flag |
| `GET` | `/api/v1/auth/oidc/login` | — | Start OIDC flow |
| `GET` | `/api/v1/auth/oidc/callback` | — | OIDC callback |
| `GET` | `/api/v1/shares` | ✔ | List my shares |
| `POST` | `/api/v1/shares` | ✔ | Create a share |
| `GET` | `/api/v1/shares/{id}` | ✔ | Get share details |
| `PATCH` | `/api/v1/shares/{id}` | ✔ | Update a share |
| `DELETE` | `/api/v1/shares/{id}` | ✔ | Delete a share |
| `GET` | `/api/v1/shares/{id}/files` | ✔ | List files in a share |
| `POST` | `/api/v1/shares/{id}/files` | ✔ | Upload a file to a share |
| `DELETE` | `/api/v1/files/{id}` | ✔ | Delete a file |
| `GET` | `/api/v1/me` | ✔ | Get my profile |
| `PATCH` | `/api/v1/me` | ✔ | Update my profile |
| `PUT` | `/api/v1/me/password` | ✔ | Change password |
| `GET` | `/api/v1/me/oidc/link` | ✔ | Start OIDC link flow |
| `GET` | `/api/v1/me/oidc/callback` | ✔ | OIDC link callback |
| `DELETE` | `/api/v1/me/oidc` | ✔ | Unlink OIDC identity |
| `GET` | `/api/v1/admin/users` | ✔ admin | List all users |
| `POST` | `/api/v1/admin/users` | ✔ admin | Create a user |
| `GET` | `/api/v1/admin/users/{id}` | ✔ admin | Get a user |
| `PATCH` | `/api/v1/admin/users/{id}` | ✔ admin | Update a user |
| `DELETE` | `/api/v1/admin/users/{id}` | ✔ admin | Delete a user |
| `GET` | `/s/{slug}` | — | View a public share |
| `POST` | `/s/{slug}/verify` | — | Unlock a password-protected share |
| `GET` | `/s/{slug}/files/{fileId}` | — | Download a file |
| `GET` | `/s/{slug}/files/{fileId}/preview` | — | Preview a file |
| `POST` | `/s/{slug}/upload` | — | Upload to a reverse share |

## Development

### Prerequisites

- Go 1.26+
- Node.js 22+ with [pnpm](https://pnpm.io/)
- [Air](https://github.com/air-verse/air) for live reload of the Go backend
- [goreman](https://github.com/mattn/goreman) or [overmind](https://github.com/DarthSim/overmind) to run the `Procfile.dev` (optional; only needed for `make dev`)

### Getting started

```bash
# Install frontend dependencies
make dev-setup

# Start backend and frontend dev servers with live reload
make dev
```

The backend defaults to <http://localhost:8080> and the Vite dev server proxies API calls from <http://localhost:5173>.

### Common targets

```bash
make build          # production binary (frontend embedded)
make build-backend  # backend only, faster iteration
make test           # go test ./...
make test-coverage  # test + HTML coverage report
make lint           # go vet ./... (CI also runs golangci-lint v2)
make fmt            # gofmt + Prettier (formats Go and frontend code)
make clean          # remove build artifacts
```

### S3-compatible storage (local dev)

The dev compose file ships [RustFS](https://rustfs.com/), an S3-compatible server:

```bash
make rustfs         # start RustFS in Docker
make rustfs-stop    # stop it
make rustfs-logs    # tail logs
```

Then set `STORAGE_TYPE=s3` and point `S3_ENDPOINT` at `http://localhost:9000`.

## Building a Docker Image

```bash
make docker-build   # builds enlace:latest
make docker-run     # run the image locally
```

The `Dockerfile` uses a multi-stage build: Node 22 compiles the Svelte frontend, then Go embeds the compiled assets and produces a minimal final image.

## Project Layout

```
cmd/enlace/        # main entrypoint
internal/
  config/          # environment-based configuration
  database/        # SQLite helpers & migrations
  handler/         # HTTP handlers and router (chi)
  middleware/       # auth and rate-limiting middleware
  model/           # domain types (Share, File, User)
  otel/            # structured logging setup (slog)
  repository/      # data-access layer
  service/         # business logic
  storage/         # Storage interface + local & S3 implementations
  telemetry/       # anonymous opt-in telemetry
frontend/          # Svelte + TypeScript + Vite app
```

## License

See [LICENSE](LICENSE) if present.
