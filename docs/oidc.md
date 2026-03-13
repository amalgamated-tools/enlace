# OIDC Authentication

Enlace supports OpenID Connect (OIDC) for Single Sign-On (SSO). This allows users to authenticate using an external identity provider instead of (or in addition to) a local email/password.

## Environment Variables

| Variable | Required | Default | Description |
|---|---|---|---|
| `OIDC_ENABLED` | Yes | `false` | Set to `true` to enable OIDC authentication |
| `OIDC_ISSUER_URL` | Yes | — | The OIDC provider's issuer URL (must support discovery at `/.well-known/openid-configuration`) |
| `OIDC_CLIENT_ID` | Yes | — | The client ID from your OIDC provider |
| `OIDC_CLIENT_SECRET` | Yes | — | The client secret from your OIDC provider |
| `OIDC_REDIRECT_URL` | Yes | — | The callback URL: `https://<your-enlace-domain>/api/v1/auth/oidc/callback` |
| `OIDC_SCOPES` | No | `openid email profile` | Space-separated list of OIDC scopes to request |

## How It Works

1. When OIDC is enabled, a **"Sign in with SSO"** button appears on the login page.
2. Clicking it redirects the user to the OIDC provider for authentication.
3. After successful authentication, the provider redirects back to Enlace's callback URL.
4. Enlace extracts the user's email, name, and subject from the ID token.
5. If a local user with the same email exists **and the provider has verified that email address**, the OIDC identity is automatically linked. If the provider reports the email as unverified, sign-in is rejected to prevent account takeover.
6. If no matching user exists, a new account is created.

Existing users can also link/unlink their OIDC identity from the **Settings → Security** tab.

> **Important:** You cannot unlink an OIDC identity from an account that has no local password — doing so would lock you out entirely. Before unlinking, make sure your account has a local password set. You can set one via **Settings → Security** or the `PUT /api/v1/me/password` endpoint. The `has_password` field in `GET /api/v1/me` shows whether your account has a password.

> **Note:** The OIDC provider must return an `email` claim. If it does not, authentication will fail.

## OIDC and Two-Factor Authentication (2FA)

OIDC authentication and TOTP-based 2FA are **mutually exclusive** in Enlace:

- **Linking OIDC disables 2FA.** When you link an OIDC identity to an account that already has 2FA enabled, 2FA is automatically disabled. The OIDC provider is responsible for its own multi-factor authentication.
- **OIDC accounts cannot enable 2FA.** The `POST /api/v1/me/2fa/setup`, `POST /api/v1/me/2fa/confirm`, `POST /api/v1/me/2fa/disable`, and `POST /api/v1/me/2fa/recovery-codes` endpoints return HTTP **403** for accounts with a linked OIDC identity.
- **OIDC logins skip the 2FA verification step.** Even if `REQUIRE_2FA` is set to `true`, users authenticated via OIDC proceed directly to the application without a TOTP challenge.

If you want 2FA enforcement for SSO users, configure MFA at the OIDC provider level.

## General Setup

1. Register Enlace as a client/application in your OIDC provider.
2. Set the callback/redirect URL to: `https://<your-enlace-domain>/api/v1/auth/oidc/callback`
3. Copy the **Client ID** and **Client Secret** from the provider.
4. Note the provider's **Issuer URL** (the base URL that serves `/.well-known/openid-configuration`).
5. Configure the environment variables and restart Enlace.

### Example `.env`

```env
OIDC_ENABLED=true
OIDC_ISSUER_URL=https://auth.example.com
OIDC_CLIENT_ID=your-client-id
OIDC_CLIENT_SECRET=your-client-secret
OIDC_REDIRECT_URL=https://enlace.example.com/api/v1/auth/oidc/callback
OIDC_SCOPES=openid email profile
```

## Pocket ID Example

[Pocket ID](https://github.com/pocket-id/pocket-id) is a lightweight, self-hosted OIDC provider that uses passkey authentication. It's a great choice if you want simple SSO for your self-hosted services without the complexity of Keycloak or similar solutions.

### 1. Set Up Pocket ID

Follow the [Pocket ID installation guide](https://docs.pocket-id.org/docs/setup/installation) to deploy your instance. A minimal Docker Compose setup looks like:

```yaml
services:
  pocket-id:
    image: ghcr.io/pocket-id/pocket-id:latest
    ports:
      - "3000:80"
    environment:
      - PUBLIC_APP_URL=https://auth.example.com
    volumes:
      - pocket-id-data:/app/backend/data
    restart: unless-stopped

volumes:
  pocket-id-data:
```

### 2. Create an OIDC Client in Pocket ID

1. Log in to your Pocket ID admin panel.
2. Navigate to **OIDC Clients** and create a new client.
3. Set the **Name** to `Enlace` (or any name you prefer).
4. Set the **Callback URL** to: `https://<your-enlace-domain>/api/v1/auth/oidc/callback`
5. Copy the **Client ID** and **Client Secret**.
6. Note the **OIDC Discovery URL** — the issuer URL will be your Pocket ID domain (e.g., `https://auth.example.com`).

### 3. Configure Enlace

Set the following environment variables for Enlace (shown here in a `docker-compose.yml`):

```yaml
services:
  enlace:
    image: enlace:latest
    ports:
      - "8080:8080"
    environment:
      - PORT=8080
      - DATABASE_PATH=/app/data/enlace.db
      - BASE_URL=https://enlace.example.com
      - OIDC_ENABLED=true
      - OIDC_ISSUER_URL=https://auth.example.com
      - OIDC_CLIENT_ID=<client-id-from-pocket-id>
      - OIDC_CLIENT_SECRET=<client-secret-from-pocket-id>
      - OIDC_REDIRECT_URL=https://enlace.example.com/api/v1/auth/oidc/callback
      - OIDC_SCOPES=openid email profile
    volumes:
      - enlace-data:/app/data
      - enlace-uploads:/app/uploads
    restart: unless-stopped

volumes:
  enlace-data:
  enlace-uploads:
```

### 4. Test the Integration

1. Open Enlace in your browser.
2. On the login page, click **"Sign in with SSO"**.
3. You should be redirected to Pocket ID to authenticate with your passkey.
4. After authentication, you'll be redirected back to Enlace and logged in.

## SSO and Two-Factor Authentication

**SSO (OIDC) and 2FA are mutually exclusive.** When you link an OIDC identity to a Enlace account — either by signing in for the first time or by linking via the **Settings → Security** tab — any existing TOTP 2FA configuration on that account is automatically and permanently removed.

### Why?

When authentication is delegated to an identity provider, the provider is responsible for all factors of authentication (including second factors such as passkeys, hardware tokens, or its own TOTP). Adding an additional Enlace-managed TOTP layer on top would create redundancy without meaningful security benefit, and could create confusing or inaccessible account states.

### Behaviour for SSO-linked accounts

| Situation | What happens |
|---|---|
| User links an OIDC identity (first login or explicit link) | Any active 2FA is silently removed |
| SSO user attempts to set up 2FA (`POST /me/2fa/setup`) | **HTTP 403** — `"2FA is not available for SSO accounts"` |
| SSO user attempts to confirm setup, disable, or regenerate recovery codes | **HTTP 403** — same error |
| SSO user logs in | 2FA challenge step is skipped entirely |
| 2FA UI in Settings → Security | Hidden for SSO-linked accounts |

### Migrating from 2FA to SSO

If you have 2FA enabled and want to switch to SSO, simply link your OIDC identity via **Settings → Security → Linked Accounts**. Your 2FA configuration will be removed automatically. You do not need to disable 2FA manually first.

### Migrating from SSO to local password + 2FA

1. Ensure your account has a local password set (**Settings → Security → Change Password** or `PUT /api/v1/me/password`).
2. Unlink your OIDC identity via **Settings → Security → Linked Accounts**.
3. You can now enroll in 2FA under **Settings → Security → Two-Factor Authentication**.

## Troubleshooting

### "OIDC is not enabled"
- Verify `OIDC_ENABLED` is set to `true`.
- Ensure all required OIDC environment variables are set.
- Check the application logs for startup errors related to OIDC provider discovery.

### "failed to exchange code" or redirect errors
- Confirm `OIDC_REDIRECT_URL` exactly matches the callback URL registered in your OIDC provider.
- Make sure Enlace can reach the OIDC provider's issuer URL from the server (DNS, network, TLS).
- If running behind a reverse proxy, ensure `BASE_URL` reflects the public-facing URL.

### "OIDC provider did not return email"
- The provider must include an `email` claim in the ID token.
- Ensure the `openid` and `email` scopes are included in `OIDC_SCOPES`.
- In Pocket ID, make sure the user has an email address configured.

### "sign-in rejected: your identity provider has not verified your email address"

- Enlace only auto-links an OIDC identity to an existing local account when the provider's ID token includes `email_verified: true`.
- Check your OIDC provider's settings to ensure it marks user emails as verified (e.g., by requiring email confirmation during registration).
- If the provider never sets `email_verified`, contact the provider's administrators — Enlace cannot override this check because doing so would allow an attacker with control of an unverified email address to hijack an existing account.

### State mismatch errors
- This usually indicates a cookie issue. Ensure cookies are not being stripped by a reverse proxy.
- If using HTTPS, make sure TLS is properly terminated and `SameSite` cookie rules are not blocking the flow.
