# Go API Foundry

This is a Go backend API starter template: Gin + PostgreSQL (GORM) + optional Redis, with sane production defaults (timeouts, safer proxy handling, body size limits), standardized errors, correlation IDs, and opt-out Prometheus metrics.

- Developer docs: [docs/developer-guide.md](docs/developer-guide.md)
- Design notes: [docs/DESIGN_PATTERNS.md](docs/DESIGN_PATTERNS.md)
- Production checklist: [docs/production-checklist.md](docs/production-checklist.md)
- Reference implementation (for onboarding): [domain/waitlist/](domain/waitlist/) (`/v1/waitlist`)

## License

Apache-2.0. See [LICENCE](LICENCE) and [NOTICE](NOTICE).

## Quick Start

### 1) Configure environment

Copy and edit the sample env:

```bash
cp .env.example .env
```

At minimum, set database + app port values.

### 2) Run locally (hot reload)

```bash
make dev
```

With auto-migrate (development only):

```bash
make dev-migrate
```

### 3) Run locally (no hot reload)

```bash
make run
```

With auto-migrate (development only):

```bash
make run-with-migrate
```

### 4) Run with Docker Compose

Development (includes extra infra containers like LocalStack + RabbitMQ):

```bash
docker-compose -f docker-compose.dev.yaml up --build
```

Production:

```bash
docker-compose -f docker-compose.prod.yaml up --build -d
```

Server URL: `http://localhost:${APP_PORT:-8080}`

## Health, Metrics, and Headers

- `GET /health` health check
- `GET /metrics` Prometheus metrics (enabled by default; set `METRICS_ENABLED=false` to disable)
- Correlation ID: request/response header `X-Correlation-ID`

Rate limiting adds headers to all responses:

```http
X-RateLimit-Limit: 100
X-RateLimit-Window: 1m0s
```

On 429 responses it also includes RFC-compliant retry information:

```http
Retry-After: 60
```

## Migrations

This template supports migrations via:

- Versioned SQL migrations (recommended): stored in `./migrations/` and applied via the CLI.
- Server flag (development only): `--auto-migrate` (GORM AutoMigrate; convenient but not versioned). This is blocked when `APP_ENV` is production-like.

```bash
make migrate
```

By default the CLI reads migrations from `./migrations`. Override with `MIGRATIONS_DIR`.

## Tracing (OpenTelemetry)

Tracing is opt-in.

Environment variables:

- `OTEL_TRACES_ENABLED=true`
- `OTEL_SERVICE_NAME=go-api-foundry` (optional)
- `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318` (OTLP/HTTP)

### Local backend (Jaeger)

For local development, Jaeger is the simplest “works out of the box” backend: you get a UI and it accepts OTLP/HTTP.

Run Jaeger:

```bash
docker-compose -f docker-compose.tracing.yaml up -d
```

Then enable tracing and point the exporter at it:

```bash
export OTEL_TRACES_ENABLED=true
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_SERVICE_NAME=go-api-foundry
```

Jaeger UI: http://localhost:16686

### Verify traces end-to-end

1) Start Jaeger (see above).
2) Start the API (e.g. `make dev` or `make run`).
3) Generate traffic:

```bash
curl -sS http://localhost:${APP_PORT:-8080}/health >/dev/null
curl -sS http://localhost:${APP_PORT:-8080}/v1/waitlist >/dev/null || true
```

4) Open Jaeger UI, select service `go-api-foundry`, and search for recent traces.

## Configuration (high-signal)

Common env vars (see [.env.example](.env.example) for the full list):

- `APP_PORT` (default `8080`)
- `APP_ENV` (used for defaults like HSTS)
- `GIN_MODE` (`debug|release|test`)
- `SKIP_DOTENV=true` to skip loading `.env` (useful in container/prod)

HTTP/router safety knobs:

- `REQUEST_TIMEOUT` (default `30s`): request timeout budget (also used for server read/write timeouts)
- `MAX_REQUEST_BODY_BYTES` (default `1048576`): request payload limit
- `TRUSTED_PROXIES` (default: disabled): comma-separated list of CIDRs/addresses, or `*` to trust all (dev only)
- `METRICS_ENABLED` (default `true`)

HSTS:

- Default: enabled when `APP_ENV=production|prod` and request is effectively HTTPS
- `HSTS_ENABLED=true|false` overrides
- `HSTS_MAX_AGE` (seconds, default `31536000`)
- `HSTS_INCLUDE_SUBDOMAINS=true|false` (default `true`)

## Acknowledgements

AI was utilized as a critical friend, seeking feedback on performance metrics and critical reviews of my implementation to identify potential bottlenecks. The use of these models in no way completely replaces the author's imagination and creativity in crafting this solution; rather, they enhanced the author's ability to proactively spot issues and co-create responsibly. Below are the generative AI models consulted during the development of this solution and the prompt used:
- GPT-5.2 
- Claude Opus 4.5

## AI Prompt Used

```
Critically review this Go starter template to determine whether it is suitable for a real-world production application.
Evaluate the following aspects in depth:
- Architectural philosophy and design principles
- Code organization, readability, and idiomatic Go practices
- Scalability (codebase growth, team scaling, and system load)
- Maintainability and long-term evolution of the project
- Testing strategy and testability
- Error handling and resilience patterns
- Configuration and environment management
- Observability (logging, metrics, tracing)
- Security considerations and potential risks
- Dependency and package management
- Performance and concurrency design
- Deployment and production-readiness (CI/CD, containerization, cloud readiness)
- 12-factor App principles

Also identify:
- Strengths of the template
- Weaknesses and architectural risks
- Missing production-critical components
- Areas that may become problematic as the project scales
- Concrete improvements and best-practice recommendations
- Provide a clear, structured, and brutally honest assessment with actionable suggestions
```


