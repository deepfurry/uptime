# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- Initial repository and engineering foundation.
- Public bbolt storage backend implementing Fiber Uptime v0.2.0's Store contract.
- Schema v1 with database identity, strict decoding, transactional persistence,
  and behavioral tests for concurrency, corruption, cleanup, locking, and reopen.
- Separate `make race` verification and Go 1.27.x Ubuntu race CI job.
- Strict YAML v3 configuration with omission-aware defaults, active one-pass
  environment interpolation, typed normalization, and secret-safe validation.
- `uptime config check`, an offline-validating official example, and configuration/
  CLI regression tests, including generated TLS keypairs.
- `uptime serve` with bbolt, Fiber v3.5.0, Fiber Uptime v0.2.0 endpoint probing,
  built-in dashboard/API, liveness/readiness routes, and graceful signal shutdown.
- Single-use application lifecycle, explicit unsupported capability gates, and
  integration tests for UP/DOWN targets, persistence, cancellation, and failures.

### Changed

- Updated checkout/setup-go GitHub Actions to v7.
- Limited formatting to repository Go files so ignored dependency caches remain untouched.
- Extended the existing race target/job to include `internal/app`.

### Fixed

- Prevented newly registered instances with zero `LastSeenAt` from expiring during cleanup.
- Removed full sample-day scans from heartbeat and current-day query hot paths,
  retaining full integrity validation during rollup and raw-sample cleanup.
- Reject explicitly empty UI description/footer so Fiber Uptime cannot replace
  them with upstream defaults; omitted values retain DeepFurry defaults.
