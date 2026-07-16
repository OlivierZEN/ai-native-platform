# LOOP.md — AI-Native Platform

## Purpose

Run a bounded Phase 0 Loop Engineering program that makes project state, safety constraints, evidence, and handoff decisions durable. The current loop is not an implementation loop: it establishes and observes the control system needed before the Capability Contract PoC can enter an assisted L2 implementation phase.

## Active Loop

- Pattern: `phase0-capability-contract`
- Level: `L1 — report-only`
- Window: 2026-07-17 00:02–05:02 Asia/Shanghai
- Cadence: every 30 minutes, plus this initial run
- State: `STATE.md`, `.claw/current-status.md`, `.claw/task-board.md`, and `loop-run-log.md`
- Machine-readable registry: `patterns/registry.yaml`
- Primary work: `TASK-021`; watched delivery tasks: `TASK-009`, `TASK-010`, and `TASK-020`

## Run Procedure

1. Stop immediately if `STATE.md` has `pause: true` or the end time has passed.
2. Read `loop-constraints.md`, `loop-budget.md`, `docs/safety.md`, `STATE.md`, `.claw/current-status.md`, `.claw/task-board.md`, `FEAT-009`, and `FEAT-020`.
3. Triage only one smallest verifiable Phase 0 item.
4. In this L1 window, do not change application source code, package manifests, dependencies, CI, infrastructure, secrets, or remote Git state.
5. Run read-only evidence commands only. Record each result in `STATE.md` and append one JSON object to `loop-run-log.md`.
6. Escalate ambiguity, conflicting specifications, or any request to enter L2 before the observation period has completed.

## L2 Promotion Gate

The loop remains L1 until all conditions are true:

- At least seven calendar days of L1 runs have stable state updates and no safety violations.
- The user explicitly approves the L2 path allowlist and the Node 24 LTS + TypeScript PoC ADR.
- `TASK-010` has a verified build/test toolchain and `TASK-020` has an approved implementation plan.
- A verifier independent of the implementer is available. Until then, no loop may self-approve an implementation result.

## Safety and Human Gates

- No push, merge, pull request, release, external message, secret change, infrastructure change, or auto-merge.
- No subagents in this project loop.
- A run changes only durable state, budget, log, planning, or ADR documents that are directly in scope.
- Never claim test, elapsed-time, cost, or implementation results without command evidence.
- Follow `docs/safety.md`; repeated no-progress outcomes must escalate and pause rather than loop indefinitely.
- This L1 pattern has no MCP connector access. It uses the shared main worktree only for documentation; L2 source work requires an isolated worktree, as declared in `patterns/registry.yaml`.

## Completion

At the end of the five-hour window, the loop writes a factual handoff in `STATE.md` and `.claw/current-status.md`, appends a final run log entry, and pauses. Code work remains a user-approved L2 follow-up.
