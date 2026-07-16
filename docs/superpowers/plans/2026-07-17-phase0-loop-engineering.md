# Phase 0 Loop Engineering Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Within five elapsed hours, establish a safe L1 Loop Engineering control system, collect the evidence needed for the Phase 0 runtime decision, and complete the verified implementation plan for a TypeScript Capability Contract proof of concept.

**Architecture:** A durable loop layer (`LOOP.md`, `STATE.md`, constraints, budget, and append-only run log) controls iteration and human gates. The L2 PoC design uses one capability registry and one invocation layer; HTTP API, MCP, and CLI are transport adapters only, so authorization, idempotency, error handling, audit records, and business results cannot drift.

**Tech Stack:** Node.js 24 LTS in CI and production, TypeScript with ESM, `@modelcontextprotocol/sdk`, Zod, Node `http`, and Vitest. The local Node 26 installation may run development checks but does not change the production LTS baseline.

## Global Constraints

- No Web, mobile, BFF, graphical page, interactive terminal prompt, or menu.
- Every published atomic capability must expose API, MCP Tool, and non-interactive CLI entry points with equivalent JSON schemas and semantics.
- STDIO MCP servers write protocol messages only to stdout; diagnostics go to stderr.
- CLI accepts flags or JSON on stdin and emits JSON or JSON Lines; high-risk approval is an asynchronous state, not a prompt.
- Never push, merge, modify secrets, or change infrastructure without explicit user approval.
- Run the narrowest relevant test before and after each code change; no more than three attempts per failed item.
- Automation starts L1 report-only and reaches L2 only for allowlisted Phase 0 files with real verification evidence.

---

## Five-Hour Delivery Schedule

| Elapsed window | Deliverable | Gate |
|---|---|---|
| 00:00–00:35 | Loop control files, L1 automation, initial state and run log | Constraints and budget exist; no source code changed |
| 00:35–01:00 | `loop-audit` baseline and stack evidence | Audit result recorded; Node 24 LTS and TypeScript/MCP evidence captured |
| 01:00–01:40 | ADR-005 proposal and L2 implementation-plan review | Runtime recommendation and explicit non-goals recorded; no code created |
| 01:40–04:30 | Repeated L1 triage, state validation, scope/risk review, and evidence logging | Each run either records an evidence-backed finding or exits report-only |
| 04:30–05:00 | Final audit, handoff, commit, and human-review report | No push; L2 activation remains a human decision |

## Task 1: Establish Loop Engineering Controls

**Files:**
- Create: `LOOP.md`
- Create: `STATE.md`
- Create: `loop-constraints.md`
- Create: `loop-budget.md`
- Create: `loop-run-log.md`
- Modify: `.claw/current-status.md`
- Modify: `.claw/task-board.md`

**Interfaces:**
- Consumes: `.claw/current-status.md`, `.claw/task-board.md`, `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`, and `docs/specs/FEAT-020-pure-agent-capability-contract.md`.
- Produces: a durable loop state read before each automation run, a scoped allowlist, a fixed five-hour stop condition, and an append-only run record.

- [ ] **Step 1: Create the L1 loop documents.**

  Define the loop purpose as Phase 0 capability-contract delivery, explicitly exclude pushes, merges, secrets, infrastructure, and unrelated refactors, and list `src/`, `tests/`, `package.json`, `tsconfig.json`, `.claw/`, and `docs/` as the only L2-eligible paths.

- [ ] **Step 2: Configure budget and stop behavior.**

  Set a five-hour window, a maximum of one active item per run, three failed attempts per item, and report-only behavior when the budget reaches 80 percent or the pause flag is active.

- [ ] **Step 3: Record the initial run.**

  Append a JSON Lines entry with a real start timestamp, `pattern` set to `phase0-capability-contract`, zero source actions, and `outcome` set to `started`.

- [ ] **Step 4: Validate governance state.**

  Run: `python3 /Users/owenmacbook/.agents/skills/y-agentic-project-guidelines/scripts/validate-state.py .claw`

  Expected: `State validation passed for: .claw`.

## Task 2: Establish the Phase 0 Runtime Decision

**Files:**
- Modify: `.claw/decisions.md`
- Modify: `.claw/devops.md`
- Modify: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`

**Interfaces:**
- Consumes: Node.js release policy, MCP TypeScript server guidance, and FEAT-020's three-entry-point contract.
- Produces: ADR-005 declaring Node.js 24 LTS + TypeScript/ESM as the isolated Phase 0 capability-contract PoC runtime.

- [ ] **Step 1: Record decision evidence.**

  Record that Node.js projects should use Active or Maintenance LTS for production, that Node 24 is LTS, and that the MCP SDK's TypeScript guide supports Node, ESM, Zod-backed tool schemas, and STDIO transport.

- [ ] **Step 2: Write ADR-005.**

  Define Node 24 LTS + TypeScript/ESM as the Phase 0 PoC runtime only; explicitly leave the final multi-service CRM runtime subject to later capacity and operations evidence.

- [ ] **Step 3: Update the active task.**

  Move `TASK-010` to `in_progress`, set its immediate scope to this PoC, and retain `TASK-020` as dependent contract work.

## Future L2 Task 3: Bootstrap the Capability Contract Test Harness

> **Not executed in this five-hour L1 window.** Execute only after the loop has passed the L1 observation period and a human explicitly authorizes the L2 allowlist.

**Files:**
- Create: `package.json`
- Create: `tsconfig.json`
- Create: `src/contracts/types.ts`
- Create: `src/contracts/registry.ts`
- Create: `src/runtime/invoke.ts`
- Create: `tests/contracts/registry.test.ts`
- Create: `tests/runtime/invoke.test.ts`

**Interfaces:**
- Produces: `CapabilityDefinition`, `CapabilityRequest`, `CapabilityResult`, `CapabilityRegistry`, and `invokeCapability`.
- `CapabilityDefinition` has `id`, `version`, `inputSchema`, `riskLevel`, `idempotency`, and `handler`.
- `invokeCapability(registry, request)` returns a JSON-safe success or error envelope containing `capabilityId`, `requestId`, `auditId`, and `result` or stable `error`.

- [ ] **Step 1: Write failing unit tests.**

  Test registration and lookup of `system.capability.list`; test that malformed input yields `VALIDATION_FAILED`; test that an unknown ID yields `CAPABILITY_NOT_FOUND`; and test that each result contains a non-empty request and audit ID.

- [ ] **Step 2: Run the focused tests.**

  Run: `npm test -- tests/contracts/registry.test.ts tests/runtime/invoke.test.ts`

  Expected: failure because the package scripts and implementation files do not exist.

- [ ] **Step 3: Implement the smallest registry and invocation layer.**

  Use Zod for input validation, generate IDs with `crypto.randomUUID()`, and keep audit events in an in-memory sink for the PoC.

- [ ] **Step 4: Re-run the focused tests.**

  Run: `npm test -- tests/contracts/registry.test.ts tests/runtime/invoke.test.ts`

  Expected: all focused tests pass.

## Future L2 Task 4: Add HTTP API and Non-Interactive CLI Adapters

> **Not executed in this five-hour L1 window.**

**Files:**
- Create: `src/api/server.ts`
- Create: `src/cli/main.ts`
- Create: `tests/adapters/api-cli-parity.test.ts`
- Modify: `package.json`

**Interfaces:**
- API: `POST /v1/capabilities/:id/invoke` accepts a `CapabilityRequest` JSON body and returns the `CapabilityResult` JSON envelope.
- CLI: `agent-cli capability invoke <id> --input - --output json` reads one JSON object from stdin and writes one `CapabilityResult` JSON object to stdout.
- Both call `invokeCapability` directly and never reimplement validation, authorization, idempotency, or audit behavior.

- [ ] **Step 1: Write failing parity tests.**

  Submit the same `system.capability.list` input through the HTTP handler and CLI runner, then assert equal capability ID, status, error code, and result payload shape.

- [ ] **Step 2: Run the parity tests.**

  Run: `npm test -- tests/adapters/api-cli-parity.test.ts`

  Expected: failure because neither adapter exists.

- [ ] **Step 3: Implement the two adapters.**

  Return HTTP `400` for `VALIDATION_FAILED`, `404` for `CAPABILITY_NOT_FOUND`, and `200` for a successful invocation. Keep CLI diagnostics on stderr and produce JSON only on stdout.

- [ ] **Step 4: Re-run parity tests.**

  Run: `npm test -- tests/adapters/api-cli-parity.test.ts`

  Expected: all parity tests pass with no interactive input.

## Future L2 Task 5: Add the STDIO MCP Adapter

> **Not executed in this five-hour L1 window.**

**Files:**
- Create: `src/mcp/server.ts`
- Create: `tests/adapters/mcp-parity.test.ts`
- Modify: `package.json`

**Interfaces:**
- MCP Tool name: `system_capability_list` maps to capability ID `system.capability.list`.
- MCP Tool handler passes its parsed input into `invokeCapability` and serializes the same `CapabilityResult` envelope as JSON text content.
- STDIO startup and diagnostics use stderr; stdout is reserved for JSON-RPC.

- [ ] **Step 1: Write a failing MCP parity test.**

  Invoke `system_capability_list` through the in-process MCP server and assert that its parsed result equals the direct invocation result for the same request ID.

- [ ] **Step 2: Run the MCP test.**

  Run: `npm test -- tests/adapters/mcp-parity.test.ts`

  Expected: failure because the server module does not exist.

- [ ] **Step 3: Implement the MCP server.**

  Register the tool from the same registry data and connect through `StdioServerTransport`; do not write logs with `console.log`.

- [ ] **Step 4: Re-run the MCP test.**

  Run: `npm test -- tests/adapters/mcp-parity.test.ts`

  Expected: all MCP parity assertions pass.

## Future L2 Task 6: Verify, Audit, and Hand Off

> **Not executed in this five-hour L1 window.** The L1 window only validates the governance state and the loop audit.

**Files:**
- Modify: `STATE.md`
- Modify: `loop-run-log.md`
- Modify: `.claw/current-status.md`
- Modify: `.claw/task-board.md`
- Modify: `.claw/test-report.md`

**Interfaces:**
- Consumes: all test results, build output, and loop readiness audit.
- Produces: a single evidence-backed handoff with remaining Phase 0 risks.

- [ ] **Step 1: Run all code checks.**

  Run: `npm test && npm run build`

  Expected: all tests pass and TypeScript writes a production build without diagnostics.

- [ ] **Step 2: Audit loop readiness.**

  Run: `npx @cobusgreyling/loop-audit . --suggest`

  Expected: report a score and concrete improvements; record the exact score rather than claiming a target score.

- [ ] **Step 3: Record evidence and stop condition.**

  Update state with the real commands, results, commit hashes, unresolved choices, and whether five elapsed hours have ended. Do not push or merge.

## Plan Self-Review

- Spec coverage: FEAT-020's common contract, API/MCP/CLI parity, non-interactive CLI, and contract testing map to Tasks 3–5; Loop Engineering state, budget, safety, and observability map to Tasks 1 and 6.
- Scope: the L2 plan deliberately delivers one capability (`system.capability.list`) as a verified vertical slice; the five-hour L1 window delivers its control plane and evidence only. CRM record, metadata, and Changeset handlers remain separate Phase 0 tasks.
- Consistency: all three adapters depend on the same `invokeCapability` function and JSON-safe `CapabilityResult` envelope.
- No unattended production change: the five-hour automation is bounded, L1 first, has no push/merge permission, and stops at the defined deadline.
