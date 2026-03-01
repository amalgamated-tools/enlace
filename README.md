# Sharer

A self-hosted file-sharing application with a Go backend and Svelte frontend. Create password-protected, expiring shares, set download or view limits, and let others upload files to you via reverse shares.

## Features

- **File shares** — upload files and generate a public link
- **Reverse shares** — let others upload files to a link you control
- **Access controls** — optional password protection, expiry date, download limit, and view limit per share
- **Authentication** — local email/password accounts with JWT; optional OpenID Connect (OIDC/SSO)
- **Storage backends** — local filesystem or any S3-compatible object store
- **Admin panel** — manage users from the UI
- **Rate limiting** — IP-based rate limiting on API endpoints
- **Embeds frontend** — single binary ships the compiled Svelte app

## Quick Start (Docker)

```bash
docker run -d \
  -p 8080:8080 \
  -e JWT_SECRET=change-me \
  -v sharer-data:/data \
  sharer:latest
```

Open <http://localhost:8080> and register your first user.

> **Admin access:** The registration endpoint creates regular (non-admin) users. To grant your first user admin privileges, update the database directly after registering:
>
> ```bash
> sqlite3 /path/to/sharer.db \
>   "UPDATE users SET is_admin = 1 WHERE email = 'you@example.com';"
> ```
>
> Once you have one admin user you can create additional admin users through the Admin panel without touching the database.

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
| `DATABASE_PATH` | `./sharer.db` | Path to the SQLite database file |
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

### SMTP (optional)

| Variable | Default | Description |
|---|---|---|
| `SMTP_HOST` | — | SMTP server hostname |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USER` | — | SMTP username |
| `SMTP_PASS` | — | SMTP password |
| `SMTP_FROM` | `noreply@example.com` | Sender address |

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

### Response envelope

Every API response is wrapped in a standard JSON envelope:

```json
{ "success": true, "data": { ... } }
```

On errors, `data` is omitted and an `error` field is present instead:

```json
{ "success": false, "error": "description" }
```

Validation failures return HTTP 400 with a `fields` map:

```json
{ "success": false, "error": "validation failed", "fields": { "email": "invalid email format" } }
```

### JWT tokens

| Token | Lifetime |
|---|---|
| Access token | 15 minutes |
| Refresh token | 7 days |

Use `POST /api/v1/auth/refresh` with the refresh token to obtain a new pair before the access token expires.

### Rate limiting

Requests exceeding the limit receive HTTP 429 with `{"error":"rate limit exceeded"}`.

| Endpoint group | Limit |
|---|---|
| `POST /api/v1/auth/login` | 5 requests / minute |
| `POST /api/v1/auth/register` | 3 requests / minute |
| All other API endpoints | 60 requests / minute |

### Endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/health` | — | Health check |
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

- Go 1.25+
- Node.js 22+ with [pnpm](https://pnpm.io/)
- [Air](https://github.com/air-verse/air) (live reload) and [goreman](https://github.com/mattn/goreman) (optional, for `make dev`)

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
make lint           # go vet ./...
make fmt            # gofmt
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
make docker-build   # builds sharer:latest
make docker-run     # run the image locally
```

The `Dockerfile` uses a multi-stage build: Node 22 compiles the Svelte frontend, then Go embeds the compiled assets and produces a minimal final image.

## Project Layout

```
cmd/sharer/        # main entrypoint
internal/
  config/          # environment-based configuration
  database/        # SQLite helpers & migrations
  handler/         # HTTP handlers and router (chi)
  middleware/       # auth and rate-limiting middleware
  model/           # domain types (Share, File, User)
  repository/      # data-access layer
  service/         # business logic
  storage/         # Storage interface + local & S3 implementations
frontend/          # Svelte + TypeScript + Vite app
```

## License

See [LICENSE](LICENSE) if present.
