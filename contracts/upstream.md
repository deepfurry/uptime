# Upstream Contract

P3 directly uses Fiber v3.5.0, Fiber Contrib Uptime v0.2.0, bbolt v1.5.0, and
stable `go.yaml.in/yaml/v3`. `go.mod` is authoritative for versions. Uptime's root
package brings Redis-related transitive imports, but the product supplies only
its public bbolt Store and does not construct or connect Redis. YAML v3 replaces
the original provisional v4 choice; no CLI/config framework is used.

| Upstream | Role | Important assumption |
| --- | --- | --- |
| Go | Toolchain/runtime | Support window follows the active Fiber major version |
| Fiber v3 | HTTP/runtime lifecycle | Fiber hooks own runtime lifecycle |
| Fiber Contrib Uptime | Uptime engine | Upstream semantics remain authoritative unless explicitly overridden |
| Fiber Storage Redis | Future optional external persistence | Not constructed by the product runtime |
| bbolt | Embedded persistence | Single-process file locking and explicit transaction model |
| YAML v3 | Configuration decoding | Known fields, duplicate rejection, single document; scalar errors must not expose secrets |

The v0.2.0 Store contract distinguishes nil service selections (all registered
services) from explicit empty selections (none); query bounds are inclusive,
retention/rollup bounds are exclusive, and ordering is unspecified. Callers own
initialization and shutdown. `ExpectedSlots` runs only during the rollup call;
our backend additionally guarantees it runs outside database transactions.

P3 verifies these lifecycle assumptions against the pinned source and integration tests:

- `uptime.New` starts runtime workers during construction. Open bbolt and bind
  first. Supply `Config.Storage`; do not set Redis `Store` or self-service fields.
- A custom Store is already initialized and caller-owned; upstream does not close
  it. Uptime's Fiber pre-shutdown hook cancels/waits its workers. Close bbolt only
  after Fiber shutdown and HTTP handler completion, including failure paths.
- Register ready=false pre-shutdown before Uptime. Fiber's OnListen occurs before
  fasthttp Serve takes the listener: the private listener's first Accept signals
  that shutdown can safely close it. A shutdown timeout does not itself force-close
  every active connection, so the wrapper closes transport I/O and waits for the
  server to finish those connections before releasing storage.
- Uptime constructor rejection panics rather than returning an error. Convert this
  boundary into a secret-safe operational error without rewriting degraded runtime
  policy. Configuration loading enforces v0.2.0's endpoint timeout >= 1ms rule,
  as well as timeout <= effective endpoint interval, before runtime construction.
- Empty UI description/footer are defaulted again upstream; the P3 config corrective
  change rejects explicit empty values. An empty favicon remains supported.
- Storage operation failures after successful handoff retain upstream degraded
  behavior. Target failures remain monitoring data, independent of process readiness.

- Prefer Fiber-native capabilities when they satisfy product requirements.
- Do not introduce a parallel runtime lifecycle framework.
- Fiber Contrib Uptime is the engine; do not casually fork or copy it.
- Its exported `uptime/storage.Store` contract is the intended integration
  boundary for the public bbolt backend. Do not depend on unexported internals.
- Shared Redis persistence does not imply distributed probe scheduling: each
  application instance still runs its own probes.
- Before material upgrades, inspect upstream release/migration notes and the
  affected source, tests, and contracts. Validate assumptions against the actual
  selected version, rather than treating design-time API sketches as authoritative.
- Follow Fiber's supported Go window; keep `go.mod`, CI, and documentation aligned.

The [product design](../docs/design/v0.1.0-product-technical-design.md) captures
initial intended integrations. Upstream changes do not authorize silently
changing a product contract or broadening the current phase.
