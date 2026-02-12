# Developer Guidelines

This repository is a Go API starter template built around:

- Gin for HTTP routing
- GORM + PostgreSQL for persistence
- Optional Redis for cache + distributed rate limiting
- Structured errors + correlation IDs
- Optional Prometheus metrics at `GET /metrics`

## Getting Started

### Prerequisites

- Go 1.25+
- (Optional) Air for hot reload (already baked into the dev Docker stage)
- Docker + Docker Compose (recommended for running Postgres/Redis locally)

### Configure environment

```bash
cp .env.example .env
```

## Common Commands

```bash
make dev              # hot reload via Air
make dev-migrate      # hot reload + auto-migrate (development only)

make run              # run server
make run-with-migrate # run server + auto-migrate (development only)

make migrate          # run migrations explicitly via CLI

make test             # go test ./...
make lint             # go vet ./...
make format           # go fmt ./...
make vendor           # go mod vendor
```

## Running

### Local (recommended for app code work)

```bash
make dev
```

### Docker Compose

- Development compose: [docker-compose.dev.yaml](../docker-compose.dev.yaml)
  - Includes `api`, `postgres`, `redis`, plus extra infra containers (LocalStack + RabbitMQ) for local experimentation.
  - The application template does not require LocalStack/RabbitMQ by default; remove them if you don’t use them.

```bash
docker-compose -f docker-compose.dev.yaml up --build
```

- Production compose: [docker-compose.prod.yaml](../docker-compose.prod.yaml)

```bash
docker-compose -f docker-compose.prod.yaml up --build -d
```

Server URL: `http://localhost:${APP_PORT:-8080}`

## Migrations

There are two supported migration flows:

- Explicit, versioned SQL migrations (recommended):

```bash
make migrate
```

- Convenience flag (development only): pass `--auto-migrate` to the server (GORM AutoMigrate).

This flag is gated by `APP_ENV` and will error in production-like environments.

### Migration directory

By default the CLI reads migrations from `./migrations`. Override with:

- `MIGRATIONS_DIR=/path/to/migrations`

## Tracing (OpenTelemetry)

Tracing is opt-in and uses OTLP/HTTP.

Enable:

- `OTEL_TRACES_ENABLED=true`
- `OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318`

Optional:

- `OTEL_SERVICE_NAME=go-api-foundry`

### Local backend: Jaeger

Jaeger is a good default local tracing backend because it provides a UI and accepts OTLP.

Option A: run the provided compose file:

```bash
docker-compose -f docker-compose.tracing.yaml up -d
```

Option B: add this snippet to an existing compose:

```yaml
  jaeger:
    image: jaegertracing/jaeger:2.15.1
    environment:
      - COLLECTOR_OTLP_ENABLED=true
    ports:
      - "16686:16686" # Jaeger UI
      - "4318:4318"   # OTLP/HTTP
      - "4317:4317"   # OTLP/gRPC
```

Jaeger UI: http://localhost:16686

### Verify traces end-to-end

1) Run Jaeger (above).
2) Enable tracing:

```bash
export OTEL_TRACES_ENABLED=true
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
export OTEL_SERVICE_NAME=go-api-foundry
```

3) Start the API.
4) Make a request (any route works):

```bash
curl -sS http://localhost:${APP_PORT:-8080}/health >/dev/null
```

5) Open Jaeger UI and search for service `go-api-foundry`.

## HTTP Server & Router Behavior

### Timeouts

- `REQUEST_TIMEOUT` (default `30s`) controls the request timeout budget.
- The template enforces timeouts using `http.Server` read/write timeouts plus per-request context deadlines.

### Trusted proxies (Client IP)

Gin’s proxy behavior is locked down by default.

- Default: trusted proxies disabled (prevents spoofed `X-Forwarded-For` from affecting `ClientIP()`)
- Configure via `TRUSTED_PROXIES`:
  - empty/unset: disabled
  - `*`: trust all (dev escape hatch)
  - comma-separated list of CIDRs/addresses for real deployments

### Request body size limit

- `MAX_REQUEST_BODY_BYTES` (default `1048576` = 1 MiB)
- Requests exceeding the limit return HTTP 413.

### Security headers

The router sets baseline headers on all responses:

- `X-Content-Type-Options: nosniff`
- `X-Frame-Options: DENY`
- `Referrer-Policy: no-referrer`

### HSTS

HSTS is only set when the request is effectively HTTPS (direct TLS or `X-Forwarded-Proto=https`).

- Default: enabled when `APP_ENV=production|prod`
- Override with `HSTS_ENABLED=true|false`
- Tune with:
  - `HSTS_MAX_AGE` (seconds, default `31536000`)
  - `HSTS_INCLUDE_SUBDOMAINS=true|false` (default `true`)

## Observability

### Correlation IDs

- Request header: `X-Correlation-ID` (optional)
- Response header: `X-Correlation-ID` (always present)
- All request logs include the correlation ID.

### Metrics

- `GET /metrics` exposes Prometheus metrics when enabled.
- Control via `METRICS_ENABLED`:
  - unset/empty: enabled
  - `false`: disabled

## Rate Limiting

Rate limiting is applied per client IP.

- Single instance: in-memory limiter keyed per client
- Multi-instance: Redis-backed limiter (enabled when Redis is configured)

Configuration:

- `RATE_LIMIT_REQUESTS` (default from constants)
- `RATE_LIMIT_WINDOW` (duration, e.g. `30s`, `1m`, `5m`)

Headers:

- Always:
  - `X-RateLimit-Limit`
  - `X-RateLimit-Window`
- On 429:
  - `Retry-After` is integer seconds

## Errors

Guideline: return typed errors from repositories/services and let controllers translate them into HTTP responses.

- Prefer `pkg/errors.AppError` constructors (e.g., `NewNotFoundError`, `NewInvalidRequestError`).
- Avoid returning raw internal errors directly to clients.
- `GetHumanReadableMessage` intentionally returns a generic message for non-`AppError` inputs.

## Adding a New Domain

You can scaffold a domain skeleton:

```bash
make generate-domain
```

Then:

1. Create a model in `internal/models/`
2. Register it in `internal/models/main.go`
3. Implement repository/service/controller in `domain/<name>/`
4. Mount the controller in `domain/main.go`

### Reference implementation: Waitlist

The [domain/waitlist/](../domain/waitlist/) domain is the canonical onboarding example.

It shows the intended structure and conventions for:

- DTOs with Gin binding validation (create vs update semantics)
- Controller wiring with the router service and consistent error responses
- Service-layer validation and typed errors
- Repository patterns with GORM and error mapping
- Unit tests (service) and integration tests (HTTP)

## Testing

Unit tests:

```bash
make test
```

Integration tests (when supported by the repo):

```bash
RUN_INTEGRATION_TESTS=true go test ./integration/... -v
```

## Dependency Management

This repo tracks `vendor/modules.txt` for dependency verification.

- After changing `go.mod`/`go.sum`, run:

```bash
make vendor
```

- CI verifies that `vendor/modules.txt` is in sync. If you forget to run `make vendor` after dependency changes, CI will fail.
