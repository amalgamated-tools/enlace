# API

All authenticated endpoints require an `Authorization: Bearer <access_token>` header.

> **Token types:** Enlace issues two distinct JWT token types. Access tokens (`token_type: "access"`, 15-minute expiry) are required for all API calls. Refresh tokens (`token_type: "refresh"`, 7-day expiry) are accepted **only** by `POST /api/v1/auth/refresh` — passing a refresh token to any other endpoint returns HTTP 401. Likewise, presenting an access token to the refresh endpoint returns HTTP 401. This prevents token misuse and limits the blast radius of a leaked token.

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
| `old_password` | string | ✔ | Current password |
| `new_password` | string | ✔ | New password (min 8 characters) |

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
| `GET` | `/api/v1/admin/files` | ✔ admin | Get file upload restriction configuration |
| `PUT` | `/api/v1/admin/files` | ✔ admin | Update file upload restrictions |
| `DELETE` | `/api/v1/admin/files` | ✔ admin | Clear file upload restrictions (revert to defaults) |
| `GET` | `/s/{slug}` | — | View a public share |
| `POST` | `/s/{slug}/verify` | — | Unlock a password-protected share |
| `GET` | `/s/{slug}/files/{fileId}` | — | Download a file |
| `GET` | `/s/{slug}/files/{fileId}/preview` | — | Preview a file |
| `POST` | `/s/{slug}/upload` | — | Upload to a reverse share |
