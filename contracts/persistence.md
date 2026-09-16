# Persistence Contract

These constraints govern the planned v0.1.0 persistence implementation. P0 has
no storage code or released database schema.

- Every bbolt database has an explicit schema version. Unsupported newer
  versions refuse to open; released schema changes use explicit forward migrations.
- Service `CreatedAt` remains stable; instance `StartedAt` remains stable.
- `LastSeenAt` is monotonic, including repeated or out-of-order observations.
- Heartbeat identity is `(ServiceID, Day, Slot)`. Repeated writes are idempotent
  and cannot increase the successful-slot count more than once.
- Finalized daily rows are immutable.
- Cleanup cannot delete samples required by an unfinalized day.
- Removing an endpoint from configuration never implicitly removes its history.
  Service deletion is explicit and atomic across the service's stored entities.
- Archive export is not database backup.
- Storage failures never silently fall back to ephemeral/in-memory persistence.

The bbolt file is a versioned internal persistent representation, not a
user-editable public data format. Third-party callers use public Go APIs or
documented archive formats, rather than depending on raw bucket layout.

See [bbolt design](../docs/design/bbolt-storage.md) for the proposed schema and
transaction rationale, and [compatibility](compatibility.md) for evolution rules.
