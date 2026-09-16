# Architecture

P0 implements only `cmd/uptime`: help, version, usage errors, and behavioral tests.
It has no third-party Go dependencies, configuration loader, HTTP listener, or
storage. The following direction describes the planned v0.1.0 application;
future packages must be created only when implementation requires them.

```text
cmd/uptime
    |
    v
internal/app
    |
    +-----------------+------------------+
    v                 v                  v
internal/config   internal/archive   Fiber runtime
                                         |
                                         v
                                 Fiber Contrib Uptime
                                         |
                                         v
                                 uptime/storage.Store
                                    /          \
                                   v            v
                             storage/bbolt   Fiber Redis
```

The app composes concrete backends; archive operations use the Store query
boundary. The arrows describe responsibilities, not a requirement to wrap every
upstream API in a new abstraction.

## Responsibilities

| Area | Responsibility | P0 status |
| --- | --- | --- |
| `cmd/uptime` | Executable entry point and CLI wiring | Help and version only |
| `internal/app` | Fiber composition, lifecycle, health endpoints | Planned |
| `internal/config` | Strict YAML, defaults, active environment resolution, validation | Planned |
| `internal/archive` | Backend-independent service archive/export | Planned |
| `storage/bbolt` | Public implementation of Fiber Uptime's Store contract | Planned |
| `configs` | User-facing example configuration | Planned |
| `contracts` | Stable engineering constraints | Present |
| `docs/design` | Design rationale and implementation baselines | Present |
| `docs/decisions` | Significant changes after the initial baseline | ADR guide only |

## Invariants

- `cmd` contains CLI wiring, not product/domain logic.
- Public `storage/bbolt` never imports `internal/*`.
- Config parsing is independent of the application runtime; validation must not
  require starting the HTTP application.
- Archive logic does not depend on raw bbolt buckets. CLI maintenance commands
  do not manipulate buckets directly.
- Runtime persistence goes through explicit persistence boundaries, with no
  hidden fallback to memory when a backend fails.
- Fiber hooks own startup/shutdown lifecycle after application construction.
  Storage remains open until Uptime background tasks stop and Fiber shuts down;
  construction failures still require caller-owned cleanup.
- Removing an endpoint from configuration never deletes its history.
- Redis provides shared persistence, not distributed probe scheduling.
- Target availability never determines the Uptime process's readiness.

See the [product baseline](../docs/design/v0.1.0-product-technical-design.md)
for planned behavior and [contracts](../contracts/) for stable constraints.
