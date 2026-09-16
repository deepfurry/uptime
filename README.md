# Uptime

A tiny, self-hosted uptime service being built around Fiber.

The planned experience: one binary, one YAML file, and zero external
infrastructure required by default.

> [!NOTE]
> Uptime is under active development toward v0.1.0. The current P0 phase provides
> repository engineering foundations and a help/version CLI only. The monitoring
> runtime is not implemented yet.

## Overview

DeepFurry Uptime aims to make HTTP service availability and persistent history
easy to operate in a small standalone product. Fiber Contrib Uptime will provide
the monitoring engine; the product will compose its configuration, storage, and
runtime lifecycle.

## Design Principles

- **Fiber-native:** use the framework and engine's existing capabilities.
- **Zero infrastructure by default:** embedded persistence is the planned default.
- **One binary:** keep deployment small.
- **Configuration-driven:** YAML will describe the monitored services.
- **Explicit history lifecycle:** configuration changes never silently delete history.
- **Small dependency surface:** add dependencies only when implementation needs them.
- **Predictable operations:** make startup, shutdown, and failure behavior explicit.

## Planned v0.1.0

Planned features include HTTP/HTTPS endpoint monitoring, a status page and JSON
API, a reusable public bbolt backend, optional Redis persistence, optional TLS
and Basic Auth, health endpoints, and explicit service history export/removal.
These features are not available in P0. See the
[product/technical design](docs/design/v0.1.0-product-technical-design.md) for
the intended behavior and boundaries.

## Current Development Status

P0 establishes the Go module, minimal CLI and behavioral tests, Agent context,
engineering contracts, design documentation, and local/CI verification.

The current executable supports only help and version. No-argument invocation
shows help. Unknown commands or extra arguments return usage errors; there are
no placeholder runtime commands. P0 uses only the Go standard library.

## Architecture

Today, `cmd/uptime` contains the entire executable. In later phases, internal
application/configuration/archive packages will compose Fiber and a public
`storage/bbolt` implementation. Those packages are intentionally absent until
they have real implementations. See the [architecture guide](.agents/architecture.md).

## Development

Use Go 1.26 or a supported newer version and GNU Make. CI is configured to run
the same checks on Go 1.26.x and 1.27.x.

```sh
make check
```

This checks formatting, runs `go vet` and tests, and verifies package builds
without producing a packaged binary. Use `make fmt` to format Go source. The
[playbook](.agents/playbook.md) documents individual checks and equivalent Go
commands when Make is unavailable.

Try the implemented CLI:

```sh
go run ./cmd/uptime --help
go run ./cmd/uptime version
```

Development version output identifies `dev`, the Go runtime version, and commit
`unknown`. The package-level version and commit strings allow future linker
injection; no release packaging workflow exists in P0.

## Documentation

- [v0.1.0 Product & Technical Design](docs/design/v0.1.0-product-technical-design.md)
- [bbolt Storage Design](docs/design/bbolt-storage.md)
- [Agent Entry Point](AGENTS.md) and [Agent Architecture](.agents/architecture.md)
- [Engineering Contracts](contracts/): [Compatibility](contracts/compatibility.md),
  [Persistence](contracts/persistence.md), [Upstream](contracts/upstream.md)
- [Change Playbook](.agents/playbook.md)
- [Architecture Decisions](docs/decisions/README.md)
- [Changelog](CHANGELOG.md)

## License

[MIT](LICENSE), Copyright (c) 2026 DeepFurry.
