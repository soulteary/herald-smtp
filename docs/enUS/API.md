# herald-smtp API Documentation

herald-smtp implements the HTTP send contract used by Herald's external provider for the email channel. Request/response types align with [provider-kit](https://github.com/soulteary/provider-kit) `HTTPSendRequest` / `HTTPSendResponse`.

## Base URL

```
http://localhost:8084
```

## Authentication

When `API_KEY` is set, Herald (or any caller) must send the same value in the `X-API-Key` header. If the header is missing or does not match, the server returns `401 Unauthorized` with `error_code: "unauthorized"`.

If `API_KEY` is not set, no authentication is required for `/v1/send`.

## Endpoints

### Health Check

**GET /healthz**

Check service health. Implemented via [health-kit](https://github.com/soulteary/health-kit).

**Response (Success):**
```json
{
  "status": "healthy",
  "service": "herald-smtp"
}
```

### Send (SMTP Email)

**POST /v1/send**

Send an email via SMTP. Called by Herald when channel is `email` and `HERALD_SMTP_API_URL` is set.

**Headers:**
- `X-API-Key` (optional): Required when herald-smtp `API_KEY` is set; must match.
- `Idempotency-Key` (optional): Used for idempotent sends; can also be set in the request body as `idempotency_key`.
- `Content-Type`: `application/json`

**Request body (HTTPSendRequest):**

| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `channel` | string | No | Typically `"email"` when sent by Herald. |
| `to` | string | Yes | Recipient email address. |
| `subject` | string | No | Email subject. Defaults to "Verification code" if empty. |
| `body` | string | No | Email body. If empty, see content resolution below. |
| `idempotency_key` | string | No | Idempotency key; same key within TTL returns cached result. |
| `template` | string | No | Optional; not used for content in current implementation. |
| `params` | object | No | If `body` is empty and `params.code` exists, content becomes "Your verification code is: " + params.code. |
| `locale` | string | No | Optional. |

**Content resolution (in order):**
1. If `body` is non-empty, use `body`.
2. Else if `params.code` exists, use "Your verification code is: " + params.code.
3. Else use default: "You have a verification message. Please check your code."

**Response (Success) – HTTP 200:**
```json
{
  "ok": true,
  "message_id": "uuid-or-challenge-id",
  "provider": "smtp"
}
```

**Response (Failure):**
```json
{
  "ok": false,
  "error_code": "error_code",
  "error_message": "human-readable message"
}
```

**Error codes and HTTP status:**

| error_code | HTTP status | Description |
|------------|-------------|-------------|
| `unauthorized` | 401 | `API_KEY` is set but `X-API-Key` is missing or invalid. |
| `invalid_request` | 400 | Request body parse error (invalid JSON). |
| `invalid_destination` | 400 | `to` is missing or empty. |
| `provider_down` | 503 | SMTP not configured (SMTP_HOST / SMTP_FROM not set). |
| `send_failed` | 500 | SMTP send error (e.g. connection, auth, or server error). |

## Idempotency

- Send requests support idempotency via `Idempotency-Key` header or body field `idempotency_key`.
- Within the configured TTL (`IDEMPOTENCY_TTL_SECONDS`, default 300), a repeated request with the same key returns the cached response (same `ok`, `message_id`, `provider`) without sending again.
- Cache is in-memory; key expires after TTL.
