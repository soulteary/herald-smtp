# Changelog

Notable changes to herald-smtp are recorded in this file. The project follows
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [1.2.9] - 2026-11-13

### Changed

- Require Go 1.27.4 (previously 1.27.3) in `go.mod` and build the container
  image with `golang:1.27.4-alpine3.24`. Go 1.27.4 is a security release and
  includes the net/http fix for GO-2026-6859.
- Update Fiber from v3.6.1 to v3.7.0.
- Update kit modules: cli-kit v1.9.3 to v1.9.4, health-kit/v4 v4.0.5 to v4.0.7,
  logger-kit/v3 v3.0.4 to v3.0.6, provider-kit v1.7.4 to v1.7.5 and
  version-kit/v4 v4.0.3 to v4.0.5.
- Refresh indirect modules: gofiber/schema v1.9.2 to v1.9.3, gofiber/utils/v2
  v2.6.3 to v2.6.4 and fasthttp v1.77.0 to v1.78.0.
- This release contains no changes to herald-smtp source code; the HTTP API,
  configuration and delivery behavior are the same as in v1.2.8.

## [1.2.8] - 2026-11-02

### Fixed

- Exit with status 1 when HTTP shutdown fails, including a deadline, instead of
  returning success after the existing `shutdown error` warning. Normal drain
  still exits 0. API/configuration, warning fields/level and stored records are
  unchanged; review supervisor restart policy and reconcile uncertain operations
  before retrying. Rollback restores the old failure exit status.
- Add a real subprocess regression that requires the shutdown diagnostic and
  exit status 1, with standard Go child coverage instrumentation.

### Documentation

- Give container stop-grace examples margin beyond the unchanged application
  shutdown budget so forced termination does not preempt failure reporting.

## [1.2.7] - 2026-11-02

### Tests

- Verify configured SMTP client initialization, conflicting TLS modes, retention
  of live/in-flight idempotency entries, cancellation responses and bare listen
  ports. Raise the CI statement coverage requirement from 95% to 98%.
- Linux race tests reach 98.3% total statement coverage; production behavior is
  unchanged.

## [1.2.6] - 2026-11-02

### Fixed

- Return a fixed error for malformed request bodies instead of exposing parser
  diagnostics in HTTP responses and logs. JSON field names and XML entities can
  contain sensitive request content. Existing logs are not rewritten; see the
  security guide for retention and credential rotation requirements.

### Changed

- Include the previously merged health-kit dependency alignment and verified kit
  maintenance upgrades; refresh required transitive dependencies and checksums.

## [1.2.5] - 2026-10-22

### Changed

- Require Go 1.27.3 and align build images and CI toolchains.
- Update compatible Go dependencies and published Kit versions; refresh module checksums.
- Preserve the project toolchain after quality report generation.


## [1.2.3] - 2026-09-21

### Changed

- Upgrade health-kit to v4.0.0, logger-kit to v3.0.0, and version-kit to v4.0.0, adopting each kit's new major module path.
- Serve `/healthz` through health-kit's `fiberadapter.SimpleHandler`, which replaces the Fiber handlers the root package no longer exports. The endpoint's status, headers, and response body are unchanged.
- Point the version-kit linker paths in the CI, release, and Docker builds at the v4 module path. A stale path is not a build error; it leaves the binary reporting `dev`.
- Drop the Redis, xxhash, and `go.uber.org/atomic` transitive dependencies, which health-kit v4 moved out of its root package into the `redisprobe` subpackage.
- Refresh the transitive gofiber/schema, gofiber/utils, and go-brrr requirements that the upgraded kits raise.

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

[Unreleased]: https://github.com/soulteary/herald-smtp/compare/v1.2.9...HEAD
[1.2.9]: https://github.com/soulteary/herald-smtp/compare/v1.2.8...v1.2.9
[1.2.8]: https://github.com/soulteary/herald-smtp/compare/v1.2.7...v1.2.8
[1.2.7]: https://github.com/soulteary/herald-smtp/compare/v1.2.6...v1.2.7
[1.2.6]: https://github.com/soulteary/herald-smtp/compare/v1.2.5...v1.2.6
[1.2.5]: https://github.com/soulteary/herald-smtp/compare/v1.2.4...v1.2.5
[1.2.3]: https://github.com/soulteary/herald-smtp/compare/v1.2.2...v1.2.3
[1.0.0]: https://github.com/soulteary/herald-smtp/compare/v0.6.0...v1.0.0
[0.6.0]: https://github.com/soulteary/herald-smtp/releases/tag/v0.6.0
