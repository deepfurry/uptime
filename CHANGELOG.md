# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added

- Initial repository and engineering foundation.
- Public bbolt storage backend implementing Fiber Uptime v0.2.0's Store contract.
- Schema v1 with database identity, strict decoding, transactional persistence,
  and behavioral tests for concurrency, corruption, cleanup, locking, and reopen.
- Separate `make race` verification and Go 1.27.x Ubuntu bbolt race CI job.

### Changed

- Updated checkout/setup-go GitHub Actions to v7.
- Limited formatting to repository Go files so ignored dependency caches remain untouched.
