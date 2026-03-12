# Enlace Documentation

> This documentation is also published at **<https://amalgamated-tools.github.io/enlace/docs/>** via GitHub Pages and is updated automatically on every push to `main`.

| Document | Description |
|---|---|
| [Configuration](configuration.md) | Environment variables, CLI flags, reverse proxy setup |
| [API Reference](api.md) | Full REST API documentation with request/response examples |
| [Deployment](deployment.md) | Docker, Docker Compose, and production builds |
| [Development](development.md) | Local dev environment, make targets, dev services |
| [OIDC / SSO](oidc.md) | OpenID Connect setup with provider-specific guides |
| [Architecture](architecture.md) | Technical architecture and design overview |

## Auto-generated API Specs

The `docs.go`, `swagger.json`, and `swagger.yaml` files in this directory are auto-generated
by [swag](https://github.com/swaggo/swag). Do not edit them manually. Run `make swagger`
to regenerate.
