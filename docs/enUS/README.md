# Documentation Index

Welcome to the herald-smtp documentation. herald-smtp is the SMTP email adapter for [Herald](https://github.com/soulteary/herald).

## Multi-language Documentation

- [English](README.md) | [中文](../zhCN/README.md)

## Document List

### Core Documents

- **[README.md](../../README.md)** - Project overview and quick start guide
- **[CHANGELOG.md](../../CHANGELOG.md)** - Release history and the v1 compatibility baseline

### Detailed Documents

- **[API.md](API.md)** - Complete API reference
  - Base URL and authentication
  - POST /v1/send field behavior and runnable request example
  - GET /healthz liveness semantics
  - GET /readyz local SMTP readiness semantics
  - Error codes and HTTP status codes
  - Idempotency, SMTP concurrency, and `message_id` behavior

- **[DEPLOYMENT.md](DEPLOYMENT.md)** - Deployment guide
  - Binary and Docker run
  - Complete configuration and SMTP TLS mode matrix
  - Health checks, shutdown, and replica limits
  - Integration with Herald (HERALD_SMTP_API_URL)

- **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Troubleshooting guide
  - Email not received
  - 503 provider_down
  - 401 unauthorized
  - HTTP status quick reference
  - invalid_destination, send_failed, idempotency, and logs

- **[SECURITY.md](SECURITY.md)** - Security practices
  - API Key usage
  - SMTP credential management
  - Request metadata and log exposure
  - Production recommendations

## Quick Navigation

### Getting Started

1. Read [README.md](../../README.md) to understand the project
2. Check the [Quick Start](../../README.md#quick-start) section
3. Refer to [DEPLOYMENT.md](DEPLOYMENT.md) for configuration and Herald integration

### Developers

1. Check [API.md](API.md) for the send contract and error codes
2. Review [DEPLOYMENT.md](DEPLOYMENT.md) for deployment options

### Operations

1. Read [DEPLOYMENT.md](DEPLOYMENT.md) for deployment and Herald side config
2. Refer to [SECURITY.md](SECURITY.md) for production practices
3. Troubleshoot issues: [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
