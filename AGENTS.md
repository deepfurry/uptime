# Agent Guide

## Repository Purpose

DeepFurry Uptime is a small, self-hosted uptime product built around Fiber.
P1 provides a public bbolt backend; P2 adds strict configuration loading;
P3 composes Fiber/Uptime into `serve` with bbolt, plain HTTP, health endpoints,
and graceful shutdown. Redis/TLS/Auth runtime support is not implemented.

## Start Here

1. Read [.agents/architecture.md](.agents/architecture.md).
2. Read [.agents/playbook.md](.agents/playbook.md).
3. Read the relevant [contracts](contracts/).
4. Inspect current source and tests.
5. Consult [designs](docs/design/) and [ADRs](docs/decisions/) for architectural intent.

## Repository Map

- `cmd/uptime`: executable entry point, minimal CLI, behavioral tests.
- `internal/config`: raw YAML, active env resolution, normalized types, validation.
- `internal/app`: capability gates, Fiber/Uptime composition, health and lifecycle.
- `configs`: official example that validates without external services or secrets.
- `storage/bbolt`: public Fiber Uptime Store implementation and persistence tests.
- `.agents`: architectural boundaries and change workflow.
- `contracts`: compatibility, persistence, and upstream constraints.
- `docs/design`: planned v0.1.0 baseline and bbolt design rationale.
- `docs/decisions`: significant decisions after the baseline.
- `Makefile`, `.github/workflows/ci.yml`: local verification and CI.

## Sources of Truth

Source/tests define implemented behavior; `go.mod` and CI define toolchain policy.
Contracts define stable constraints. The product design is the future v0.1.0
baseline, not evidence that a feature exists. Schemas and migrations become the
authority for concrete persistent representations when implemented. Surface any
conflict before changing behavior; do not silently rewrite a contract or design.

## Core Rules

- Work within this repository; do not depend on surrounding workspace content.
- Stay Fiber-native, with a deliberately small dependency surface. Direct dependencies
  are Fiber v3, Fiber Contrib Uptime, bbolt, and stable YAML v3; add dependencies with their feature.
- Do not reimplement Fiber/Fiber Contrib Uptime behavior without a concrete product reason.
- YAML is the user-facing configuration source of truth. Resolve and
  validate active configuration only; strict decoding still rejects unknown keys.
- `internal/config.LoadFile` returns fully normalized active branches. Config check
  reads only YAML/active TLS files; it never opens storage or starts network/runtime work.
- Never log or dump normalized configuration or include configured secret values in errors.
- `internal/app.New` performs no I/O. Single-use `Run` opens bbolt and binds before
  constructing Uptime, then serves only configured endpoints without a self service.
- The public `storage/bbolt` package must remain independent of `internal/*`.
- Never delete service history implicitly. Version persistent schema evolution.
- Fiber hooks own runtime lifecycle once the application is running. Storage
  must outlive Uptime background tasks and close after Fiber shutdown.
- Target-service failure is monitoring data; Uptime runtime failure is operational health.
- No hot reload, management UI, user account system, alerting, APM, or unrelated
  monitoring expansion without an explicit architecture decision.
- Preserve MIT `LICENSE`; use DeepFurry organization community files rather than
  adding local Code of Conduct, Contributing, or Security copies.
- Do not create empty future packages or advertise unimplemented CLI commands.

## Verification

Run `make check` before completing a change. When GNU Make is unavailable, use the
equivalent Go commands in [.agents/playbook.md](.agents/playbook.md) and verify
formatting leaves a clean diff. Report what actually ran and any gaps.
For storage or runtime changes also run `make race`; race is deliberately separate from
the fast canonical check and runs in a dedicated Go 1.27.x CI job.
