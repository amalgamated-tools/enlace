# Enlace

A self-hosted file-sharing application with a Go backend and Svelte frontend. Create password-protected, expiring shares, set download or view limits, and let others upload files to you via reverse shares.

## Features

- **File shares** — upload files and generate a public link
- **Reverse shares** — let others upload files to a link you control
- **Access controls** — optional password protection, expiry date, download limit, and view limit per share
- **Authentication** — local email/password accounts with JWT; optional OpenID Connect (OIDC/SSO)
- **Two-factor authentication** — per-user TOTP 2FA with QR-code setup, recovery codes, and optional admin-enforced enrollment (`REQUIRE_2FA`); mutually exclusive with SSO/OIDC
- **Storage backends** — local filesystem or any S3-compatible object store
- **Admin panel** — manage users from the UI
- **Rate limiting** — IP-based rate limiting middleware. `TFAVerifyRateLimiter` (5 req/min) is applied by default to the 2FA login endpoints; the additional pre-built helpers `LoginRateLimiter` (5 req/min), `RegisterRateLimiter` (3 req/min), and `APIRateLimiter` (60 req/min) in `internal/middleware/ratelimit.go` are available but not wired up by default.
- **Email notifications** — optionally email share links to recipients via SMTP; resend from the share detail page
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

### SMTP (email notifications)

Configure SMTP to let Enlace email share links to recipients. Emails are sent as multipart (plain-text + HTML) messages and use opportunistic TLS by default.

| Variable | Default | Description |
|---|---|---|
| `SMTP_HOST` | — | SMTP server hostname |
| `SMTP_PORT` | `587` | SMTP port |
| `SMTP_USER` | — | SMTP username (omit for unauthenticated relays) |
| `SMTP_PASS` | — | SMTP password (omit for unauthenticated relays) |
| `SMTP_FROM` | `noreply@example.com` | Sender address |
| `SMTP_TLS_POLICY` | `opportunistic` | TLS mode: `opportunistic` (STARTTLS when available), `mandatory` (STARTTLS required), or `none` (no TLS) |

Email notifications are **disabled** when `SMTP_HOST` is not set. When configured, you can:

- Supply a `recipients` array on share creation to notify addresses immediately.
- Call `POST /api/v1/shares/{id}/notify` at any time to (re-)send the share link.

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

### Two-Factor Authentication (optional)

Enlace supports TOTP-based 2FA. Users enable it in their account settings; admins can require it for all accounts.

| Variable | Default | Description |
|---|---|---|
| `REQUIRE_2FA` | `false` | Set to `true` to enforce 2FA enrollment for all users. Users who have not yet set up 2FA will receive `requires_2fa_setup: true` on login and must complete TOTP setup before proceeding. |

> **Note:** 2FA and SSO/OIDC are mutually exclusive. When a user links an OIDC identity, any existing 2FA configuration is automatically removed. SSO-linked accounts cannot set up or use 2FA — the identity provider is trusted to handle second-factor concerns. All 2FA mutation endpoints (`/me/2fa/setup`, `/me/2fa/confirm`, `/me/2fa/disable`, `/me/2fa/recovery-codes`) return HTTP 403 for OIDC users, and the 2FA section is hidden in the UI for those accounts. See [OIDC.md](OIDC.md) for details.

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

## CLI Flags

The `enlace` binary accepts optional command-line flags that take precedence over environment variables:

| Flag | Description |
|---|---|
| `-port <n>` | Override the `PORT` environment variable |
| `-version` | Print the version string and exit |

Example:

```bash
enlace -port 9090
enlace -version
```

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

**`POST /api/v1/auth/login`** — authenticates the user. Returns `access_token`, `refresh_token`, and `user` on success, or a `pending_token` when 2FA verification is required.

```json
{ "email": "user@example.com", "password": "secret" }
```

Normal response (no 2FA):

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

**Two-phase login when 2FA is enabled.** When the user has 2FA active, the login response omits tokens and instead returns a short-lived `pending_token`:

```json
{
  "success": true,
  "data": {
    "requires_2fa": true,
    "pending_token": "<short-lived-jwt>"
  }
}
```

Pass the `pending_token` to `POST /api/v1/auth/2fa/verify` (TOTP code) or `POST /api/v1/auth/2fa/recovery` (recovery code) to complete the login and receive real tokens.

**Enforced enrollment.** When `REQUIRE_2FA=true` and the user has not yet set up 2FA, the response includes real tokens **and** a flag prompting the client to redirect to the 2FA setup flow:

```json
{
  "success": true,
  "data": {
    "access_token": "<jwt>",
    "refresh_token": "<token>",
    "user": { "id": "<uuid>", "email": "user@example.com", "display_name": "Alice" },
    "requires_2fa_setup": true
  }
}
```

**`POST /api/v1/auth/refresh`** — returns new `access_token` and `refresh_token`.

```json
{ "refresh_token": "<token>" }
```

**`POST /api/v1/auth/logout`** — invalidates the session on the client side. Always returns HTTP 200. Discard stored tokens after calling this endpoint.

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

**`PATCH /api/v1/me`** — update the current user's profile (all fields optional). Returns the updated profile (same shape as `GET /api/v1/me`).

| Field | Type | Description |
|---|---|---|
| `display_name` | string | New display name |
| `email` | string | New email address |

> **Note:** Omitting a field leaves it unchanged. Returns HTTP 409 if the new email is already taken.

**`PUT /api/v1/me/password`** — change the current user's password.

| Field | Type | Required | Description |
|---|---|---|---|
| `old_password` | string | ✔ | Current password |
| `new_password` | string | ✔ | New password (min 8 characters) |

### Two-factor authentication (2FA) endpoints

> **Note:** OIDC (SSO) and 2FA are mutually exclusive. Accounts with a linked OIDC identity cannot set up or use 2FA — the setup, confirm, disable, and recovery-code endpoints return HTTP 403 for those accounts. OIDC logins also bypass the 2FA verification step. See [OIDC.md](OIDC.md#oidc-and-two-factor-authentication-2fa) for details.

All `/me/2fa/*` endpoints require a valid `Authorization: Bearer <access_token>` header.
The `/auth/2fa/*` endpoints require a `pending_token` (returned by `POST /auth/login` when 2FA is enabled) in the `Authorization: Bearer` header.

**`GET /api/v1/me/2fa/status`** — returns the current user's 2FA status.

Response `data` fields:

| Field | Type | Description |
|---|---|---|
| `enabled` | bool | Whether 2FA is currently enabled for the user |
| `require_2fa` | bool | Whether the server enforces 2FA for all users (`REQUIRE_2FA`) |

**`POST /api/v1/me/2fa/setup`** — begin 2FA setup. Returns the TOTP secret, a base64-encoded QR code image, and a `otpauth://` provisioning URI to scan in an authenticator app.

Response `data` fields:

| Field | Type | Description |
|---|---|---|
| `secret` | string | Raw TOTP secret (for manual entry) |
| `qr_code` | string | Base64-encoded PNG QR code |
| `provisioning_uri` | string | `otpauth://totp/...` URI |

**`POST /api/v1/me/2fa/confirm`** — confirm 2FA setup by submitting a valid TOTP code. Returns one-time recovery codes on success.

```json
{ "code": "123456" }
```

Response `data`:

```json
{ "recovery_codes": ["abcd-efgh-ijkl-mnop-qrst", "..."] }
```

Recovery codes are 80-bit random values in `xxxx-xxxx-xxxx-xxxx-xxxx` format. Store them securely — they are not shown again.

**`POST /api/v1/me/2fa/disable`** — disable 2FA. Requires the user's current password.

```json
{ "password": "current-password" }
```

**`POST /api/v1/me/2fa/recovery-codes`** — regenerate recovery codes. Requires the current password. Invalidates all previous codes.

```json
{ "password": "current-password" }
```

Response `data`:

```json
{ "recovery_codes": ["abcd-efgh-ijkl-mnop-qrst", "..."] }
```

---

**`POST /api/v1/auth/2fa/verify`** — complete a two-phase login with a TOTP code. Pass the `pending_token` in the `Authorization: Bearer` header.

```json
{ "code": "123456" }
```

Returns the same shape as a normal `POST /auth/login` success: `access_token`, `refresh_token`, and `user`.

**`POST /api/v1/auth/2fa/recovery`** — complete a two-phase login with a recovery code. Pass the `pending_token` in the `Authorization: Bearer` header.

```json
{ "code": "abcd-efgh-ijkl-mnop-qrst" }
```

Returns `access_token`, `refresh_token`, and `user`. The used recovery code is consumed and cannot be reused.

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

**`GET /api/v1/admin/users`** — list all users. Returns an array of admin user objects.

**`GET /api/v1/admin/users/{id}`** — get a specific user by ID. Returns an admin user object.

**`DELETE /api/v1/admin/users/{id}`** — delete a user. Returns HTTP 200 on success.

Admin user responses include `id`, `email`, `display_name`, `is_admin`, `created_at`, and `updated_at`.

### Share endpoints

**`GET /api/v1/shares`** — list all shares owned by the authenticated user. Returns an array of share objects.

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
| `recipients` | array of strings | | Email addresses to notify immediately (requires SMTP to be configured) |

**`GET /api/v1/shares/{id}`** — retrieve a single share by ID. Returns the share object. Returns HTTP 404 if the share does not exist or is owned by another user.

**`PATCH /api/v1/shares/{id}`** accepts the same fields (all optional). Use `"clear_password": true` or `"clear_expiry": true` to remove those constraints.

**`DELETE /api/v1/shares/{id}`** — permanently delete a share and all its files. Returns HTTP 200 on success.

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

### File endpoints

**`GET /api/v1/shares/{id}/files`** — list files in a share you own. Returns an array of file objects.

**`POST /api/v1/shares/{id}/files`** — upload one or more files to a share you own.

The request must use `Content-Type: multipart/form-data`. Include each file under the `files` field (repeat the field for multiple files):

```
POST /api/v1/shares/{id}/files
Authorization: Bearer <access_token>
Content-Type: multipart/form-data; boundary=----boundary

------boundary
Content-Disposition: form-data; name="files"; filename="report.pdf"
Content-Type: application/pdf

<binary content>
------boundary--
```

Returns an array of file objects (see [File object](#file-object) below).

### File object

File responses (e.g., from `GET /api/v1/shares/{id}/files`) include:

| Field | Type | Description |
|---|---|---|
| `id` | string | File UUID |
| `name` | string | Original filename |
| `size` | int | File size in bytes |
| `mime_type` | string | Detected MIME type |

**`DELETE /api/v1/files/{id}`** — delete a file from a share you own. Returns HTTP 200 on success. Only the share owner can delete files.

### Public share endpoints

The following endpoints are publicly accessible (no authentication) and are used to view and interact with shares via their slug.

**`GET /s/{slug}`** — retrieve a share's metadata and file list.

- If the share is **not** password-protected, the response is returned immediately.
- If the share **is** password-protected, you must first obtain an access token (see `POST /s/{slug}/verify` below) and pass it in the `X-Share-Token` header.

Response `data` fields:

| Field | Type | Description |
|---|---|---|
| `share` | object | Share metadata (see fields below) |
| `files` | array | List of file objects in the share |

`share` object fields:

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

---

**`POST /s/{slug}/verify`** — unlock a password-protected share and receive an access token.

```json
{ "password": "your-share-password" }
```

On success, returns:

```json
{ "token": "<share-access-token>" }
```

The token is valid for **1 hour**. Pass it in subsequent requests to the same share as either:
- `X-Share-Token: <token>` header, or
- `?token=<token>` query parameter.

---

**`GET /s/{slug}/files/{fileId}`** — download a file. Returns the raw file content with `Content-Disposition: attachment`.

For password-protected shares, include the access token as `X-Share-Token: <token>` or `?token=<token>`.

**`GET /s/{slug}/files/{fileId}/preview`** — preview a file inline. Identical to the download endpoint but serves the file with `Content-Disposition: inline`, suitable for in-browser preview.

---

**`POST /s/{slug}/upload`** — upload files to a reverse share (no authentication required).

Uses the same `multipart/form-data` format as the authenticated upload endpoint — attach files under the `files` field. Returns an array of uploaded file objects.

### Notification endpoints

**`GET /api/v1/shares/{id}/recipients`** — list all previously notified recipients for a share. Returns an array of recipient objects (see [Recipient object](#recipient-object) below). Returns an empty array if no notifications have been sent.

**`POST /api/v1/shares/{id}/notify`** — send (or resend) email notifications for a share. Requires SMTP to be configured.

| Field | Type | Required | Description |
|---|---|---|---|
| `recipients` | array of strings | ✔ | One or more email addresses to notify |

Example:

```json
{ "recipients": ["alice@example.com", "bob@example.com"] }
```

### Recipient object

Fields in each recipient object:

| Field | Type | Description |
|---|---|---|
| `id` | string | Recipient UUID |
| `email` | string | Notified email address |
| `sent_at` | string (RFC3339) | Timestamp when the notification was sent |

### Endpoint reference

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/health` | — | Health check |
| `GET` | `/swagger/*` | — | Swagger UI (requires `SWAGGER_ENABLED=true`) |
| `POST` | `/api/v1/auth/register` | — | Create account |
| `POST` | `/api/v1/auth/login` | — | Obtain JWT tokens (may return `pending_token` when 2FA is active) |
| `POST` | `/api/v1/auth/refresh` | — | Refresh access token |
| `POST` | `/api/v1/auth/logout` | — | Revoke refresh token |
| `POST` | `/api/v1/auth/2fa/verify` | pending | Complete 2FA login with TOTP code |
| `POST` | `/api/v1/auth/2fa/recovery` | pending | Complete 2FA login with recovery code |
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
| `POST` | `/api/v1/shares/{id}/notify` | ✔ | Send email notifications for a share |
| `GET` | `/api/v1/shares/{id}/recipients` | ✔ | List notified recipients for a share |
| `DELETE` | `/api/v1/files/{id}` | ✔ | Delete a file |
| `GET` | `/api/v1/me` | ✔ | Get my profile |
| `PATCH` | `/api/v1/me` | ✔ | Update my profile |
| `PUT` | `/api/v1/me/password` | ✔ | Change password |
| `GET` | `/api/v1/me/2fa/status` | ✔ | Get 2FA status |
| `POST` | `/api/v1/me/2fa/setup` | ✔ | Begin 2FA setup (get QR code) |
| `POST` | `/api/v1/me/2fa/confirm` | ✔ | Confirm 2FA setup and get recovery codes |
| `POST` | `/api/v1/me/2fa/disable` | ✔ | Disable 2FA |
| `POST` | `/api/v1/me/2fa/recovery-codes` | ✔ | Regenerate recovery codes |
| `GET` | `/api/v1/me/oidc/link` | ✔ | Start OIDC link flow |
| `GET` | `/api/v1/me/oidc/callback` | ✔ | OIDC link callback |
| `DELETE` | `/api/v1/me/oidc` | ✔ | Unlink OIDC identity (requires a local password to be set) |
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

### Email (local dev)

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

Enlace is released under the [MIT License](LICENSE).
