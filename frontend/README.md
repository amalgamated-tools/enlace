# Enlace — Frontend

The Enlace web UI, built with [Svelte](https://svelte.dev/), TypeScript, and [Vite](https://vitejs.dev/).

## Recommended IDE Setup

[VS Code](https://code.visualstudio.com/) + the [Svelte extension](https://marketplace.visualstudio.com/items?itemName=svelte.svelte-vscode).

## Development

```bash
pnpm install
pnpm dev        # start Vite dev server at http://localhost:5173
```

API calls are proxied to the Go backend at `http://localhost:8080` (configured in `vite.config.ts`). Start the backend first with `make run-backend` from the repo root.

## Testing

```bash
pnpm test       # run Vitest unit tests
```

Tests live next to their source files under `src/**/__tests__/`.

## Building for Production

```bash
pnpm build      # outputs to dist/
```

The compiled `dist/` directory is embedded into the Go binary at build time via `go:embed`. Run `make build` from the repo root to produce the full application binary.

## Source Layout

```
src/
  lib/
    api/          # typed API client (auth, shares, files, OIDC) and shared utilities
                  # dateToRFC3339(date) converts HTML date inputs (YYYY-MM-DD) to RFC3339
    components/   # shared UI components (FileList, FileUploader, ShareCard, …)
    stores/       # Svelte stores for auth and toast notifications
  routes/         # page components (Login, Dashboard, Shares, ShareDetail, …)
  App.svelte      # root component with client-side router
  main.ts         # application entry point
  routes.ts       # route definitions
```
