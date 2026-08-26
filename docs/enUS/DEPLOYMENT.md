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
# Build image
docker build -t herald-smtp .

# Run with env vars
docker run -d --name herald-smtp -p 8084:8084 \
  --health-cmd='curl -fsS http://localhost:8084/healthz || exit 1' \
  --health-interval=30s --health-timeout=3s \
  -e SMTP_HOST=smtp.example.com \
  -e SMTP_FROM=noreply@example.com \
  -e SMTP_USER=user \
  -e SMTP_PASSWORD=secret \
  herald-smtp
```

Optional: if you use `API_KEY` on herald-smtp, pass it and use the same value in Herald as `HERALD_SMTP_API_KEY`:

```bash
docker run -d --name herald-smtp -p 8084:8084 \
  -e API_KEY=your_shared_secret \
  -e SMTP_HOST=smtp.example.com \
  -e SMTP_FROM=noreply@example.com \
  -e SMTP_USER=user \
  -e SMTP_PASSWORD=secret \
  herald-smtp
```

### Docker Compose (example)

Minimal example for herald-smtp only:

```yaml
services:
  herald-smtp:
    image: herald-smtp:latest
    build: .
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
      # - IDEMPOTENCY_TTL_SECONDS=300
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

`SMTP_USE_TLS` and `SMTP_USE_STARTTLS` are mutually exclusive. For implicit TLS, set `SMTP_USE_TLS=true` and `SMTP_USE_STARTTLS=false`.
The effective HTTP write timeout is always at least `SMTP_TIMEOUT_SECONDS + 5` seconds so an SMTP operation can finish before the response deadline.

When `SMTP_HOST` or `SMTP_FROM` is missing, `POST /v1/send` returns `503` with `error_code: "provider_down"`.

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
- On `SIGINT` or `SIGTERM`, the server stops accepting new requests and waits up to 10 seconds for HTTP shutdown. Each SMTP operation is also bounded by `SMTP_TIMEOUT_SECONDS`.

## Replica and Idempotency Model

Idempotency reservations and cached results live in process memory. They are not shared across containers or hosts. If the same key reaches different replicas, each replica can perform its own SMTP send. Run a single replica when local idempotency is required, or provide a shared idempotency layer in the caller or gateway before scaling out.

## Integration with Herald

Herald calls herald-smtp over HTTP when the OTP channel is `email` and `HERALD_SMTP_API_URL` is set. Configure Herald with:

- **`HERALD_SMTP_API_URL`** – Base URL of herald-smtp (e.g. `http://herald-smtp:8084`).
- **`HERALD_SMTP_API_KEY`** (optional) – Same value as herald-smtp `API_KEY`; Herald will send it as `X-API-Key`.

When `HERALD_SMTP_API_URL` is set, Herald does not use built-in SMTP (no `SMTP_HOST` in Herald). All SMTP credentials live only in herald-smtp.

Herald should reuse one stable idempotency key for retries of the same logical send and must not reuse that key for different recipients or content.
