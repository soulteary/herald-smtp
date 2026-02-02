# herald-smtp Troubleshooting Guide

This guide helps you diagnose and resolve common issues with herald-smtp.

## Table of Contents

- [Email Not Received](#email-not-received)
- [503 provider_down](#503-provider_down)
- [401 Unauthorized](#401-unauthorized)
- [invalid_destination](#invalid_destination)
- [send_failed](#send_failed)
- [Idempotency and Logs](#idempotency-and-logs)

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
   - `send ok` with `message_id`: herald-smtp successfully sent via SMTP; delivery issues may be on the SMTP server or recipient side (spam, wrong address).
   - `send_failed` with errmsg: note the error for the next steps.

2. **Verify SMTP configuration**  
   - Confirm `SMTP_HOST`, `SMTP_FROM` are set; if the server requires auth, set `SMTP_USER` and `SMTP_PASSWORD`.
   - Test SMTP connectivity (e.g. `telnet SMTP_HOST 587`) and STARTTLS if `SMTP_USE_STARTTLS=true`.

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

- `POST /v1/send` returns HTTP 500 with `error_code: "send_failed"`, `error_message` containing SMTP or network error details.

### Cause

The SMTP send failed: connection refused, auth failure, TLS error, or server rejection.

### Solutions

1. Verify `SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `SMTP_USE_STARTTLS` match your SMTP provider (e.g. port 587 with STARTTLS, or 465 with TLS).
2. Check network connectivity from herald-smtp to the SMTP server (firewall, DNS).
3. Confirm the SMTP server allows the sender address (`SMTP_FROM`) and that credentials are correct.

---

## Idempotency and Logs

### Idempotent hit (cached response)

When Herald (or any client) sends the same `Idempotency-Key` (or body `idempotency_key`) within the configured TTL, herald-smtp returns the cached response without sending again. This is expected.

### Log level

- **info**: You see `send ok`, `send_failed`, and 503/401 as above.
- **debug**: You also see `send idempotent hit`. Set `LOG_LEVEL=debug` to verify that repeated requests with the same idempotency key are being cached.

### TTL

Idempotency cache TTL is controlled by `IDEMPOTENCY_TTL_SECONDS` (default 300). After TTL, the same key is treated as a new request and may trigger a new send.
