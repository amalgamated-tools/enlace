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
5. If a local user with the same email exists, the OIDC identity is automatically linked.
6. If no matching user exists, a new account is created.

Existing users can also link/unlink their OIDC identity from the **Settings** page.

> **Note:** The OIDC provider must return an `email` claim. If it does not, authentication will fail.

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
      - JWT_SECRET=your-jwt-secret
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

### State mismatch errors
- This usually indicates a cookie issue. Ensure cookies are not being stripped by a reverse proxy.
- If using HTTPS, make sure TLS is properly terminated and `SameSite` cookie rules are not blocking the flow.
