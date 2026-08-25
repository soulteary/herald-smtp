# herald-smtp Troubleshooting Guide

This guide helps you diagnose and resolve common issues with herald-smtp.

## Table of Contents

- [Email Not Received](#email-not-received)
- [503 provider_down](#503-provider_down)
- [401 Unauthorized](#401-unauthorized)
- [invalid_destination](#invalid_destination)
- [send_failed](#send_failed)
- [HTTP Status Quick Reference](#http-status-quick-reference)
- [Idempotency and Logs](#idempotency-and-logs)

## HTTP Status Quick Reference

| Status | Meaning | First check |
|-------:|---------|-------------|
| 400 | Invalid JSON, missing destination, invalid address, or invalid header value | Request body and recipient address |
| 401 | Missing or mismatched API key | `API_KEY` and `X-API-Key` |
| 409 | Same idempotency key used for different request content | Caller key generation and reuse |
| 413 | Request exceeds `HTTP_BODY_LIMIT_BYTES` | Body size and configured limit |
| 429 | Provider limit or idempotency store capacity | Provider quota and `IDEMPOTENCY_MAX_ENTRIES` |
| 503 | SMTP is not configured or cannot be initialized | Startup logs, `SMTP_HOST`, `SMTP_FROM`, TLS mode |
| 504 | Send or idempotency wait exceeded its deadline | SMTP latency and timeout settings |
| 500 | SMTP send failed or an unexpected internal failure occurred | Server logs; the HTTP message is intentionally generic |

`GET /healthz` is a liveness check only. A 200 response does not rule out SMTP configuration, authentication, connectivity, or delivery problems.

## Email Not Received

### Symptoms

- Herald creates a challenge with channel `email` and gets a successful response from herald-smtp, but the user does not receive the email.

### Diagnostic Steps

1. **Check herald-smtp logs**  
   Look for `send_failed` or SMTP errors:
   ```bash
   # If running in Docker
   docker logs herald-smtp 2>&1 | grep -E "send_failed|send ok"
   ```
   - `send ok` with `message_id`: the SMTP server accepted the message. Final mailbox delivery can still fail later because of downstream rejection, filtering, spam classification, or an incorrect address.
   - `send_failed` with errmsg: note the error for the next steps.

2. **Verify SMTP configuration**  
   - Confirm `SMTP_HOST`, `SMTP_FROM` are set; if the server requires auth, set `SMTP_USER` and `SMTP_PASSWORD`.
   - Test STARTTLS with `openssl s_client -starttls smtp -connect SMTP_HOST:587` or implicit TLS with `openssl s_client -connect SMTP_HOST:465`. Replace `SMTP_HOST` with the real host.

3. **Check recipient and spam**  
   - Ensure `to` (destination) is a valid email address and not mistyped.
   - Check the recipient's spam/junk folder.

### Solutions

- **Wrong credentials**: Update `SMTP_HOST`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_FROM` and restart herald-smtp.
- **Wrong or invalid address**: Ensure Herald passes a valid email as `destination` for channel `email`.
- **SMTP server limits**: Check whether the SMTP provider has rate limits or blocking.

---

## 503 provider_down

### Symptoms

- `POST /v1/send` returns HTTP 503 with body: `"ok": false, "error_code": "provider_down", "error_message": "SMTP not configured"`.

### Cause

herald-smtp checks that `SMTP_HOST` and `SMTP_FROM` are non-empty. If either is missing, the SMTP client is not initialized and every send returns 503.

### Solutions

1. Set `SMTP_HOST` and `SMTP_FROM` (and auth if required) and restart the process (or container).
2. Confirm they are actually present in the runtime (e.g. no typo in env names, and in Docker/Kubernetes they are passed correctly).
3. Check logs at startup: if credentials are missing, herald-smtp logs a warning that `/v1/send` will return 503.

---

## 401 Unauthorized

### Symptoms

- `POST /v1/send` returns HTTP 401 with `error_code: "unauthorized"`, `error_message: "invalid or missing API key"`.

### Cause

herald-smtp has `API_KEY` set, but the request either does not send `X-API-Key` or sends a value that does not match.

### Solutions

1. **If you intend to use API Key**  
   - Set `API_KEY` on herald-smtp.  
   - Set `HERALD_SMTP_API_KEY` on Herald to the same value so Herald sends it in `X-API-Key`.  
   - Ensure no proxy or gateway strips the `X-API-Key` header.

2. **If you do not want API Key auth**  
   - Leave `API_KEY` unset on herald-smtp (and do not set `HERALD_SMTP_API_KEY` on Herald).

---

## invalid_destination

### Symptoms

- `POST /v1/send` returns HTTP 400 with `error_code: "invalid_destination"`, `error_message: "to is required"`.

### Cause

The request body has an empty or missing `to` field.

### Solutions

1. Ensure Herald sends a non-empty `to` (recipient email address) for channel `email`.
2. Check that the mapping from user identifier to email is correct and never yields an empty string.

---

## send_failed

### Symptoms

- `POST /v1/send` returns HTTP 500 with `error_code: "send_failed"` and a generic external message.

### Cause

The SMTP send failed because of connection, authentication, TLS, or server rejection. Detailed transport errors are kept in server logs rather than returned to callers.

### Solutions

1. Verify `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_USE_STARTTLS` match your SMTP provider (e.g. port 587 with STARTTLS, or 465 with TLS).
2. Check network connectivity from herald-smtp to the SMTP server (firewall, DNS).
3. Confirm the SMTP server allows the sender address (`SMTP_FROM`) and that credentials are correct.
4. Check the server logs around `send_failed: SMTP error`; do not expect internal host or connection details in the HTTP response.

---

## Idempotency and Logs

### Idempotent hit (cached response)

`Idempotency-Key` (or body `idempotency_key`) is limited to 256 bytes. Concurrent requests with the same key and content share one SMTP send; followers wait for its successful result. Repeating that request within the configured TTL returns the cached response without sending again. Failed requests can be retried. Reusing a key for different request content returns `409 idempotency_conflict`.

If duplicates appear only after adding replicas, check request routing. Idempotency state is process-local; replicas do not share keys or cached results. Use one replica or add a shared idempotency layer before horizontal scaling.

### Log level

- **info**: You see `send ok`, `send_failed`, and 503/401 as above.
- **debug**: You also see `send idempotent hit`. Set `LOG_LEVEL=debug` to verify that repeated requests with the same idempotency key are being cached.

Do not use secrets or personal data as idempotency keys. A supplied key can be returned and logged as `message_id`.

### TTL

Idempotency cache TTL is controlled by `IDEMPOTENCY_TTL_SECONDS` (default 300). After TTL, the same key is treated as a new request and may trigger a new send.

If a new key returns `429 rate_limited`, the in-memory store has reached `IDEMPOTENCY_MAX_ENTRIES` (default 10000). Existing keys remain available. Reduce unique-key churn, wait for entries to expire, or raise the limit within the service's memory budget.
