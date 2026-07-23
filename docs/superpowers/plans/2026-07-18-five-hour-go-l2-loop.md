# Five-Hour L2 Loop Plan — Go Capability Contract PoC

**Window:** 2026-07-18 00:03:53–05:03:53 Asia/Shanghai
**Mode:** L2 assisted implementation; no deploy, remote write, CI, infrastructure, database migration, or production access.

## Scope and outcome

Deliver one low-risk `system.capability.list` vertical slice in Go. API, MCP and non-interactive CLI must normalize into one registry/invocation layer and return the same declared result schema. The result is a PoC evidence package, not a production CRM release.

## Plan

1. **00:00–00:35 — Preconditions and safe baseline.** Recreate L2 constraints, budget, state and append-only run log; obtain Go 1.26.5; review the exact official Go MCP SDK release and licenses. Stop MCP/dependency work if the dependency gate is not auditable.
2. **00:35–01:30 — Contract core.** Write failing unit tests, then implement a versioned JSON request/result envelope, registry, `system.capability.list`, stable validation/not-found/authorization/idempotency errors, request ID and audit ID.
3. **01:30–02:20 — API and CLI adapters.** Add an HTTP functional API and a JSON/JSON Lines-only CLI. Prove successful and failed same-input parity and prove that a non-TTY child process never prompts.
4. **02:20–03:20 — MCP adapter.** Use the approved official Go SDK to project registry metadata as a stdio tool. Test in process and as a child process; stdout must remain protocol-only.
5. **03:20–04:15 — Reliability and delivery evidence.** Add bounded-concurrency, timeout, stable-overload and idempotency coverage; cross-build the binary without CGO where the toolchain permits. Record actual commands and artifacts only.
6. **04:15–05:00 — Independent verification and handoff.** A separate verifier reruns parity, no-TTY, stdout-purity, build and denylist checks without implementation edits. Record results, unresolved gaps and the hard stop in project state.

## Stop conditions

- Three failed attempts on one work item.
- Any denylist path or production connection.
- A dependency outside ADR-007's approved license policy, or no reproducible dependency evidence.
- No independent verifier by the final verification window.

## Explicitly deferred

PostgreSQL schema/capacity, tenant control plane, authentication/authorization integration, production deployment, SBOM signing pipeline, HA, backup and recovery remain separate tasks.
