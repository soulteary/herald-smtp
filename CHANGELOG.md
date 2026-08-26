# Changelog

Notable changes to herald-smtp are recorded in this file. The project follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.0.0] - 2026-08-26

Version 1 establishes the documented HTTP, configuration, and operational behavior as the stable compatibility baseline.

### Added

- Add `/readyz` to report whether the SMTP client initialized from valid local configuration.
- Add `SMTP_MAX_CONCURRENT_SENDS` with a default of 16 and reject excess sends with `429 rate_limited`.
- Add `SHUTDOWN_TIMEOUT_SECONDS` with an effective minimum of `SMTP_TIMEOUT_SECONDS + 5` seconds.
- Add loopback SMTP protocol integration tests for successful delivery and recipient rejection.

### Changed

- Upgrade the shared health, logger, and version kits to their stable v2 module releases.
- Refresh direct and transitive dependencies.
- Return the provider JSON error envelope for errors raised through Fiber's handler stack.
- Align Docker and Compose stop grace examples with the application's effective shutdown timeout.
- Complete the bilingual configuration, readiness, concurrency, error, and deployment documentation.

### Fixed

- Correct version-kit v2 linker paths in CI and release builds.
- Allow in-flight SMTP sends enough time to finish during graceful shutdown.

### CI

- Remove the external Codecov upload while retaining the HTML coverage artifact and console summary.
- Pin golangci-lint and govulncheck versions.
- Pin all GitHub Actions to immutable commit SHAs.
- Exercise the production SMTP client against a real in-process SMTP conversation.

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

[Unreleased]: https://github.com/soulteary/herald-smtp/compare/v1.0.0...HEAD
[1.0.0]: https://github.com/soulteary/herald-smtp/compare/v0.6.0...v1.0.0
[0.6.0]: https://github.com/soulteary/herald-smtp/releases/tag/v0.6.0
