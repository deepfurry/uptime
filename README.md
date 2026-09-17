# Uptime

A tiny, self-hosted uptime service being built around Fiber.

The planned experience: one binary, one YAML file, and zero external
infrastructure required by default.

> [!NOTE]
> Uptime is under active development toward v0.1.0. It provides a reusable public
> bbolt backend and strict YAML validation through `config check`. The standalone
> monitoring runtime and `serve` command are not implemented yet.

## Overview

DeepFurry Uptime aims to make HTTP service availability and persistent history
easy to operate in a small standalone product. Fiber Contrib Uptime will provide
the monitoring engine; the product will compose its configuration, storage, and
runtime lifecycle.

## Design Principles

- **Fiber-native:** use the framework and engine's existing capabilities.
- **Zero infrastructure by default:** embedded persistence is the planned default.
- **One binary:** keep deployment small.
- **Configuration-driven:** YAML describes the monitored services.
- **Explicit history lifecycle:** configuration changes never silently delete history.
- **Small dependency surface:** add dependencies only when implementation needs them.
- **Predictable operations:** make startup, shutdown, and failure behavior explicit.

## Planned v0.1.0

Planned features include HTTP/HTTPS endpoint monitoring, a status page and JSON
API, a reusable public bbolt backend, optional Redis persistence, optional TLS
and Basic Auth, health endpoints, and explicit service history export/removal.
The public bbolt backend and configuration validation are implemented. Standalone
Fiber composition, TLS listeners, Auth middleware, and Redis product integration
remain future work. See the
[product/technical design](docs/design/v0.1.0-product-technical-design.md) for
the intended behavior and boundaries.

## Current Development Status

P1 adds `github.com/deepfurry/uptime/storage/bbolt`, directly implementing Fiber
Contrib Uptime v0.2.0's public `storage.Store` contract using bbolt v1.5.0. It
provides schema-versioned persistence, atomic service-level heartbeat deduplication,
rollup, queries, cleanup, explicit service removal, and contract tests.

P2 adds normalized configuration and `config check`. No-argument invocation shows
help; help/version remain available. Unknown commands or unexpected arguments
return usage errors. Stable YAML v3 is the only additional direct dependency;
no CLI/config framework or test framework is added.

## Validate Configuration

The [official example](configs/uptime.example.yaml) validates without environment
variables, a bbolt file, Redis, or TLS certificates:

```sh
go run ./cmd/uptime config check --config configs/uptime.example.yaml
```

Success prints only `configuration is valid`. With no `--config`, the command
reads `./uptime.yaml`. The flag belongs to `config check`, not the root command.
Validation errors go to stderr and return 1; CLI usage errors return 2.

Configuration uses one strict YAML document: unknown fields, duplicate keys,
wrong types, explicit nulls, and merge keys are rejected. Defaults apply only
to omitted fields. `${VAR_NAME}` substitution is one-pass and limited to allowed
active string fields. Inactive TLS/Auth/storage values are neither resolved nor
semantically validated; their YAML structure still has to be valid.

Config check validates matching TLS certificate/key files only when TLS is enabled.
Relative paths use the process working directory. It does not create directories,
open storage, connect Redis, resolve DNS, probe URLs, or bind ports. Auth validation
requires username/hash but leaves verifier parsing to the future Fiber integration.
No configuration or secret values are dumped. Avoid placing secrets in URLs.

See the [configuration and CLI contract](contracts/compatibility.md) for defaults,
field rules, and normalization. `uptime serve` remains an unknown command.

## Public bbolt Package

Use `Open(Config{Path: "./data/uptime.db"})` from the public package to obtain a
ready `*Store`. `Path` is required; a zero lock timeout defaults to five seconds.
Callers own the Store and call `Close` after all users have stopped. The backend
also provides `Name`, `Ping`, and atomic `RemoveService`.

One Store owns a database file at a time. Existing invalid files are rejected,
never adopted or repaired. There is no raw DB/bucket API and no active/detached
product policy in the backend. See the [storage design](docs/design/bbolt-storage.md)
for schema, cancellation limitations, and cold-backup guidance.

## Architecture

`cmd/uptime` contains the executable and calls `internal/config` for validation.
`storage/bbolt` is independently reusable and imports no project `internal/*`
package. Future application/archive packages will compose Fiber and storage;
they are still absent.
See the [architecture guide](.agents/architecture.md).

## Development

Use Go 1.26 or a supported newer version and GNU Make. CI is configured to run
the same checks on Go 1.26.x and 1.27.x.

```sh
make check
make race
```

`make check` checks formatting, runs `go vet` and tests, and verifies package
builds without producing a packaged binary. `make race` separately checks the
storage package with the Go race detector and runs in a dedicated Go 1.27.x
Ubuntu CI job. It requires a supported platform and C compiler. Use `make fmt`
to format repository Go source, excluding ignored caches. The
[playbook](.agents/playbook.md) documents individual checks and equivalent Go
commands when Make is unavailable.

Try the implemented CLI:

```sh
go run ./cmd/uptime --help
go run ./cmd/uptime version
go run ./cmd/uptime config check --help
```

Development version output identifies `dev`, the Go runtime version, and commit
`unknown`. The package-level version and commit strings allow future linker
injection; no release packaging workflow exists.

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
