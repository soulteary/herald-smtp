# herald-smtp Deployment Guide

## Quick Start

### Binary

```bash
# Build
go build -o herald-smtp .

# Run (set SMTP env vars first)
./herald-smtp
```

### Docker

```bash
# Pull a fixed release
docker pull ghcr.io/soulteary/herald-smtp:v1.2.9

# Run with env vars
docker run -d --name herald-smtp -p 8084:8084 --stop-timeout=45 \
  --health-cmd='curl -fsS http://localhost:8084/healthz || exit 1' \
  --health-interval=30s --health-timeout=3s \
  -e SMTP_HOST=smtp.example.com \
  -e SMTP_FROM=noreply@example.com \
  -e SMTP_USER=user \
  -e SMTP_PASSWORD=secret \
  ghcr.io/soulteary/herald-smtp:v1.2.9
```

To build the current source instead, run `docker build -t herald-smtp:local .` and substitute `herald-smtp:local` for the image name. Use a fixed release tag in production rather than `latest`.

Optional: if you use `API_KEY` on herald-smtp, pass it and use the same value in Herald as `HERALD_SMTP_API_KEY`:

```bash
docker run -d --name herald-smtp -p 8084:8084 --stop-timeout=45 \
  -e API_KEY=your_shared_secret \
  -e SMTP_HOST=smtp.example.com \
  -e SMTP_FROM=noreply@example.com \
  -e SMTP_USER=user \
  -e SMTP_PASSWORD=secret \
  ghcr.io/soulteary/herald-smtp:v1.2.9
```

### Docker Compose (example)

Minimal example for herald-smtp only:

```yaml
services:
  herald-smtp:
    image: ghcr.io/soulteary/herald-smtp:v1.2.9
    stop_grace_period: 45s
    ports:
      - "8084:8084"
    environment:
      - PORT=:8084
      - SMTP_HOST=${SMTP_HOST}
      - SMTP_FROM=${SMTP_FROM}
      - SMTP_USER=${SMTP_USER}
      - SMTP_PASSWORD=${SMTP_PASSWORD}
      # Optional:
      # - API_KEY=${API_KEY}
      # - LOG_LEVEL=info
      # - SMTP_MAX_CONCURRENT_SENDS=16
      # - IDEMPOTENCY_TTL_SECONDS=300
      # - SHUTDOWN_TIMEOUT_SECONDS=40
    healthcheck:
      test: ["CMD", "curl", "-fsS", "http://localhost:8084/healthz"]
      interval: 30s
      timeout: 3s
      retries: 3
```

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `PORT` | Listen port (with or without leading colon, e.g. `8084` or `:8084`) | `:8084` | No |
| `API_KEY` | If set, callers must send `X-API-Key` with this value | `` | No |
| `SMTP_HOST` | SMTP server host | `` | Yes (for send) |
| `SMTP_PORT` | SMTP server port | `587` | No |
| `SMTP_USER` | SMTP username | `` | No (if server allows anonymous) |
| `SMTP_PASSWORD` | SMTP password | `` | No |
| `SMTP_FROM` | Sender email address | `` | Yes (for send) |
| `SMTP_FROM_NAME` | Optional sender display name | `` | No |
| `SMTP_USE_TLS` | Use implicit TLS (typically port 465) | `false` | No |
| `SMTP_USE_STARTTLS` | Use STARTTLS | `true` | No |
| `SMTP_SKIP_TLS_VERIFY` | Skip certificate verification; development only | `false` | No |
| `SMTP_TIMEOUT_SECONDS` | End-to-end SMTP send timeout | `30` | No |
| `SMTP_MAX_CONCURRENT_SENDS` | Maximum simultaneous SMTP sends | `16` | No |
| `LOG_LEVEL` | Log level: trace, debug, info, warn, error | `info` | No |
| `IDEMPOTENCY_TTL_SECONDS` | Idempotency cache TTL in seconds | `300` | No |
| `IDEMPOTENCY_MAX_ENTRIES` | Maximum in-flight and cached idempotency keys | `10000` | No |
| `HTTP_BODY_LIMIT_BYTES` | Maximum HTTP request body size | `65536` | No |
| `HTTP_READ_TIMEOUT_SECONDS` | HTTP request read timeout | `10` | No |
| `HTTP_WRITE_TIMEOUT_SECONDS` | HTTP response write timeout | `40` | No |
| `HTTP_IDLE_TIMEOUT_SECONDS` | HTTP keep-alive idle timeout | `60` | No |
| `SHUTDOWN_TIMEOUT_SECONDS` | Maximum graceful shutdown wait; never less than the SMTP timeout plus 5 seconds | `40` | No |

`SMTP_USE_TLS` and `SMTP_USE_STARTTLS` are mutually exclusive. For implicit TLS, set `SMTP_USE_TLS=true` and `SMTP_USE_STARTTLS=false`.
The effective HTTP write timeout is always at least `SMTP_TIMEOUT_SECONDS + 5` seconds so an SMTP operation can finish before the response deadline.

When `SMTP_HOST` or `SMTP_FROM` is missing, `POST /v1/send` returns `503` with `error_code: "provider_down"`.

`SMTP_MAX_CONCURRENT_SENDS` is a non-blocking capacity limit. When all slots are occupied, a new send returns `429 rate_limited` immediately rather than waiting in an unbounded queue. Tune it against the SMTP provider's connection limits and the container's CPU, memory, and file-descriptor budget.

### SMTP Transport Modes

| Mode | Typical port | `SMTP_USE_TLS` | `SMTP_USE_STARTTLS` | Notes |
|------|-------------:|:--------------:|:-------------------:|-------|
| STARTTLS | 587 | `false` | `true` | Default and recommended when the SMTP service supports STARTTLS. |
| Implicit TLS | 465 | `true` | `false` | TLS begins immediately after connecting. |
| Plain SMTP | 25 or custom | `false` | `false` | Use only on a trusted private network. |

Keep `SMTP_SKIP_TLS_VERIFY=false` outside isolated development environments.

## Health and Shutdown

- `/healthz` is a **liveness** endpoint. It confirms that the HTTP process can respond, but it does not check SMTP configuration, authentication, connectivity, or delivery.
- `/readyz` is a **readiness** endpoint. It returns success only when the SMTP client was initialized from valid local configuration; it does not test SMTP network connectivity or delivery.
- On `SIGINT` or `SIGTERM`, the server stops accepting new requests and waits up to `SHUTDOWN_TIMEOUT_SECONDS` for HTTP shutdown. The effective timeout is never shorter than `SMTP_TIMEOUT_SECONDS + 5` seconds, allowing in-flight sends to finish.
- Give the container runtime or process supervisor a grace period longer than the effective application shutdown budget, with margin for scheduling and logging. The examples use 45 seconds for the default 40-second application budget. Increase `--stop-timeout` or `stop_grace_period` when the application budget increases. Matching the two deadlines can let `SIGKILL` preempt the warning and exit status 1; forced termination is not successful drain.

## Replica and Idempotency Model

Idempotency reservations and cached results live in process memory. They are not shared across containers or hosts. If the same key reaches different replicas, each replica can perform its own SMTP send. Run a single replica when local idempotency is required, or provide a shared idempotency layer in the caller or gateway before scaling out.

## Integration with Herald

Herald calls herald-smtp over HTTP when the OTP channel is `email` and `HERALD_SMTP_API_URL` is set. Configure Herald with:

- **`HERALD_SMTP_API_URL`** – Base URL of herald-smtp (e.g. `http://herald-smtp:8084`).
- **`HERALD_SMTP_API_KEY`** (optional) – Same value as herald-smtp `API_KEY`; Herald will send it as `X-API-Key`.

When `HERALD_SMTP_API_URL` is set, Herald does not use built-in SMTP (no `SMTP_HOST` in Herald). All SMTP credentials live only in herald-smtp.

Herald should reuse one stable idempotency key for retries of the same logical send and must not reuse that key for different recipients or content.

## Shutdown exit status and upgrades

From v1.2.8, successful signal-driven shutdown exits 0; any error from HTTP shutdown, including its deadline, logs the existing `shutdown error` warning and exits 1. The message, log fields/level, signal handling and shutdown budget are unchanged. This corrects failure reporting, not authentication, delivery or replay protection. A supervisor using restart-on-failure may now restart after a failed shutdown; check its policy and grace period. An unfinished operation may already have committed or been accepted upstream: reconcile uncertain results before retrying. No API, configuration, record format, migration or new mixed-version restriction is introduced; mixed versions differ in failure exit status. Rolling replacement needs no special order from v1.2.7. Rollback restores the misleading zero exit on shutdown failure and must retain warning-based monitoring.
