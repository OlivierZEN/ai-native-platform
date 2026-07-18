# Phase 0 Go Capability Contract Implementation Plan

> **Status:** L2 source work for the bounded Capability Contract PoC was explicitly authorized by the user on 2026-07-18. The active five-hour execution plan is `docs/superpowers/plans/2026-07-18-five-hour-go-l2-loop.md`; deployment, release, production infrastructure and database work remain unauthorized.

> **2026-07-18 outcome:** The active L2 plan independently verified Tasks 1–4 and Task 7 only for the low-risk `system.capability.list` slice. Task 5's release artifacts and Task 6's PostgreSQL baseline remain separate, unauthorized work. The broad exit criteria at the end of this historical Phase 0 plan are therefore not claimed by this PoC.

**Goal:** Prove one low-risk capability can be invoked through functional API, MCP, and a non-interactive CLI from one Go implementation, then establish the PostgreSQL 16.13 Phase 0 performance and isolation baseline.

**Architecture:** A single Go binary owns transport adapters, but adapters normalize into one versioned JSON Schema capability request and call one registry/invocation layer. PostgreSQL 16.13 runs in the already available Docker environment as a single-AZ, single-writer PoC. HA, backup, recovery and production deployment are outside this plan.

**Runtime:** Go 1.26.5 for CI and release evidence; Go 1.26.4 on this workstation is not sufficient release evidence. A dependency is selected only after version, maintenance, license and SBOM review. The initial artifact target matrix is Linux amd64/arm64, macOS arm64, and Windows amd64, with `CGO_ENABLED=0` unless an approved exception exists.

## Entry Gates

All gates must be recorded with actual evidence before Task 1 starts:

1. The user explicitly authorized the narrow `TASK-020` L2 vertical slice on 2026-07-18. This is not approval of the remaining FEAT-009 production decisions.
2. The L2 path allowlist and denylist are recorded in `loop-constraints.md`.
3. An approved Go 1.26.5 toolchain must be installed and its version recorded before release-quality verification; Go 1.26.4 is not such evidence.
4. The Go MCP SDK and JSON Schema validator must pass the ADR-007 dependency, vulnerability and license gate before their use.
5. An independent verifier must be named before API/MCP/CLI parity can be declared complete.

## Task 1: Create the Go Repository Baseline

**Files:** `go.mod`, `go.sum`, `cmd/ai-native-platform/main.go`, `internal/`, `schemas/`, release and compliance configuration.

- Create a single `ai-native-platform` executable with machine-only command parsing: `serve`, `mcp stdio`, `capability list`, `capability describe`, and `capability invoke`.
- Freeze module checksums and configure CI to use the approved internal module proxy; release builds must not resolve public modules directly.
- Add a reproducible local build command and the cross-platform build matrix with CGO disabled.
- Verify with Go 1.26.5: `go test ./...`, `go vet ./...`, and the declared cross-compilation commands.

## Task 2: Implement the Capability Core with Tests First

**Files:** `internal/capability/registry.go`, `internal/capability/invoke.go`, `internal/capability/*.test.go`, `schemas/capability-request.schema.json`, `schemas/capability-result.schema.json`.

- Define versioned JSON Schema Draft 2020-12 request/result contracts and a stable error vocabulary.
- Write failing tests for `system.capability.list`, malformed input, unknown capability, non-empty request/audit IDs, authorization denial, and idempotency conflict.
- Implement a registry as the only source for ID, version, schema, permission, risk, idempotency, audit and asynchronous-operation metadata.
- Ensure handlers receive normalized identity and tenant context but cannot identify the caller transport.
- Verify focused tests and then `go test ./...`.

## Task 3: Implement API and Agent CLI Projections

**Files:** `internal/api/`, `internal/cli/`, adapter parity tests.

- Expose a functional API invocation endpoint that normalizes into the shared request envelope.
- Implement CLI JSON/JSON Lines input and output only; a normal error is structured JSON plus a non-zero exit code.
- Test no-TTY operation by launching the binary as a child process with JSON stdin; stdout must contain only declared result records and diagnostics only go to stderr.
- Run same-input API/CLI parity tests for success and each stable failure case.

## Task 4: Implement MCP Projections

**Files:** `internal/mcp/`, MCP parity and child-process tests.

- Project MCP tool metadata from the registry and map it back to the identical capability ID.
- Run stdio MCP with stdout reserved exclusively for JSON-RPC and stderr for diagnostics.
- Add in-process and child-process tests that compare MCP results against direct invocation/API/CLI results and detect any stdout pollution.

## Task 5: Produce Delivery and Compliance Evidence

**Files:** build/release configuration, SBOM generator configuration, `THIRD_PARTY_NOTICES`, provenance and signature verification tests.

- Generate SPDX or CycloneDX SBOM, dependency/license inventory, notices, SHA-256 checksums, build provenance and detached signatures for every artifact.
- Reject dependencies outside the ADR-007 initial license allowlist unless an approved compliance exception is present.
- Cross-build the declared platforms and verify the resulting hashes and build metadata as JSON.

## Task 6: Validate PostgreSQL 16.13 Phase 0 Baseline

**Files:** a dedicated PostgreSQL feature spec, reproducible Docker/benchmark manifest, schema and benchmark test files, `.claw/test-report.md`.

- Freeze the 8 GiB and 16 GiB Docker-memory profiles, CPU limit, volume type/IOPS, PostgreSQL parameters, record width, read/write mix and hot-tenant distribution.
- Create the 128 `tenant_bucket` LIST partitions, tenant RLS policies and connection-pool context reset tests against Docker PostgreSQL 16.13.
- Populate 1,000,000 business records and run the documented 50-concurrent/200-active-user workload for each memory profile.
- Record p50/p95/p99 latency, error rate, CPU, memory, disk, pool saturation, queue depth and tenant fairness. Ordinary read/write P95 targets are 300 ms/500 ms.
- Do not add HA, WAL archiving, backup or recovery testing under this Phase 0 task.

## Task 7: Independent Verification and Handoff

- The independent verifier reruns the API/MCP/CLI parity suite, no-TTY CLI test, stdout-purity test, cross-build and compliance checks without changing implementation.
- Record real commands, tool versions, outputs, benchmark manifests and unresolved limitations in `.claw/test-report.md` and `current-status.md`.
- Update the task board and loop state with an evidence-backed result. Do not push, publish, deploy or expand the L2 allowlist without separate user authorization.

## Exit Criteria

- CC-01 through CC-09 in `FEAT-020` have passing, reproducible Go 1.26.5 evidence.
- The release binary can serve API, MCP and JSON/JSON Lines CLI from one registry and invocation layer.
- The binary, not Node/npm/node_modules, is the Agent-host delivery unit and has required supply-chain artifacts.
- Both PostgreSQL memory profiles have recorded results under the accepted dataset and concurrency model, without any claim of HA/DR capability.
