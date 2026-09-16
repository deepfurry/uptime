// Package bbolt implements Fiber Uptime's public storage.Store contract using
// a schema-versioned local bbolt database. Open returns a ready Store; callers
// must stop all users of the Store before closing it.
//
// A database file has one writer owner. Maintenance from another Store or
// process requires releasing that ownership first. Context cancellation is
// checked before transactions and during scans, but cannot precisely interrupt
// bbolt lock acquisition or an in-flight I/O operation.
//
// The database representation is private, versioned persistence. Use Store
// operations rather than editing buckets. The backend does not interpret YAML,
// enforce product endpoint-ID rules, or decide whether a service is active.
package bbolt
