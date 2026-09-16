# Upstream Contract

P1 directly uses Fiber Contrib Uptime v0.2.0's public `storage` package and bbolt
v1.5.0. `go.mod` is authoritative for actual dependencies and versions. Only the
storage contract is imported from Uptime; its transitive module graph does not
mean that a standalone Fiber app or Redis integration has been implemented.
The remaining entries record assumptions for future v0.1.0 integrations.

| Upstream | Role | Important assumption |
| --- | --- | --- |
| Go | Toolchain/runtime | Support window follows the active Fiber major version |
| Fiber v3 | HTTP/runtime lifecycle | Fiber hooks own runtime lifecycle |
| Fiber Contrib Uptime | Uptime engine | Upstream semantics remain authoritative unless explicitly overridden |
| Fiber Storage Redis | Optional external persistence | Application owns storage lifecycle |
| bbolt | Embedded persistence | Single-process file locking and explicit transaction model |

The v0.2.0 Store contract distinguishes nil service selections (all registered
services) from explicit empty selections (none); query bounds are inclusive,
retention/rollup bounds are exclusive, and ordering is unspecified. Callers own
initialization and shutdown. `ExpectedSlots` runs only during the rollup call;
our backend additionally guarantees it runs outside database transactions.

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
