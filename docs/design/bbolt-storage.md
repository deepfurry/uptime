# bbolt Storage Design

Status: planned v0.1.0 design; no backend is implemented in P0.

This document develops the storage rationale in the
[initial product/technical baseline](v0.1.0-product-technical-design.md), sections
15–20 and 26–27. Stable invariants live in the
[persistence contract](../../contracts/persistence.md). The schema below is a
design, not a released file format or a third-party bucket API.

## Default Backend and Public Package

bbolt provides embedded persistence in one local file, matching the product's
one-binary, zero-external-infrastructure default. Explicit transactions allow
slot deduplication, counters, and related metadata to change atomically. This
choice accepts single-process file ownership and offline maintenance rather
than introducing a database service for the default deployment.

The planned `github.com/deepfurry/uptime/storage/bbolt` package is public so a
third-party Fiber application can reuse it without running the standalone
product. It implements Fiber Contrib Uptime's exported `uptime/storage.Store`
contract and never imports this repository's `internal/*` packages. Verify the
exact upstream interface during implementation and add a compile-time interface
assertion then; P0 does not create speculative Go types or dependencies.

The old Redis-emulation approach mentioned by the product baseline is rejected:
a direct Store implementation expresses service, heartbeat, and daily semantics
without translating embedded data through a Redis-shaped compatibility layer.
This decision follows the in-repository baseline and requires no prototype code.
Optional Redis persistence will use Fiber's backend directly.

## Planned Schema v1

```text
uptime.db
├── meta
│   └── schema_version
├── services
│   └── <service-id>
│       ├── name
│       ├── description
│       ├── created_at
│       ├── last_seen_at
│       └── sample_interval
├── instances
│   └── <instance-id>
│       ├── service_id
│       ├── hostname
│       ├── pid
│       ├── started_at
│       └── last_seen_at
├── samples
│   └── <service-id>
│       └── <YYYY-MM-DD>
│           ├── up_slots
│           └── slots
│               └── <slot-key>
└── daily
    └── <service-id>
        └── <YYYY-MM-DD>
            ├── up_slots
            ├── expected_slots
            └── finalized
```

Service IDs are persistent identities. Names and descriptions can change;
`CreatedAt` cannot. Instance `StartedAt` is stable, while its service association,
hostname, and PID may refresh. Both entities' `LastSeenAt` take the maximum of
existing and incoming timestamps. Active/detached status is derived from current
configuration and stored service IDs; it is not a persisted boolean.

Use raw UTF-8 strings, eight-byte big-endian int64/uint64 values, UnixNano int64
timestamps, nanosecond int64 durations, and single-byte booleans. Instance and
slot keys use big-endian integers; days use ASCII `YYYY-MM-DD`, interpreted in
the configured timezone. Avoid JSON, Gob, Protobuf, or MsgPack inside the DB.
Concrete encoding helpers and their tests will define signed-value handling
when implemented. No general serialization framework is needed.

## Transactions and Heartbeat Deduplication

Use short explicit read/write transactions. Do not retain transaction-owned
values after the transaction ends; copy data needed by callers. Failed writes
must roll back related changes together.

Heartbeat identity is `(ServiceID, Day, Slot)`. In one `db.Update`:

1. Advance service and instance `LastSeenAt` monotonically.
2. Insert the slot key only if absent.
3. Increment the day's `up_slots` only for a newly inserted slot.

This makes retries idempotent and keeps the redundant count consistent with
stored slots. One slot uses one key; bitmap optimization is deferred. Upstream
day/slot semantics remain authoritative. Contract tests must cover duplicate
and out-of-order writes as well as concurrent callers.

## Rollup, Finalization, and Cleanup

Rollup processes days strictly before `BeforeDay`. Once a daily row is finalized,
it cannot be overwritten. Use the `ExpectedSlots` callback only within the
operation's call lifetime; do not retain it. Eligible days for one service can
be processed in a short write transaction. Rollup does not immediately remove
raw samples.

Cleanup removes raw samples only when all conditions hold:

- The day is strictly before `SamplesBeforeDay`.
- Its corresponding daily row exists.
- That row is finalized.

Apply this guard before pruning daily history using `DailyBeforeDay`; an
unfinalized day's necessary raw samples must survive. Instance metadata may be
pruned after roughly 24 hours during cleanup, without a separate worker.
Return queries in deterministic service-ID/day order where applicable.

## Explicit Service Removal

`RemoveService` deletes the service, its raw samples, its daily rows, and all
instances associated with its ID in one write transaction. Failure cannot leave
a partially removed history.

The application enforces the planned prohibition on removing services still
active in YAML; the storage package does not import application configuration.
Deleting an endpoint from YAML only stops future observation. It never invokes
history deletion implicitly. Maintenance commands use public storage operations,
not raw bucket access.

## Context and File Ownership

bbolt transactions do not accept a `context.Context`. Cancellation is best
effort: check `ctx.Err()` before entering a transaction and periodically during
long loops. Do not promise that context cancellation can precisely interrupt
writer-lock waiting or an in-flight transaction. The planned open timeout bounds
file-lock acquisition; it is not a general context-cancellation guarantee.

The product uses a single process to own a bbolt file. A second process must not
open that same file for concurrent product operations. Consequently bbolt
service list/export/remove commands are planned as offline maintenance: stop
the application, perform the command, then restart. No IPC or admin HTTP control
plane is introduced to bypass this boundary.

## Schema Evolution and Readiness

Create new databases with `meta/schema_version = 1`. Open a supported version;
refuse a newer unsupported schema without rewriting it. Older released schemas
need explicit supported forward migration before use. Missing or malformed
metadata in an existing database is an error, not permission to reinitialize or
discard it. A future v2 can add a concrete v1-to-v2 migration; P0 needs no generic
migration framework.

Planned `Ping` uses a lightweight read transaction to verify an open handle and
readable, supported schema metadata. It creates no temporary write data and does
not promise that the next write will succeed. Backend failures never switch
silently to in-memory persistence.

## Lifecycle and Backup

The caller opens the Store before giving it to Fiber Uptime and owns its close.
The Store contract does not transfer resource ownership to the middleware.
During normal shutdown, Fiber Uptime background tasks stop before application
`OnPostShutdown` closes storage. A top-level safety cleanup covers construction
failures; `Close` must be idempotent so both paths can safely share ownership.

The planned default path is `./data/uptime.db`, with parent directories created
as needed (recommended `0750`) and the database file using `0600`. Failure to
open the configured backend is fatal at startup.

Use a cold backup for v0.1.0: gracefully stop the application, copy the complete
database, then restart. An ordinary copy during writes is not a promised
consistent backup. Online transactional backup can be designed later if needed.

A service archive is a versioned, human-readable history export assembled through
Store queries, independent of raw bbolt layout. It excludes secrets and process
instance metadata. It is not a database backup and v0.1.0 provides no archive
import/restore. Third-party consumers use public Go APIs or documented archives,
not direct bucket edits.
