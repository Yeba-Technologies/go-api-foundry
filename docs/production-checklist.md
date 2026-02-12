# Production Checklist

This checklist is for deploying this template safely in a real environment.

## Before go-live (template gaps)

This template provides good production *defaults* (timeouts, safer proxy handling, request limits, standardized errors, correlation IDs, metrics), but it does not ship with product-specific production requirements.

Confirm you have a plan for:

- Authentication & authorization (not included)
- Secrets management (do not store secrets in `.env` in production)
- Database backups + restore drills
- Migrations strategy (locking, rollback, and deploy coordination)
- Observability beyond basic metrics/logs (alerting, on-call ownership; tracing if needed)
- Runtime safeguards (resource limits, autoscaling policy, and safe rollouts)

## Required configuration

- Set `GIN_MODE=release`
- Set `APP_ENV=production` (or `prod`) to enable production defaults like HSTS (when effectively HTTPS)
- Set `SKIP_DOTENV=true` in container/prod (prefer real env vars / secret manager)

If this API is used from a browser (CORS):

- Set `CORS_ALLOWED_ORIGIN` to a comma-separated allowlist (or `*` only if you fully understand the risk)
- If `CORS_ALLOWED_ORIGIN` is not set, the router denies cross-origin requests by default

## Database and migrations

- Do not rely on `--auto-migrate` in production (the server refuses it when `APP_ENV` is production-like).
- Run migrations explicitly via the CLI:

```bash
make migrate
```

- Ensure DB transport security is correct for your environment:
  - `POSTGRES_SSLMODE` defaults to `require` if not set
  - If you use `APP_DATABASE_URL`, include `sslmode=` explicitly.

Data safety:

- Configure automated backups (and retention) for the production database
- Perform periodic restore drills into a non-production environment
- Decide on your RPO/RTO targets and ensure they match backup frequency and restore time

## Reverse proxy / load balancer

### Trusted proxies

Gin proxy trust is **disabled by default**.

- Configure `TRUSTED_PROXIES` to the CIDRs/IPs of your proxy/load balancer.
- Do not use `TRUSTED_PROXIES=*` in production.

Reason: `ClientIP()` should not be influenced by spoofed `X-Forwarded-For`.

### HTTPS and HSTS

HSTS is only set when the request is effectively HTTPS:

- Direct TLS to the app (`c.Request.TLS != nil`), OR
- TLS termination at the proxy with `X-Forwarded-Proto=https`

Controls:

- `HSTS_ENABLED=true|false` (overrides defaults)
- `HSTS_MAX_AGE` (seconds, default `31536000`)
- `HSTS_INCLUDE_SUBDOMAINS=true|false` (default `true`)

Proxy alignment:

- Ensure proxy/LB timeouts are >= your `REQUEST_TIMEOUT` budget (or explicitly design a smaller upstream timeout)
- Ensure `X-Forwarded-Proto` is set correctly when TLS is terminated upstream

## Request limits & timeouts

- Set `REQUEST_TIMEOUT` based on your SLOs (default `30s`).
  - This timeout is used both for request context deadlines and for `http.Server` read/write timeouts.

- Set `MAX_REQUEST_BODY_BYTES` to an appropriate limit for your API (default `1048576`).
  - Oversized requests return HTTP 413.

## Secrets and configuration

- Use a secret manager or your platform’s encrypted secret store for DB/Redis credentials and any API keys
- Avoid logging secrets/credentials; audit log fields if you add request/response logging
- Validate configuration at startup (env var types, required values, and safe defaults) before accepting traffic

## Rate limiting

- Configure `RATE_LIMIT_REQUESTS` and `RATE_LIMIT_WINDOW` for your expected traffic.
- For multi-instance deployments, configure Redis (`REDIS_HOST`, `REDIS_PORT`, optional `REDIS_PASSWORD`) so rate limiting is consistent across instances.

Client-visible behavior:

- `X-RateLimit-Limit` and `X-RateLimit-Window` are present on all responses.
- On 429 responses, `Retry-After` is set to integer seconds.

## Observability

- Metrics:
  - `GET /metrics` is enabled by default.
  - Disable with `METRICS_ENABLED=false`.
  - If you enable it, ensure it is protected appropriately in your network (do not expose it publicly without controls).

- Correlation IDs:
  - Requests/responses include `X-Correlation-ID`.
  - Ensure your ingress/proxy forwards it if you generate correlation IDs upstream.

Operational expectations:

- Route logs/metrics to a central system with retention appropriate for your org
- Add alerting on basic golden signals (latency, error rate, saturation, and availability)
- Decide whether you need request tracing (not included by default)

## Docker / runtime

- Production Docker image runs as non-root (UID/GID `65532:65532`).
- Keep `docker-compose.prod.yaml` aligned with your deployment platform, but avoid adding `--auto-migrate` to the production command.

Runtime safeguards (platform-specific, but important):

- Set CPU/memory limits and requests (or equivalent) to prevent noisy-neighbor failures
- Use health checks (and readiness checks if your platform supports them) to avoid routing traffic before dependencies are ready
- Prefer rolling deployments with quick rollback capability

## CI and safety checks

- Run at least:
  - `go vet ./...`
  - `go test ./...`
  - `go test -race ./...`

- If you modify dependencies, keep vendoring in sync:

```bash
make vendor
```

Recommended additions (if available in your CI platform):

- Dependency vulnerability scanning (Go modules)
- Container image scanning
- SAST/security linting appropriate for Go
