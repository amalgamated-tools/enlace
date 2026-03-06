# Deployment

## Quick Start (Docker)

Pull the pre-built image from the GitHub Container Registry and run it:

```bash
docker run -d \
  -p 8080:8080 \
  -v enlace-data:/app/data \
  -v enlace-uploads:/app/uploads \
  -e BASE_URL=http://localhost:8080 \
  ghcr.io/amalgamated-tools/enlace:latest
```

Open <http://localhost:8080> and register your first user.

> **First admin bootstrap:** The first user to register on a fresh instance is automatically granted admin privileges. This applies to both standard registration and OIDC-based sign-in. Subsequent registrations create regular users. Once an admin account exists, additional admins can be created or promoted via the admin panel or `POST /api/v1/admin/users`.

## Admin Panel

The admin panel is accessible at `/#/admin/users` and is visible only to accounts with admin privileges. It has three tabs:

### Users tab (`/#/admin/users`)

Create, edit, and delete user accounts. From this tab you can:

- Create new users with email, display name, password, and optional admin flag.
- Edit an existing user's display name, password, and admin status.
- Delete users.

### Storage tab (`/#/admin/storage`)

View and override the storage configuration without restarting or redeploying. The page shows the current database overrides (if any) alongside the active storage type.

- **No overrides configured** — Enlace is using the environment variable configuration. Saving a new configuration stores it as a DB override that takes precedence on the next restart.
- **Local storage** — set the upload directory path (`storage_local_path`).
- **S3 storage** — set the endpoint, bucket, region, access key, secret key, and optional path prefix. The access key and secret key fields are masked; leave them blank to keep the currently stored values unchanged.
- **Reset to environment defaults** — removes all DB overrides so Enlace reverts to environment variables on the next restart.

> **Note:** Storage configuration changes require a restart to take effect. See [Configuration — Storage](configuration.md#storage) for environment variable reference and encryption details.

### Email tab (`/#/admin/email`)


View and override the SMTP configuration. Changes take effect on the next restart. The page shows the current database overrides (if any).

- **No overrides configured** — Enlace is using the environment variable SMTP configuration (or email is disabled if `SMTP_HOST` is not set).
- **Configuring SMTP** — set the host, port, username, password, sender address, and TLS policy. The password field is masked; leave it blank to keep the currently stored value unchanged. Use the **Clear password** checkbox to explicitly remove a stored password.
- **Reset to environment defaults** — removes all DB overrides so Enlace reverts to environment variables on the next restart.

> **Note:** SMTP configuration changes require a restart to take effect. See [Configuration — SMTP](configuration.md#smtp-email-notifications) for environment variable reference and encryption details.


### Available tags

| Tag | Description |
|---|---|
| `latest` | Most recent build from `main` |
| `vX.Y.Z` | Specific release version (e.g. `v1.2.3`) |
| `vX.Y` | Latest patch for a minor version |

Images are published for `linux/amd64` and `linux/arm64`.

## Docker Compose

```bash
cp .env.sample .env   # edit values as needed
docker-compose up -d
```

The included `docker-compose.yml` builds the image locally from source. To use the pre-built GHCR image instead, replace `build: .` with `image: ghcr.io/amalgamated-tools/enlace:latest` in `docker-compose.yml`.

See [Configuration](configuration.md) for all available environment variables.

## Building a Docker Image Locally

```bash
make docker-build   # builds enlace:latest
make docker-run     # run the image locally
make docker-up      # start with docker-compose (detached)
make docker-down    # stop docker-compose
make docker-logs    # tail docker-compose logs
```

The `Dockerfile` uses a multi-stage build: Node 22 compiles the Svelte frontend, then Go embeds the compiled assets and produces a minimal final image.

## Binary (non-Docker) Deployment

You can run Enlace as a plain binary if you prefer not to use containers.

### 1. Build the binary

```bash
# Requires Go 1.22+ and Node 22 + pnpm
make build
```

This produces an `enlace` binary in the project root with the frontend embedded.

### 2. Run the binary

```bash
DATABASE_PATH=/var/lib/enlace/enlace.db \
DATA_DIR=/var/lib/enlace/data \
STORAGE_LOCAL_PATH=/var/lib/enlace/uploads \
BASE_URL=https://enlace.example.com \
./enlace
```

Set any additional [configuration variables](configuration.md) as environment variables (or in a `.env` file in the working directory).

### 3. Run as a systemd service

Create `/etc/systemd/system/enlace.service`:

```ini
[Unit]
Description=Enlace file sharing
After=network.target

[Service]
Type=simple
User=enlace
WorkingDirectory=/opt/enlace
EnvironmentFile=/opt/enlace/.env
ExecStart=/opt/enlace/enlace
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Then:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now enlace
```

## Reverse Proxy

Enlace listens on HTTP only. In production, front it with a reverse proxy that handles TLS termination and (optionally) compression.

Set `BASE_URL` to your public HTTPS URL, and add your proxy's IP range to `TRUSTED_PROXIES` so that real client IPs are used for rate limiting. See [Networking / Reverse Proxy](configuration.md#networking--reverse-proxy) for details.

### Caddy

```caddy
enlace.example.com {
    reverse_proxy localhost:8080
}
```

Caddy automatically provisions and renews TLS certificates. No extra configuration is needed for `TRUSTED_PROXIES` when Caddy runs on the same host (use `TRUSTED_PROXIES=127.0.0.1/32`).

### nginx

```nginx
server {
    listen 443 ssl;
    server_name enlace.example.com;

    ssl_certificate     /etc/ssl/certs/enlace.example.com.crt;
    ssl_certificate_key /etc/ssl/private/enlace.example.com.key;

    client_max_body_size 200M;  # must be at least as large as the admin-configured max file size (default 100 MB)

    location / {
        proxy_pass         http://127.0.0.1:8080;
        proxy_set_header   Host              $host;
        proxy_set_header   X-Real-IP         $remote_addr;
        proxy_set_header   X-Forwarded-For   $proxy_add_x_forwarded_for;
        proxy_set_header   X-Forwarded-Proto $scheme;
        proxy_read_timeout 120s;
    }
}
```

Set `TRUSTED_PROXIES=127.0.0.1/32` in your Enlace environment when running nginx on the same host.

### Traefik (Docker Compose)

```yaml
services:
  enlace:
    image: ghcr.io/amalgamated-tools/enlace:latest
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.enlace.rule=Host(`enlace.example.com`)"
      - "traefik.http.routers.enlace.entrypoints=websecure"
      - "traefik.http.routers.enlace.tls.certresolver=letsencrypt"
      - "traefik.http.services.enlace.loadbalancer.server.port=8080"
    environment:
      - BASE_URL=https://enlace.example.com
      - TRUSTED_PROXIES=172.16.0.0/12  # Docker bridge network
    volumes:
      - enlace-data:/app/data
      - enlace-uploads:/app/uploads
    restart: unless-stopped
```

## Health Check

The `/health` endpoint returns HTTP 200 with `{"status":"ok"}` and requires no authentication. Use it for load balancer health checks and container readiness probes:

```bash
curl https://enlace.example.com/health
# {"status":"ok"}
```

The included `docker-compose.yml` already configures a `healthcheck` using this endpoint.
