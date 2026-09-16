# Compatibility Contract

## Current Surface

P0 exposes only help (`uptime`, `help`, `-h`, `--help`) and version (`version`,
`--version`). Each invocation accepts at most one command or option. Additional
arguments and unknown commands/options are usage errors, including planned but
unimplemented commands. Successful output goes to stdout; diagnostics go to
stderr. Help and version return 0, usage errors return 2, and internal failures
(such as writing successful output) return 1. See source/tests for concrete text.

P0 has no public library API or runtime configuration interface. The module is
`github.com/deepfurry/uptime`; its Go baseline is 1.26. Go support follows the
active Fiber major version's supported window; CI currently covers 1.26.x and
1.27.x. Updating that window requires coordinated module, CI, and documentation
changes.

## Compatibility-Sensitive Boundaries

As implemented and released, preserve the semantics of:

- Public Go APIs and their lifecycle/error behavior.
- CLI commands, flags, output semantics, and exit behavior.
- Future `uptime.yaml` keys, defaults, and environment interpolation behavior.
- `/livez`, `/readyz`, and product-exposed Uptime dashboard/API routes.
- Archive JSON formats and documented defaults.
- Persistent schema migration behavior.

This list constrains future work; it does not claim those runtime surfaces exist
in P0. Concrete APIs and formats belong in source, tests, and their specific
documentation, not duplicated here.

## Pre-1.0 Evolution

- v0.x may evolve, but prefer additive changes whenever practical.
- Patch releases (for example, v0.1.1 to v0.1.2) must not intentionally introduce
  breaking user-facing changes.
- Minor releases (for example, v0.1 to v0.2) may break compatibility before v1.0
  only intentionally, with documentation, tests, and migration guidance where
  applicable.
- Released persistent data must not become unreadable without a supported
  forward migration. Pre-1.0 status is not permission to discard user history.
