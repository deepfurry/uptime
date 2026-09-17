# Architecture

P1 implements `storage/bbolt`, an independently reusable Fiber Uptime Store.
P2 adds `internal/config`, called directly by `cmd/uptime config check`, alongside
help/version. The CLI does not wire storage or start a monitoring runtime.
The following direction describes the planned
v0.1.0 application; future packages are created only when implementation needs them.

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

| Area | Responsibility | Current status |
| --- | --- | --- |
| `cmd/uptime` | Executable entry point and CLI wiring | Help, version, config check |
| `internal/app` | Fiber composition, lifecycle, health endpoints | Planned |
| `internal/config` | Strict YAML, defaults, active environment resolution, validation | Implemented; returns normalized typed config |
| `internal/archive` | Backend-independent service archive/export | Planned |
| `storage/bbolt` | Public implementation of Fiber Uptime's Store contract | Implemented, caller-owned lifecycle |
| `configs` | User-facing example configuration | Implemented; no active secret/env requirements |
| `contracts` | Stable engineering constraints | Present |
| `docs/design` | Design rationale and implementation baselines | Present |
| `docs/decisions` | Significant changes after the initial baseline | ADR guide only |

## Invariants

- `cmd` contains CLI wiring, not product/domain logic.
- Public `storage/bbolt` never imports `internal/*`.
- Config parsing is independent of the application runtime; validation must not
  require starting the HTTP application.
- Raw YAML and normalized configuration are separate. Defaults apply only to
  omitted fields; inactive TLS/Auth/storage branches are nil in the result.
  Durations, calendar days, URLs, timezone, and active TLS keypairs are parsed
  before return. Do not repeat defaults, branch selection, or parsing in the app.
- Config check may read active TLS files, but cannot create directories, open
  databases, resolve DNS, connect Redis, bind listeners, or probe endpoints.
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
- The bbolt backend owns no goroutines or product policy. Transactions provide
  write serialization; caller callbacks run outside transactions. Its database
  format, scalar encoding, and invariants are validated by persistence tests.

See the [product baseline](../docs/design/v0.1.0-product-technical-design.md)
for planned behavior and [contracts](../contracts/) for stable constraints.
