# Persistence Contract

P1 implements these constraints in the public `storage/bbolt` package. P3 supplies
the same public Store to Fiber Uptime's standalone runtime; Schema v1 is unchanged.

- Every database has `meta/format = deepfurry-uptime-bbolt` and an eight-byte
  big-endian uint64 `meta/schema_version = 1`, plus services, instances, samples,
  and daily buckets. Missing/malformed metadata, wrong identity, and unsupported
  versions (older or newer) refuse to open. Released schema changes require
  explicit forward migrations; P1 has no migration from other versions.
- Only a definitively newly created file may be initialized. Existing empty,
  unrelated, or invalid files are never adopted or repaired.
- Service `CreatedAt` and instance `StartedAt` remain stable once meaningfully
  set, falling back to incoming `LastSeenAt` when absent. Both-zero registration
  leaves the creation/start field absent so a later upsert can establish it.
- `LastSeenAt` is monotonic, including repeated or out-of-order observations.
- Heartbeat identity is `(ServiceID, Day, Slot)`. Repeated writes are idempotent
  and cannot increase the successful-slot count more than once.
- Heartbeats only refresh existing metadata; they never invent registrations.
- Heartbeat and current-day query hot paths require a strictly decoded,
  non-negative persisted `up_slots` counter and a `slots` bucket, without
  scanning historical slots. A heartbeat validates an existing marker only at
  its own slot; duplicates require `0x01` and do not increment the counter.
- Rollup and raw-sample cleanup fully validate each sample day they process:
  slot keys must be valid non-negative integers, markers must be `0x01`, and
  the actual slot count must equal `up_slots`. Historical corruption unrelated
  to the accessed slot may remain undetected by hot paths. Malformed accessed
  fields, invalid encodings, counter mismatches found by full validation, and
  platform-int overflow return contextual errors; data is never auto-repaired.
- Finalized daily rows are immutable.
- Rollup callbacks run outside transactions. A short write transaction rechecks
  finalization and rereads current samples; it cannot resurrect removed history.
- Cleanup cannot delete samples required by an unfinalized day.
- Cleanup uses exclusive boundaries and processes raw samples before daily
  history, then instance metadata. Instance activity is `LastSeenAt`, falling
  back to `StartedAt` only when last-seen is zero. Only non-zero activity
  strictly before the current time minus 24 hours expires; the exact cutoff
  and instances with both timestamps zero are retained.
- Removing an endpoint from configuration never implicitly removes its history.
  Service deletion is explicit and atomic across the service's stored entities.
- Archive export is not database backup.
- Storage failures never silently fall back to ephemeral/in-memory persistence.
- The application opens bbolt before listener/Uptime construction and owns Close.
  Startup open/schema/lock failures are fatal; later operations retain upstream
  degraded behavior. On shutdown, Uptime workers and HTTP handlers stop before
  the application closes storage, including timeout and listener-error paths.
- `Close` is idempotent. Operations observe context cancellation before work and
  during scans, without promising to interrupt a blocked bbolt lock or disk I/O.
- Nil query service selections mean all registered services, explicit empty
  selections mean none, and daily query bounds are inclusive. Ordering remains
  unspecified. Empty sample days return no rows; zero-up sample rows are omitted.

The bbolt file is a versioned internal persistent representation, not a
user-editable public data format. Third-party callers use public Go APIs or
documented archive formats, rather than depending on raw bucket layout.

See [bbolt design](../docs/design/bbolt-storage.md) for the implemented schema and
transaction rationale, and [compatibility](compatibility.md) for evolution rules.
