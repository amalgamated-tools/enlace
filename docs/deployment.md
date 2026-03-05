# Deployment

## Quick Start (Docker)

```bash
docker run -d \
  -p 8080:8080 \
  -v enlace-db:/app/data \
  -v enlace-uploads:/app/uploads \
  enlace:latest
```

Open <http://localhost:8080> and register your first user.

> **First admin bootstrap:** All registered users start without admin privileges. After registering, grant admin access to your account by running the SQL command below against the SQLite database (replace the email address with your own):
>
> ```bash
> # Docker volume
> docker exec -it <container_name> sqlite3 /app/data/enlace.db \
>   "UPDATE users SET is_admin = 1 WHERE email = 'you@example.com';"
>
> # Local / binary
> sqlite3 ./enlace.db "UPDATE users SET is_admin = 1 WHERE email = 'you@example.com';"
> ```
>
> Once at least one admin account exists, additional admins can be created or promoted via the admin panel or `POST /api/v1/admin/users`.

## Docker Compose

```bash
cp .env.sample .env   # edit values as needed
docker-compose up -d
```

See [Configuration](configuration.md) for all available environment variables.

## Building a Docker Image

```bash
make docker-build   # builds enlace:latest
make docker-run     # run the image locally
make docker-up      # start with docker-compose (detached)
make docker-down    # stop docker-compose
make docker-logs    # tail docker-compose logs
```

The `Dockerfile` uses a multi-stage build: Node 22 compiles the Svelte frontend, then Go embeds the compiled assets and produces a minimal final image.
