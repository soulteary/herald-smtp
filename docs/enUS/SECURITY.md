# herald-smtp Security Practices

This document describes security considerations and recommendations for herald-smtp.

## API Key

- When `API_KEY` is set, herald-smtp requires the `X-API-Key` header to match for **POST /v1/send**. Use a strong, unique value and keep it secret.
- Herald must be configured with the same value as `HERALD_SMTP_API_KEY` so that it sends the key on every request to herald-smtp.
- Do not log or expose the API key. Prefer environment variables or a secret manager over config files committed to source control.
- Ensure reverse proxies and access logs redact `X-API-Key`.

## Request Metadata

- Treat idempotency keys as non-secret correlation identifiers. A caller-supplied key can be returned and logged as `message_id`; never place passwords, access tokens, email addresses, or other sensitive data in it.
- Error responses intentionally avoid exposing unexpected SMTP host, network, or connection details. Use protected service logs for diagnosis.

## SMTP Credentials

- **SMTP_HOST**, **SMTP_USER**, **SMTP_PASSWORD**, and **SMTP_FROM** must never be hardcoded or committed to the repository.
- Keep `SMTP_SKIP_TLS_VERIFY=false` in production. Disabling certificate verification permits machine-in-the-middle attacks against the SMTP connection.
- Store them in environment variables or a secret manager (e.g. Kubernetes Secrets, HashiCorp Vault). Use `.env` only for local development and ensure `.env` is in `.gitignore`.
- Rotate SMTP passwords periodically and update herald-smtp configuration accordingly.

## Production Recommendations

- **Network**: Run herald-smtp in a private network. Only Herald (or your gateway) should call it; do not expose herald-smtp directly to the public internet unless behind HTTPS and strict access control.
- **HTTPS**: If herald-smtp is reachable over the internet or across untrusted networks, put it behind a reverse proxy (e.g. Traefik, nginx) with TLS. Herald should use `https://` for `HERALD_SMTP_API_URL` in that case.
- **Least privilege**: Run the process with a non-root user. The provided Docker image uses the dedicated `herald` user.
- **Resource limits**: Keep request-size, HTTP timeout, and idempotency-capacity limits enabled. Tune them for expected traffic instead of disabling them with very large values.
- **Replica model**: In-memory idempotency is local to one process. Multiple replicas require a shared idempotency layer if duplicate sends must be prevented across instances.
- **Logging**: Avoid logging request bodies or headers that may contain secrets. Structured logs (e.g. masked `to`, `message_id`, error codes) are sufficient for operations and troubleshooting.

## Summary

- Use `API_KEY` in production and keep it secret; configure Herald with `HERALD_SMTP_API_KEY` to match.
- Store SMTP credentials in env or a secret manager; never in code or committed config.
- Prefer private network and HTTPS in front of herald-smtp; do not expose it publicly without protection.
