# API

All authenticated endpoints require an `Authorization: Bearer <access_token>` header.

> **Token types:** Enlace issues two distinct JWT token types. Access tokens (`token_type: "access"`, 15-minute expiry) are required for all API calls. Refresh tokens (`token_type: "refresh"`, 7-day expiry) are accepted **only** by `POST /api/v1/auth/refresh` — passing a refresh token to any other endpoint returns HTTP 401. Likewise, presenting an access token to the refresh endpoint returns HTTP 401. This prevents token misuse and limits the blast radius of a leaked token.

## Rate Limiting

Several sensitive endpoints enforce per-IP rate limits to protect against brute-force attacks. Exceeding a limit returns **HTTP 429** with:

```json
{ "error": "rate limit exceeded" }
```

| Endpoint | Limit |
|---|---|
| `POST /api/v1/auth/register` | 3 requests per minute |
| `POST /api/v1/auth/login` | 5 requests per minute |
| `POST /api/v1/auth/2fa/verify` | 5 requests per minute |
| `POST /api/v1/auth/2fa/recovery` | 5 requests per minute |

Limits are tracked per source IP address. When Enlace runs behind a trusted reverse proxy (configured via `TRUSTED_PROXIES`), the real client IP from `X-Forwarded-For` / `X-Real-IP` is used instead of the proxy's address. See [Networking / Reverse Proxy](configuration.md#networking--reverse-proxy) for details.

## Response Format

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

## Auth endpoints

**`POST /api/v1/auth/register`** — create a new user account. All fields are required.

| Field | Type | Required | Description |
|---|---|---|---|
| `email` | string | ✔ | Email address |
| `password` | string | ✔ | Password (minimum 8 characters) |
| `display_name` | string | ✔ | Display name |

Returns HTTP 201 on success with the created user:

```json
{ "success": true, "data": { "id": "<uuid>", "email": "user@example.com", "display_name": "Alice" } }
```

Returns HTTP 409 if the email address is already registered.

**`POST /api/v1/auth/login`** — authenticates the user. Returns `access_token`, `refresh_token`, and `user` on success, or a `pending_token` when 2FA verification is required. All fields are required.

| Field | Type | Required | Description |
|---|---|---|---|
| `email` | string | ✔ | Valid email address |
| `password` | string | ✔ | Account password |

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

**`POST /api/v1/auth/refresh`** — returns a new `access_token` and `refresh_token`. The `refresh_token` field must be a refresh token (i.e. the `token_type` claim is `"refresh"`); supplying an access token returns HTTP 401.

```json
{ "refresh_token": "<token>" }
```

**`POST /api/v1/auth/logout`** — invalidates the session on the client side. Always returns HTTP 200. Discard stored tokens after calling this endpoint.

**`GET /api/v1/auth/oidc/config`** — returns whether OIDC/SSO login is enabled for this Enlace instance. Always available; no authentication required. Clients use this to conditionally show a "Sign in with SSO" button.

```json
{ "success": true, "data": { "enabled": true } }
```

**`GET /api/v1/auth/oidc/login`** — initiates the OIDC authorization code flow with PKCE. **Browser-only.** Redirects the browser to the configured identity provider. Not intended to be called from API clients directly — visit it in a browser tab or trigger it via a link/button. Only available when `OIDC_ENABLED=true`.

**`GET /api/v1/auth/oidc/callback`** — OAuth 2.0 callback endpoint that the identity provider redirects to after authentication. **Browser-only.** Verifies state, exchanges the authorization code for tokens, and redirects the browser to `/#/auth/callback` with a short-lived HttpOnly pending-token cookie set. Only available when `OIDC_ENABLED=true`.

**`POST /api/v1/auth/oidc/exchange`** — exchanges the short-lived HttpOnly pending-token cookie (set during the OIDC callback redirect) for a JWT access and refresh token pair. The cookie is consumed on first use; calling this endpoint a second time returns HTTP 401. This endpoint is called automatically by the frontend SPA immediately after the OIDC redirect lands on `/#/auth/callback`. No request body is required — the cookie is sent automatically by the browser.

Returns the same `access_token`, `refresh_token`, and `user` shape as a normal login success. Only available when `OIDC_ENABLED=true`.

## User profile endpoints

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
| `current_password` | string | ✔ | Current password |
| `new_password` | string | ✔ | New password (min 8 characters) |

## OIDC identity management endpoints

These endpoints let a signed-in user link or unlink an external OIDC identity on their account. All `/me/oidc/*` endpoints require `Authorization: Bearer <access_token>`. OIDC must be enabled on the server (`OIDC_ENABLED=true`) — requests to these endpoints return HTTP 404 when OIDC is disabled.

**`GET /api/v1/me/oidc/link`** — initiates the OIDC account-linking flow for the already-authenticated user.

This is a **browser navigation endpoint** (not a JSON API call). The frontend redirects the browser to this URL. The server sets short-lived HttpOnly cookies (`oidc_state`, `oidc_verifier`, `oidc_link`) and immediately redirects the browser to the OIDC provider for authentication. After the provider callback completes (handled by `/api/v1/me/oidc/callback`), the OIDC identity is linked to the user account and the browser is redirected to `/#/settings?oidc=linked`.

> **Note:** Linking an OIDC identity automatically and permanently removes any active TOTP 2FA configuration on the account. See [OIDC and 2FA](oidc.md#oidc-and-two-factor-authentication-2fa) for details.

**`GET /api/v1/me/oidc/callback`** — OIDC provider callback for the account-linking flow.

This is a **browser-facing redirect endpoint** — it is called automatically by the OIDC provider after the user authenticates, not directly by API clients. The server verifies the `state` parameter against the `oidc_state` cookie, exchanges the authorization code for user info, and links the identity to the user whose ID was stored in the `oidc_link` cookie.

| Outcome | Redirect destination |
|---|---|
| Success | `/#/settings?oidc=linked` |
| Error (state mismatch, exchange failure, etc.) | `/#/login?error=<encoded-message>` |

**`DELETE /api/v1/me/oidc`** — unlinks the OIDC identity from the current user account.

Returns HTTP 200 on success. Returns HTTP 400 if the account has no local password — removing the OIDC link without a password would make the account inaccessible. Set a password first via `PUT /api/v1/me/password`.

```json
// Success
{ "success": true, "data": null }

// Error — no local password set
{ "success": false, "error": "cannot unlink OIDC from account without password" }
```

## Two-factor authentication (2FA) endpoints

> **Note:** OIDC (SSO) and 2FA are mutually exclusive. Accounts with a linked OIDC identity cannot set up or use 2FA — the setup, confirm, disable, and recovery-code endpoints return HTTP 403 for those accounts. OIDC logins also bypass the 2FA verification step. See [OIDC / SSO guide](oidc.md#oidc-and-two-factor-authentication-2fa) for details.

All `/me/2fa/*` endpoints require a valid `Authorization: Bearer <access_token>` header.
The `/auth/2fa/*` endpoints are **unauthenticated** — the `pending_token` (returned by `POST /auth/login` when 2FA is enabled) is passed in the **request body**, not in an `Authorization` header.

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

**`POST /api/v1/auth/2fa/verify`** — complete a two-phase login with a TOTP code. Include the `pending_token` received from `/auth/login` in the request body.

```json
{ "pending_token": "<pending-token>", "code": "123456" }
```

Returns the same shape as a normal `POST /auth/login` success: `access_token`, `refresh_token`, and `user`.

**`POST /api/v1/auth/2fa/recovery`** — complete a two-phase login with a recovery code. Include the `pending_token` received from `/auth/login` in the request body.

```json
{ "pending_token": "<pending-token>", "recovery_code": "abcd-efgh-ijkl-mnop-qrst" }
```

Returns `access_token`, `refresh_token`, and `user`. The used recovery code is consumed and cannot be reused.

## OIDC account management endpoints

These endpoints allow authenticated users to link or unlink an OIDC identity from their account. All require `Authorization: Bearer <access_token>`. Only available when `OIDC_ENABLED=true`; otherwise each endpoint returns HTTP 404.

**`GET /api/v1/me/oidc/link`** — starts the OIDC account-linking flow for the currently authenticated user. **Browser-only.** Sets HttpOnly state and PKCE cookies, then redirects the browser to the identity provider's authorization endpoint. Not intended to be called from API clients directly — navigate to it in the browser (e.g., from the Settings page).

**`GET /api/v1/me/oidc/callback`** — OAuth 2.0 callback endpoint for account linking. **Browser-only.** The identity provider redirects here after the user approves the link request. Verifies state, exchanges the authorization code, links the OIDC identity to the current user account, then redirects to `/#/settings`. If the account already has 2FA enabled, it is automatically disabled (see [OIDC and 2FA](oidc.md#oidc-and-two-factor-authentication-2fa)).

On success redirects to `/#/settings`. On failure redirects to `/#/login?error=<message>`.

**`DELETE /api/v1/me/oidc`** — unlinks the OIDC identity from the current user's account. Requires the account to have a local password set (`has_password: true` in `GET /api/v1/me`) — unlinking from a password-less account would lock the user out, so the request is rejected with HTTP 400 in that case.

Returns HTTP 200 on success:

```json
{ "success": true, "data": null }
```

> **See also:** [OIDC / SSO guide](oidc.md) for provider setup, the SSO + 2FA interaction, and troubleshooting.

## Admin user endpoints

All admin endpoints require authentication with an account that has `is_admin: true`. See [Deployment guide](deployment.md) for instructions on bootstrapping the first admin account.

**`POST /api/v1/admin/users`** — create a user. Returns HTTP 201 on success.

| Field | Type | Required | Description |
|---|---|---|---|
| `email` | string | ✔ | Email address |
| `password` | string | ✔ | Initial password |
| `display_name` | string | ✔ | Display name |
| `is_admin` | bool | | Grant admin privileges |

**`PATCH /api/v1/admin/users/{id}`** — update an existing user. All fields are optional; omitted fields are left unchanged. Returns the updated user object (same shape as admin user responses).

| Field | Type | Description |
|---|---|---|
| `email` | string | New email address |
| `password` | string | New password (admin password reset) |
| `display_name` | string | New display name |
| `is_admin` | bool | Grant or revoke admin privileges |

> **Note:** Returns HTTP 409 if the new email is already taken by another account.

**`GET /api/v1/admin/users`** — list all users. Returns an array of admin user objects.

**`GET /api/v1/admin/users/{id}`** — get a specific user by ID. Returns an admin user object.

**`DELETE /api/v1/admin/users/{id}`** — delete a user. Returns HTTP 200 on success.

Admin user responses include `id`, `email`, `display_name`, `is_admin`, `created_at`, and `updated_at`.

## Admin storage endpoints

All admin storage endpoints require authentication with an account that has `is_admin: true`. Changes take effect after restart.

**`GET /api/v1/admin/storage`** — returns the current storage configuration stored in the database. Environment variable values are not included; this endpoint shows only DB overrides.

Response fields:

| Field | Type | Description |
|---|---|---|
| `storage_type` | string | `local` or `s3`; empty if not overridden |
| `storage_local_path` | string | Local filesystem path; empty if not overridden |
| `s3_endpoint` | string | S3-compatible endpoint URL |
| `s3_bucket` | string | Bucket name |
| `s3_access_key` | string | Access key ID |
| `s3_secret_key_set` | bool | `true` if a secret key is stored (the value is never returned) |
| `s3_region` | string | AWS/compatible region |
| `s3_path_prefix` | string | Optional key prefix inside the bucket |

**`PUT /api/v1/admin/storage`** — updates storage configuration in the database. Only fields that are present in the request body are updated; omitted fields are left unchanged.

| Field | Type | Description |
|---|---|---|
| `storage_type` | string | `local` or `s3` |
| `storage_local_path` | string | Local filesystem path (required when `storage_type` is `local`) |
| `s3_endpoint` | string | S3-compatible endpoint URL |
| `s3_bucket` | string | Bucket name (required when `storage_type` is `s3`) |
| `s3_access_key` | string | Access key ID (required when `storage_type` is `s3`) |
| `s3_secret_key` | string | Secret access key (required when `storage_type` is `s3`; encrypted at rest) |
| `s3_region` | string | AWS/compatible region |
| `s3_path_prefix` | string | Optional key prefix inside the bucket |

The effective configuration (existing DB values merged with the incoming request) is validated before saving. Setting `storage_type` to `s3` without also providing `s3_bucket`, `s3_access_key`, and `s3_secret_key` returns HTTP 400.

Returns the current storage configuration after the update (same shape as `GET`).

**`DELETE /api/v1/admin/storage`** — removes all storage configuration overrides from the database. On next restart, Enlace reverts to the environment variable configuration.

**`POST /api/v1/admin/storage/test`** — tests the S3 connection using the currently effective storage configuration (DB overrides merged with environment variables). This endpoint is only useful when `storage_type` is `s3`; calling it with local storage returns HTTP 422.

Returns HTTP 200 on success. On failure, returns HTTP 422 with a human-readable error message:

```json
{ "success": false, "error": "S3 connection failed: bucket not found" }
```

Possible error messages:

| Message | Cause |
|---|---|
| `S3 connection failed: bucket not found` | The configured bucket does not exist |
| `S3 connection failed: authentication failed` | Invalid access key, secret key, or signature |
| `S3 connection failed` | Other connectivity or configuration error |
| `failed to initialize S3 client` | The current effective configuration is incomplete or malformed |

## Admin file restriction endpoints

All admin file restriction endpoints require authentication with an account that has `is_admin: true`. Changes take effect immediately — no restart required.

**`GET /api/v1/admin/files`** — returns the current file upload restriction overrides stored in the database.

| Field | Type | Description |
|---|---|---|
| `max_file_size` | int or null | Maximum allowed upload size in bytes; `null` means the server default (100 MB) is used |
| `blocked_extensions` | array of strings | Extensions that are rejected on upload (e.g. `[".exe", ".sh"]`); empty array means no extensions are blocked |

**`PUT /api/v1/admin/files`** — updates one or both file restriction settings. Only fields present in the request body are changed; omitted fields are left unchanged.

| Field | Type | Required | Description |
|---|---|---|---|
| `max_file_size` | int | no | New maximum file size in bytes; must be a positive integer |
| `blocked_extensions` | string | no | Comma-separated list of extensions to block (e.g. `".exe,.sh,.bat"`); leading dots and case are normalized automatically |

Returns the current file restriction configuration after the update (same shape as `GET`).

**`DELETE /api/v1/admin/files`** — removes all file restriction overrides from the database, reverting to the server default (100 MB limit, no blocked extensions).

> **Extension normalization:** extensions sent to `PUT` are lowercased, deduplicated, and given a leading dot if one is missing (e.g. `"EXE"` becomes `".exe"`). The same normalization is applied when reading from the database, so manually inserted values are always returned in a consistent form.

## Share endpoints

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

Returns HTTP 201 on success. Returns HTTP 409 if the specified `slug` is already taken by another share.

**`GET /api/v1/shares/{id}`** — retrieve a single share by ID. Returns the share object. Returns HTTP 404 if the share does not exist or is owned by another user.

**`PATCH /api/v1/shares/{id}`** — update a share you own. All fields are optional; omitted fields are left unchanged.

| Field | Type | Description |
|---|---|---|
| `name` | string | New display name (max 255 chars) |
| `description` | string | New description |
| `password` | string | Set or change the share password |
| `clear_password` | bool | Set to `true` to remove the password |
| `expires_at` | string (RFC3339) | New expiry timestamp |
| `clear_expiry` | bool | Set to `true` to remove the expiry |
| `max_downloads` | int | New download limit (≥ 0) |
| `max_views` | int | New view limit (≥ 0) |
| `is_reverse_share` | bool | Enable or disable reverse-share uploads |

> **Note:** `slug` cannot be changed after creation. To notify new recipients, use `POST /api/v1/shares/{id}/notify`.

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

## File endpoints

**`GET /api/v1/shares/{id}/files`** — list files in a share you own. Returns an array of file objects.

**`POST /api/v1/shares/{id}/files`** — upload one or more files to a share you own. The default maximum size per file is **100 MB**; admins can override this limit and block specific extensions via the [Admin file restriction endpoints](#admin-file-restriction-endpoints).

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

Returns HTTP 201 on success with an array of file objects (see [File object](#file-object) below).

## File object

File responses (e.g., from `GET /api/v1/shares/{id}/files`) include:

| Field | Type | Description |
|---|---|---|
| `id` | string | File UUID |
| `name` | string | Original filename |
| `size` | int | File size in bytes |
| `mime_type` | string | Detected MIME type |

**`DELETE /api/v1/files/{id}`** — delete a file from a share you own. Returns HTTP 200 on success. Only the share owner can delete files.

## Public share endpoints

The following endpoints are publicly accessible (no authentication) and are used to view and interact with shares via their slug.

> **HTTP 410 Gone:** All public share endpoints return HTTP 410 when the share has **expired** (`expires_at` is in the past), the **download limit** has been reached (`download_count >= max_downloads`), or the **view limit** has been reached (`view_count >= max_views`). Clients should handle 410 as a terminal "share no longer accessible" state distinct from 404 (share does not exist).

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

**`GET /s/{slug}/files/{fileId}/preview`** — preview a file inline. Serves the file with `Content-Disposition: inline` for safe MIME types (images, PDFs, plain text, etc.), suitable for in-browser preview. **Scriptable MIME types** — `text/html`, `application/xhtml+xml`, `image/svg+xml`, `application/javascript`, `text/javascript`, `text/css`, and `application/xml` — are always forced to `Content-Disposition: attachment` regardless of the endpoint used, to prevent cross-site scripting via inline script execution. All served files also include a `Content-Security-Policy: default-src 'none'` header as defense-in-depth.

---

**`POST /s/{slug}/upload`** — upload files to a reverse share (no authentication required). The default maximum size per file is **100 MB**; the same admin-configured restrictions apply here as for authenticated uploads.

Uses the same `multipart/form-data` format as the authenticated upload endpoint — attach files under the `files` field. Returns HTTP 201 on success with an array of uploaded file objects.

## Notification endpoints

**`GET /api/v1/shares/{id}/recipients`** — list all previously notified recipients for a share. Returns an array of recipient objects (see [Recipient object](#recipient-object) below). Returns an empty array if no notifications have been sent.

**`POST /api/v1/shares/{id}/notify`** — send (or resend) email notifications for a share. Requires SMTP to be configured.

| Field | Type | Required | Description |
|---|---|---|---|
| `recipients` | array of strings | ✔ | One or more email addresses to notify |

Example:

```json
{ "recipients": ["alice@example.com", "bob@example.com"] }
```

## Recipient object

Fields in each recipient object:

| Field | Type | Description |
|---|---|---|
| `id` | string | Recipient UUID |
| `email` | string | Notified email address |
| `sent_at` | string (RFC3339) | Timestamp when the notification was sent |

## Admin API key endpoints

All admin API key endpoints require authentication with an account that has `is_admin: true`. API keys allow programmatic access to Enlace without user credentials. Each key is scoped to a fixed set of permissions and is identified by a short prefix (the first 8 characters) for management purposes. The full key value is returned **only once** at creation time.

**`GET /api/v1/admin/api-keys`** — list all API keys created by the current admin account.

Returns an array of API key objects:

| Field | Type | Description |
|---|---|---|
| `id` | string (UUID) | Key identifier |
| `name` | string | Human-readable label |
| `key_prefix` | string | First 8 characters of the key (safe to display) |
| `scopes` | array of strings | Granted permission scopes |
| `revoked_at` | string (RFC3339) or null | When the key was revoked; `null` if still active |
| `last_used_at` | string (RFC3339) or null | When the key was last used; `null` if never used |
| `created_at` | string (RFC3339) | Creation timestamp |

**`POST /api/v1/admin/api-keys`** — create a scoped API key. Returns HTTP 201 on success.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | ✔ | Human-readable label for the key |
| `scopes` | array of strings | ✔ | One or more permission scopes to grant |

Supported scopes:

| Scope | Grants access to |
|---|---|
| `shares:read` | Read shares and their metadata |
| `shares:write` | Create, update, and delete shares |
| `files:read` | Download and list files |
| `files:write` | Upload and delete files |

Returns the created key object plus the full key value (same fields as list, with an additional `key` field):

```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "name": "CI uploader",
    "key_prefix": "enlk_abc",
    "scopes": ["files:write", "shares:write"],
    "revoked_at": null,
    "last_used_at": null,
    "created_at": "2026-01-01T00:00:00Z",
    "key": "enlk_abcdef1234567890..."
  }
}
```

> **Important:** The `key` field is returned **only at creation time** and cannot be retrieved later. Store it securely immediately.

Returns HTTP 400 if `name` is empty or `scopes` is empty. Returns HTTP 422 if any scope value is not in the supported set.

**`DELETE /api/v1/admin/api-keys/{id}`** — revoke an API key. Revoked keys are rejected immediately. Returns HTTP 404 if the key does not exist or belongs to a different admin.

## Admin webhook endpoints

All admin webhook endpoints require authentication with an account that has `is_admin: true`. Webhooks send HTTP `POST` requests to a configured URL whenever specific events occur in Enlace. The `secret` returned at creation is used to sign all outgoing requests — see [Webhook verification and replay protection](#webhook-verification-and-replay-protection) for how to verify signatures.

Supported event types:

| Event | Fired when |
|---|---|
| `file.upload.completed` | A file is successfully uploaded to a share |
| `share.viewed` | A public share is viewed |
| `share.downloaded` | A file is downloaded from a public share |
| `share.created` | A new share is created |

**`GET /api/v1/admin/webhooks`** — list all webhook subscriptions created by the current admin account.

Returns an array of webhook subscription objects:

| Field | Type | Description |
|---|---|---|
| `id` | string (UUID) | Subscription identifier |
| `name` | string | Human-readable label |
| `url` | string | Destination HTTPS URL |
| `events` | array of strings | Subscribed event types |
| `enabled` | bool | Whether the subscription is active |
| `created_at` | string (RFC3339) | Creation timestamp |
| `updated_at` | string (RFC3339) | Last update timestamp |

**`POST /api/v1/admin/webhooks`** — create a webhook subscription. Returns HTTP 201 on success.

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | ✔ | Human-readable label |
| `url` | string | ✔ | Destination URL (must be `https://`; localhost/loopback URLs are rejected to prevent SSRF) |
| `events` | array of strings | | Event types to subscribe to; omit or pass `[]` to subscribe to all supported events |

Returns the subscription object plus the signing secret (same fields as list, with an additional `secret` field):

```json
{
  "success": true,
  "data": {
    "id": "uuid",
    "name": "My receiver",
    "url": "https://example.com/hook",
    "events": ["file.upload.completed", "share.created"],
    "enabled": true,
    "created_at": "2026-01-01T00:00:00Z",
    "updated_at": "2026-01-01T00:00:00Z",
    "secret": "whsec_..."
  }
}
```

> **Important:** The `secret` is returned **only at creation time** and cannot be retrieved later. Store it immediately and use it to verify `X-Enlace-Signature` headers on incoming requests.

Returns HTTP 400 if `name` or `url` is empty. Returns HTTP 422 if the URL is not a valid HTTPS URL or resolves to a loopback/localhost address. Returns HTTP 422 if `events` contains an unsupported event name.

**`PATCH /api/v1/admin/webhooks/{id}`** — update an existing webhook subscription. All fields are optional; omitted fields are left unchanged.

| Field | Type | Description |
|---|---|---|
| `name` | string | New label |
| `url` | string | New destination URL |
| `events` | array of strings | Replacement event list (replaces the entire subscription set) |
| `enabled` | bool | Enable or disable the subscription |

Returns the updated subscription object (same shape as list). Returns HTTP 404 if the subscription does not exist or belongs to a different admin.

**`DELETE /api/v1/admin/webhooks/{id}`** — delete a webhook subscription. Pending deliveries for this subscription will not be retried. Returns HTTP 404 if the subscription does not exist or belongs to a different admin.

**`GET /api/v1/admin/webhooks/deliveries`** — list recent delivery attempts for the current admin's webhook subscriptions. Accepts optional query parameters:

| Parameter | Type | Description |
|---|---|---|
| `subscription_id` | string | Filter by subscription ID |
| `status` | string | Filter by status: `pending`, `delivered`, or `failed` |
| `event_type` | string | Filter by event type (e.g. `share.created`) |
| `limit` | int | Maximum number of results (1–500, default 100) |

Returns an array of delivery objects:

| Field | Type | Description |
|---|---|---|
| `id` | string (UUID) | Delivery identifier |
| `subscription_id` | string | Subscription this delivery belongs to |
| `event_type` | string | Event that triggered the delivery |
| `event_id` | string | Stable identifier for the emitted event |
| `idempotency_key` | string | `{event_id}:{subscription_id}` — reused across retries |
| `attempt` | int | Attempt number (starts at 1) |
| `status` | string | `pending`, `delivered`, or `failed` |
| `status_code` | int or null | HTTP status code returned by the receiver |
| `next_attempt_at` | string (RFC3339) or null | When the next retry will be sent; `null` if delivered or no retry scheduled |
| `delivered_at` | string (RFC3339) or null | When the delivery succeeded; `null` if not yet delivered |
| `error` | string | Error message if delivery failed; empty string otherwise |
| `duration_ms` | int | Round-trip time in milliseconds |
| `created_at` | string (RFC3339) | When this delivery attempt was created |

## Webhook verification and replay protection

Webhook deliveries include the headers below:

- `X-Enlace-Event`: canonical event type.
- `X-Enlace-Event-Id`: stable event identifier.
- `X-Enlace-Timestamp`: RFC3339 timestamp used in signing.
- `X-Enlace-Signature`: `sha256=<hex>` HMAC signature over `<timestamp>.<raw_request_body>`.
- `Idempotency-Key`: stable key for that event + subscription, reused across retries.

Receiver guidance:

1. Recompute HMAC-SHA256 with your shared webhook secret and compare signatures in constant time.
2. Reject messages with stale timestamps (recommended window: <=5 minutes) to limit replay risk.
3. Store `Idempotency-Key` and ignore duplicates so retried deliveries are safe to process multiple times.

## Endpoint reference

| Method | Path | Auth | Description |
|---|---|---|---|
| `GET` | `/health` | — | Health check |
| `GET` | `/swagger/*` | — | Swagger UI (requires `SWAGGER_ENABLED=true`) |
| `POST` | `/api/v1/auth/register` | — | Create account |
| `POST` | `/api/v1/auth/login` | — | Obtain JWT tokens (may return `pending_token` when 2FA is active) |
| `POST` | `/api/v1/auth/refresh` | — | Refresh access token |
| `POST` | `/api/v1/auth/logout` | — | Revoke refresh token |
| `POST` | `/api/v1/auth/2fa/verify` | — | Complete 2FA login with TOTP code (pass `pending_token` in body) |
| `POST` | `/api/v1/auth/2fa/recovery` | — | Complete 2FA login with recovery code (pass `pending_token` in body) |
| `GET` | `/api/v1/auth/oidc/config` | — | OIDC feature flag |
| `GET` | `/api/v1/auth/oidc/login` | — | Start OIDC flow |
| `GET` | `/api/v1/auth/oidc/callback` | — | OIDC callback (redirects to frontend with pending cookie) |
| `POST` | `/api/v1/auth/oidc/exchange` | — | Exchange pending OIDC cookie for JWT token pair |
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
| `GET` | `/api/v1/admin/storage` | ✔ admin | Get storage configuration |
| `PUT` | `/api/v1/admin/storage` | ✔ admin | Update storage configuration |
| `DELETE` | `/api/v1/admin/storage` | ✔ admin | Clear storage configuration (revert to env vars) |
| `POST` | `/api/v1/admin/storage/test` | ✔ admin | Test S3 connection with current effective configuration |
| `GET` | `/api/v1/admin/files` | ✔ admin | Get file upload restriction configuration |
| `PUT` | `/api/v1/admin/files` | ✔ admin | Update file upload restrictions |
| `DELETE` | `/api/v1/admin/files` | ✔ admin | Clear file upload restrictions (revert to defaults) |
| `GET` | `/api/v1/admin/api-keys` | ✔ admin | List API keys created by the current admin |
| `POST` | `/api/v1/admin/api-keys` | ✔ admin | Create a scoped API key (secret returned once) |
| `DELETE` | `/api/v1/admin/api-keys/{id}` | ✔ admin | Revoke an API key |
| `GET` | `/api/v1/admin/webhooks` | ✔ admin | List webhook subscriptions created by the current admin |
| `POST` | `/api/v1/admin/webhooks` | ✔ admin | Create a webhook subscription |
| `PATCH` | `/api/v1/admin/webhooks/{id}` | ✔ admin | Update a webhook subscription |
| `DELETE` | `/api/v1/admin/webhooks/{id}` | ✔ admin | Delete a webhook subscription |
| `GET` | `/api/v1/admin/webhooks/deliveries` | ✔ admin | View webhook delivery logs |
| `GET` | `/s/{slug}` | — | View a public share |
| `POST` | `/s/{slug}/verify` | — | Unlock a password-protected share |
| `GET` | `/s/{slug}/files/{fileId}` | — | Download a file |
| `GET` | `/s/{slug}/files/{fileId}/preview` | — | Preview a file |
| `POST` | `/s/{slug}/upload` | — | Upload to a reverse share |
