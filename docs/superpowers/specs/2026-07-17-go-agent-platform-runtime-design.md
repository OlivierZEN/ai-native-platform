# Go Agent Platform Runtime Design

## Status

Architecture direction approved in conversation on 2026-07-17. This document is awaiting written-spec review before it drives the ADR, Phase 0 plan, or source implementation.

## Purpose

Make the pure AI Native platform deployable in domestic, private, and restricted-network environments without requiring an agent host to install Node.js, npm, or a `node_modules` tree. The same runtime must expose every published atomic capability as a functional API, MCP Tool, and non-interactive CLI while preserving the single Capability Contract required by ADR-004.

## Context and Constraints

- No Web, mobile, BFF, management page, or human-interactive terminal UI is in scope.
- API, MCP, and CLI must share schemas, authorization, tenant context, quotas, risks, idempotency, stable errors, audit semantics, and domain handler behavior.
- Target agent environments may have poor or prohibited access to public package registries. They must never install dependencies or self-update at invocation time.
- The platform must support high concurrent I/O workloads without letting a tenant, a slow downstream system, or a CPU-bound task monopolize request processing.
- Every distributed binary or image must have traceable inputs, a software bill of materials, license notices, hashes, and signatures.
- This decision selects the Phase 0 runtime direction. It does not authorize source creation, L2 work, deployment, public release, or a change to the proposed CRM data architecture.

## Approaches Considered

### 1. Node.js and TypeScript as the production runtime

This gives rapid JSON-schema and MCP development but leaves Node.js, npm, transitive packages, and possible native modules in every target delivery environment. It does not meet the distribution constraint and is rejected for production delivery.

### 2. Java as the production runtime

Java has mature enterprise libraries and virtual-thread support, but requires controlled JDK/JRE distribution or a runtime image. It adds runtime-distribution and JDK-license lifecycle choices that are unnecessary for the first Go-first platform slice. It remains a future option only when a proven JVM-specific requirement justifies it.

### 3. Go single-runtime delivery model (selected)

One Go module builds a signed binary for each supported operating-system and CPU architecture. The binary can run the API server, an MCP server, or the Agent CLI. These are transport modes around one registry and one invocation layer, not three implementations. This directly satisfies the distribution constraint while retaining an official Tier 1 MCP SDK option.

## Selected Architecture

### Runtime and commands

Phase 0 uses Go 1.26.5 in CI and release builds. The local Go 1.26.4 installation is not release evidence and must be upgraded or matched by the approved L2 toolchain before code verification.

The release binary is named `ai-native-platform` and has only non-interactive commands:

```text
ai-native-platform serve --listen <address>
ai-native-platform mcp stdio
ai-native-platform capability list
ai-native-platform capability describe --id <capability-id>
ai-native-platform capability invoke --id <capability-id> --input-json <json>
```

`capability invoke` also accepts one JSON request on standard input. Normal results are JSON or JSON Lines on standard output. Errors are structured JSON with a non-zero exit code. It never prompts, opens a menu, mutates a terminal session, or treats a local TTY as evidence of approval.

`mcp stdio` reserves standard output exclusively for MCP JSON-RPC messages. Logs, progress, and diagnostics go to standard error. Remote MCP transport is HTTP-based and uses the same invocation layer.

### Capability execution path

```text
REST request / MCP tool call / CLI JSON invocation
  -> transport normalization
  -> Capability Registry lookup and JSON Schema validation
  -> identity, tenant, permission, quota, risk, and idempotency checks
  -> shared domain handler
  -> audit event + synchronous result or operation_id
```

The registry is the only publication source for capability ID, version, JSON Schema, permissions, risk, idempotency policy, stable error vocabulary, audit event names, and asynchronous behavior. API routing, MCP tool metadata, and CLI discovery are projections of that registry. A handler cannot determine its caller's transport.

The canonical contract format is versioned JSON Schema Draft 2020-12 documents stored as JSON. The exact Go validator dependency is selected only after its version, license, maintenance status, and generated SBOM entry pass the Phase 0 dependency gate; the contract format itself must not be coupled to a Go-library-specific type system.

### Concurrency and workload isolation

The Go runtime serves short API, MCP, and CLI invocations concurrently. It must enforce bounded concurrency rather than create unbounded goroutines:

- per-tenant and global request quotas before handler execution;
- bounded worker pools for CPU-heavy validation, compilation, and serialization;
- context deadlines and cancellation propagated to database, AI, and connector calls;
- bounded database connection pools per shard rather than one connection per request;
- asynchronous operation records and an outbox/event path for long-running work;
- admission control, backpressure, circuit breakers, and stable overload errors for slow dependencies.

Language performance is not a capacity proof. Phase 0 must measure p50/p95/p99 latency, error rate, saturation, queue depth, memory, CPU, and tenant-fairness behavior under a documented concurrent workload before any throughput claim is made.

### Distribution model

The build pipeline produces platform-specific release artifacts for Linux amd64, Linux arm64, macOS arm64, and Windows amd64. A container image is an optional server deployment form, not a substitute for CLI/MCP binary distribution.

Pure-Go builds use `CGO_ENABLED=0` by default. Any exception that introduces C libraries must add a platform matrix, a vulnerability and license review, and release evidence before it can enter a distributed artifact.

Agent hosts download an approved version from the internal artifact channel, verify its signature and checksum, and execute it. They do not run `go get`, download modules, run package-manager commands, or self-update. The command can report its version and build metadata as JSON for machine verification.

### Supply-chain and license compliance

The Go toolchain's permissive license does not make the product automatically compliant. Each binary statically or dynamically incorporates a dependency set that must be governed.

- CI uses an internal Go module proxy and an allowlisted module source policy; direct public-registry resolution is disabled in release builds.
- `go.mod` and `go.sum` are locked inputs. A dependency change requires review, vulnerability scanning, license classification, and a reproducible-build record.
- Every release emits SPDX or CycloneDX SBOM, a machine-readable dependency/license inventory, `THIRD_PARTY_NOTICES`, build provenance, SHA-256 checksums, and a detached signature.
- The initial allowlist is MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, and ISC subject to notice obligations. Copyleft, source-available, field-of-use, and non-standard licenses are denied until legal review approves a specific dependency and distribution model.
- License policy applies equally to direct and transitive dependencies, embedded assets, container base images, and the Go toolchain distribution.

## Evidence and Test Strategy

Phase 0 starts with the low-risk `system.capability.list` vertical slice. It must prove all CC-01 through CC-09 requirements in FEAT-020, plus the Go-specific delivery checks below:

1. Registry, invocation, and parity tests prove API, MCP, and CLI use the same handler and return equal results and stable errors.
2. A non-TTY child-process test proves CLI JSON input/output and no prompt behavior.
3. An MCP stdio child-process test proves standard output contains no non-protocol diagnostics.
4. Cross-platform build checks prove the declared artifact matrix and record output hashes.
5. SBOM, license policy, notices, checksum, and signature checks are required release artifacts.
6. A bounded-concurrency benchmark publishes its workload, limits, measured p50/p95/p99 latency, error rate, queue depth, CPU, memory, and per-tenant fairness result. The benchmark cannot be replaced by an unmeasured language-performance claim.

## Explicit Boundaries

- No Node.js, npm, `node_modules`, npx, or runtime package installation is permitted in production API, MCP, or CLI delivery.
- No complete CRM microservice stack is selected by this document. PostgreSQL topology, event bus, search, OLAP, observability backend, and production capacity targets retain their existing decision status.
- No Java runtime is introduced in Phase 0.
- No code, dependency, CI workflow, container image, credential, or cloud resource is created by this design decision alone.

## Decision Consequences

ADR-005 must be revised from its Node.js proposal to an accepted Go 1.26.5 Phase 0 runtime decision. FEAT-020, the Phase 0 plan, Loop L2 prerequisites, and hot project state must replace Node-specific prerequisites with Go toolchain, binary-distribution, and compliance evidence. Historical Loop records remain unchanged because they accurately describe the earlier Node proposal at the time they were recorded.

## References

- Go release history: https://go.dev/doc/devel/release
- Go license: https://go.dev/LICENSE
- MCP official SDK tiers: https://modelcontextprotocol.io/docs/sdk
- Node event-loop guidance: https://nodejs.org/learn/asynchronous-work/dont-block-the-event-loop
- Java virtual-thread API: https://docs.oracle.com/en/java/javase/25/docs/api/java.base/java/lang/Thread.html
- OpenJDK GPLv2 with Classpath Exception: https://openjdk.org/legal/gplv2+ce.html
