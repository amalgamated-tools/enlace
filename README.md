# Sharer

**Sharer** is a self-hosted file sharing application with a Go backend and embedded Svelte frontend. Create shareable links, password-protect them, set download limits or expiry dates, and let others upload files through reverse-share links.

## Features

- **File sharing** — upload files and generate shareable slugs
- **Password protection** — optional per-share password
- **Expiry & limits** — set an expiry date, max downloads, or max views per share
- **Reverse shares** — anyone with the link can upload files to your share
- **User accounts** — local registration and JWT-based authentication
- **OIDC / SSO** — sign in via any OpenID Connect provider (see [OIDC.md](OIDC.md))
- **Admin panel** — manage users via the `/admin` routes
- **Storage backends** — local filesystem or any S3-compatible object store
- **Rate limiting** — per-IP rate limiting on all API endpoints
- **Single binary** — frontend assets are embedded at build time

## Quick Start

### Docker Compose

Copy the sample environment file, set a strong `JWT_SECRET`, then start the stack:

```sh
cp .env.sample .env
# edit .env — at minimum set JWT_SECRET
docker compose up -d
```

Sharer is then available at <http://localhost:8080>.

The first user to register becomes the admin.

### Docker run (minimal)

```sh
docker run -d \
  -p 8080:8080 \
  -e JWT_SECRET=change-me \
  -v sharer-data:/app/data \
  -v sharer-uploads:/app/uploads \
  ghcr.io/amalgamated-tools/sharer:latest
```

## Configuration

All settings are controlled via environment variables. Defaults are shown below.

### Core

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP listen port |
| `DATABASE_PATH` | `./sharer.db` | Path to SQLite database file |
| `JWT_SECRET` | _(required)_ | Secret key used to sign JWT tokens |
| `BASE_URL` | `http://localhost:8080` | Public-facing URL of this instance |

### Storage

| Variable | Default | Description |
|---|---|---|
| `STORAGE_TYPE` | `local` | Storage backend: `local` or `s3` |
| `STORAGE_LOCAL_PATH` | `./uploads` | Directory for local storage |
| `S3_ENDPOINT` | — | S3-compatible endpoint URL |
| `S3_BUCKET` | — | Bucket name |
| `S3_ACCESS_KEY` | — | S3 access key |
| `S3_SECRET_KEY` | — | S3 secret key |
| `S3_REGION` | — | Bucket region |
| `S3_PATH_PREFIX` | — | Optional key prefix for all stored objects |

### SMTP (optional)

| Variable | Default | Description |
|---|---|---|
| `SMTP_HOST` | — | SMTP server hostname |
| `SMTP_PORT` | `587` | SMTP server port |
| `SMTP_USER` | — | SMTP username |
| `SMTP_PASS` | — | SMTP password |
| `SMTP_FROM` | `noreply@example.com` | From address for outgoing mail |

### OIDC / SSO (optional)

| Variable | Default | Description |
|---|---|---|
| `OIDC_ENABLED` | `false` | Set to `true` to enable OIDC |
| `OIDC_ISSUER_URL` | — | Provider issuer URL |
| `OIDC_CLIENT_ID` | — | OAuth 2.0 client ID |
| `OIDC_CLIENT_SECRET` | — | OAuth 2.0 client secret |
| `OIDC_REDIRECT_URL` | — | Callback URL: `https://<host>/api/v1/auth/oidc/callback` |
| `OIDC_SCOPES` | `openid email profile` | Space-separated list of requested scopes |

See [OIDC.md](OIDC.md) for detailed setup instructions and provider-specific examples.

## API Reference

All API endpoints are under `/api/v1`. Authenticated routes require a `Bearer` token in the `Authorization` header.

### Authentication

| Method | Path | Auth | Description |
|---|---|---|---|
| `POST` | `/api/v1/auth/register` | — | Register a new user |
| `POST` | `/api/v1/auth/login` | — | Log in, receive access + refresh tokens |
| `POST` | `/api/v1/auth/refresh` | — | Exchange a refresh token for a new access token |
| `POST` | `/api/v1/auth/logout` | — | Invalidate the current session |
| `GET` | `/api/v1/auth/oidc/config` | — | Get OIDC configuration |
| `GET` | `/api/v1/auth/oidc/login` | — | Start OIDC login flow |
| `GET` | `/api/v1/auth/oidc/callback` | — | OIDC redirect callback |

### Shares

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/shares` | ✓ | List your shares |
| `POST` | `/api/v1/shares` | ✓ | Create a new share |
| `GET` | `/api/v1/shares/{id}` | ✓ | Get share details |
| `PATCH` | `/api/v1/shares/{id}` | ✓ | Update a share |
| `DELETE` | `/api/v1/shares/{id}` | ✓ | Delete a share |
| `GET` | `/api/v1/shares/{id}/files` | ✓ | List files in a share |
| `POST` | `/api/v1/shares/{id}/files` | ✓ | Upload a file to a share |

### Files

| Method | Path | Auth | Description |
|---|---|---|---|
| `DELETE` | `/api/v1/files/{id}` | ✓ | Delete a file |

### User profile

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/me` | ✓ | Get current user profile |
| `PATCH` | `/api/v1/me` | ✓ | Update profile (name, email) |
| `PUT` | `/api/v1/me/password` | ✓ | Change password |
| `GET` | `/api/v1/me/oidc/link` | ✓ | Start OIDC account linking |
| `DELETE` | `/api/v1/me/oidc` | ✓ | Unlink OIDC identity |

### Admin

All admin routes additionally require the `admin` role.

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/api/v1/admin/users` | ✓ admin | List all users |
| `POST` | `/api/v1/admin/users` | ✓ admin | Create a user |
| `GET` | `/api/v1/admin/users/{id}` | ✓ admin | Get a user |
| `PATCH` | `/api/v1/admin/users/{id}` | ✓ admin | Update a user |
| `DELETE` | `/api/v1/admin/users/{id}` | ✓ admin | Delete a user |

### Public share access

No authentication required.

| Method | Path | Description |
|---|---|---|
| `GET` | `/s/{slug}` | View a share (metadata + file list) |
| `POST` | `/s/{slug}/verify` | Verify share password |
| `GET` | `/s/{slug}/files/{fileId}` | Download a file |
| `GET` | `/s/{slug}/files/{fileId}/preview` | Preview a file inline |
| `POST` | `/s/{slug}/upload` | Upload a file to a reverse share |

### Health

| Method | Path | Description |
|---|---|---|
| `GET` | `/health` | Returns `{"status":"ok"}` |

## Development

### Prerequisites

- [Go 1.22+](https://go.dev/dl/)
- [Node.js 20+](https://nodejs.org/) with [pnpm](https://pnpm.io/)
- [goreman](https://github.com/mattn/goreman) or [overmind](https://github.com/DarthSim/overmind) (for the combined dev server)

### Running locally

```sh
# Install all dependencies
make dev-setup

# Start backend (air, live-reload) + frontend dev server concurrently
make dev
```

The frontend dev proxy forwards API requests to `http://localhost:8080`. Open <http://localhost:5173> in your browser.

### Backend only

If the frontend is already built, you can iterate on the backend without rebuilding the frontend on every change:

```sh
make run-backend
```

### Build

```sh
# Full production build (frontend embedded in binary)
make build

# Backend only (faster, requires existing frontend/dist)
make build-backend
```

### Tests

```sh
make test

# With HTML coverage report
make test-coverage
```

### S3-compatible storage (development)

The dev compose file includes [RustFS](https://rustfs.com/) as a local S3-compatible backend:

```sh
make rustfs        # start
make rustfs-stop   # stop
make rustfs-logs   # tail logs
```

### Linting & formatting

```sh
make lint   # go vet
make fmt    # go fmt
```

## Project Structure

```
.
├── cmd/sharer/        # Application entry point
├── internal/
│   ├── config/        # Environment-variable configuration
│   ├── database/      # SQLite initialization and migrations
│   ├── handler/       # HTTP handlers and router
│   ├── middleware/    # Auth and rate-limit middleware
│   ├── model/         # Domain types (Share, File, User)
│   ├── repository/    # Database access layer
│   ├── service/       # Business logic
│   └── storage/       # Storage interface + local/S3 backends
├── frontend/          # Svelte + TypeScript SPA
├── embed.go           # Embeds frontend/dist into the binary
├── Dockerfile
├── docker-compose.yml
├── Makefile
└── OIDC.md            # OIDC/SSO setup guide
```

## License

See [LICENSE](LICENSE) if present, or check the repository for licensing information.
