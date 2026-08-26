# Changelog

Notable changes to herald-smtp are recorded in this file. The project follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Changed

- Upgrade the shared health, logger, and version kits to their stable v2 module releases.
- Refresh direct and transitive dependencies.

## [0.6.0] - 2026-08-25

### Changed

- Migrate the HTTP server to Fiber v3.
- Integrate provider-kit v1.5.0 SMTP validation and transport hardening.

### Fixed

- Serialize concurrent requests that use the same idempotency key.
- Bound HTTP request resources and the in-memory idempotency store.
- Mask recipient addresses in logs and preserve structured provider error reasons.
- Correct release checksum generation.

### Documentation

- Clarify authentication, TLS modes, health checks, idempotency scope, and multi-replica limitations.

[Unreleased]: https://github.com/soulteary/herald-smtp/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/soulteary/herald-smtp/releases/tag/v0.6.0
