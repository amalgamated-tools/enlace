# Configuration

All settings are read from environment variables (or a `.env` file when running locally).

## Core

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | HTTP port the server listens on |
| `DATABASE_PATH` | `./enlace.db` | Path to the SQLite database file |
| `BASE_URL` | `http://localhost:8080` | Public base URL (used in share links) |
| `DATA_DIR` | `./data` | Directory for persistent runtime state: the auto-generated JWT signing secret and the telemetry install ID. **Security-sensitive** — losing or changing this directory will invalidate all existing JWT tokens (logging out every user) and trigger a new telemetry ping |

## Storage

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

### Admin storage API override

Storage settings can be overridden via the admin API without changing environment variables or redeploying. When a DB override is present, it takes precedence over the corresponding environment variable on startup. Clearing an override via the API (including to an empty string) removes the env-var value for that key as well.

See [Admin storage endpoints](api.md#admin-storage-endpoints) for the API reference.

> **Note:** The `s3_secret_key` is encrypted with AES-GCM before it is stored in the database. The plaintext value is never returned by the GET endpoint; use the `s3_secret_key_set` boolean field to check whether a secret key is configured.

> **Note:** Storage configuration changes **require a restart** to take effect.

## SMTP (email notifications)

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

The `GET /health` endpoint exposes an `email_configured` flag that reflects whether SMTP is active. The frontend reads this flag at startup and hides the **Send via Email** and **Notify by Email** UI elements when email is not configured.

### Admin SMTP API override

SMTP settings can be overridden via the admin API without changing environment variables or redeploying. When a DB override is present, it takes precedence over the corresponding environment variable on startup. Clearing an override via the API removes the env-var value for that key as well.

See [Admin SMTP endpoints](api.md#admin-smtp-endpoints) for the API reference.

> **Note:** The `smtp_pass` is encrypted with AES-GCM before it is stored in the database. The plaintext value is never returned by the GET endpoint; use the `smtp_pass_set` boolean field to check whether a password is configured.

> **Note:** SMTP configuration changes **require a restart** to take effect.

## Logging

| Variable | Default | Description |
|---|---|---|
| `LOG_FORMAT` | `json` | Log output format: `json` or `text`; any other value is treated as `text` |
| `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, or `error`. Setting `debug` also adds source location to each log line |

## Telemetry

Enlace collects **opt-in, anonymous** telemetry to help improve the project. Telemetry is **disabled by default** and only activates when `TELEMETRY_ENABLED=true` is explicitly set. When enabled, Enlace attempts to send a lightweight telemetry ping on startup; after a successful send, it writes an install ID file in `DATA_DIR` and will not send additional pings for that installation. If the request fails or the install ID file cannot be written, the ping will be retried on subsequent startups. Clearing or changing `DATA_DIR` causes Enlace to generate a new install ID and send telemetry again. The payload contains only: application name, a random install ID, version, OS, architecture, and timestamp — no user data, files, or IP addresses.

| Variable | Default | Description |
|---|---|---|
| `TELEMETRY_ENABLED` | `false` | Set to `true` to enable anonymous telemetry |
| `TELEMETRY_ENDPOINT` | `https://telemetry-worker.amalgamated-tools.workers.dev` | Endpoint that receives the telemetry ping (override for self-hosted collection) |

> **Note:** The telemetry install ID is stored in `DATA_DIR` (see [Core](#core)). Changing `DATA_DIR` causes Enlace to generate a new install ID and send telemetry again.

## API & CORS

The Swagger UI is always available at `/swagger/` and the OpenAPI spec at `/swagger/doc.json`. No additional configuration is required.

| Variable | Default | Description |
|---|---|---|
| `CORS_ORIGINS` | *(equals `BASE_URL`)* | Comma-separated list of allowed CORS origins. Defaults to the value of `BASE_URL` when not set |

## Two-Factor Authentication (optional)

Enlace supports TOTP-based 2FA. Users enable it in their account settings; admins can require it for all accounts.

| Variable | Default | Description |
|---|---|---|
| `REQUIRE_2FA` | `false` | Set to `true` to enforce 2FA enrollment for all users. Users who have not yet set up 2FA will receive `requires_2fa_setup: true` on login and must complete TOTP setup before proceeding. |

> **Note:** 2FA and SSO/OIDC are mutually exclusive. When a user links an OIDC identity, any existing 2FA configuration is automatically removed. SSO-linked accounts cannot set up or use 2FA — the identity provider is trusted to handle second-factor concerns. All 2FA mutation endpoints (`/me/2fa/setup`, `/me/2fa/confirm`, `/me/2fa/disable`, `/me/2fa/recovery-codes`) return HTTP 403 for OIDC users, and the 2FA section is hidden in the UI for those accounts. See [OIDC / SSO guide](oidc.md) for details.

## OIDC / SSO (optional)

| Variable | Default | Description |
|---|---|---|
| `OIDC_ENABLED` | `false` | Set to `true` to enable OIDC |
| `OIDC_ISSUER_URL` | — | Provider issuer URL (must expose `/.well-known/openid-configuration`) |
| `OIDC_CLIENT_ID` | — | OAuth 2.0 client ID |
| `OIDC_CLIENT_SECRET` | — | OAuth 2.0 client secret |
| `OIDC_REDIRECT_URL` | — | Callback URL: `https://<host>/api/v1/auth/oidc/callback` |
| `OIDC_SCOPES` | `openid email profile` | Space-separated scope list |

See [OIDC / SSO guide](oidc.md) for provider-specific setup guides.

## Networking / Reverse Proxy

When Enlace is deployed behind a reverse proxy (nginx, Caddy, Traefik, etc.) the direct TCP peer is the proxy, not the end user. By default, Enlace uses `RemoteAddr` for all IP-based decisions (rate limiting). Set `TRUSTED_PROXIES` to the CIDR ranges of your proxy so that the real client IP from `X-Forwarded-For` / `X-Real-IP` is used instead.

| Variable | Default | Description |
|---|---|---|
| `TRUSTED_PROXIES` | *(unset — use `RemoteAddr`)* | Comma-separated list of CIDR ranges whose `X-Forwarded-For` / `X-Real-IP` headers are trusted for client-IP extraction (e.g. rate limiting). Leave unset when not running behind a proxy. |

> **Security note:** Only list IP ranges you control. Any host in a trusted CIDR can spoof arbitrary client IPs by setting `X-Forwarded-For`. Overly broad ranges (e.g. `0.0.0.0/0`) defeat IP-based rate limiting entirely.

**Example — single local proxy:**

```bash
TRUSTED_PROXIES=127.0.0.1/32
```

**Example — RFC 1918 private ranges (typical Docker / Kubernetes setup):**

```bash
TRUSTED_PROXIES=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
```

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
