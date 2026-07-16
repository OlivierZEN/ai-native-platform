# Loop Constraints — AI-Native Platform

These rules are binding for every Loop Engineering run.

## Push and External State

- Do not push, merge, create a pull request, publish a package, send external messages, or change remote Git state without explicit user approval.
- Do not create or alter cloud resources, infrastructure configuration, credentials, secrets, or environment files.

## L1 Scope

- Until the L2 promotion gate in `LOOP.md` is satisfied, do not edit `src/`, `tests/`, `package.json`, lockfiles, CI workflows, build configuration, deployment files, or dependencies.
- L1 may edit only `LOOP.md`, `STATE.md`, `loop-constraints.md`, `loop-budget.md`, `loop-run-log.md`, `.claw/`, and directly relevant planning or ADR documents.
- One triaged item per run; no more than three evidence-gathering attempts for the same failed command before escalation.

## Product Invariants

- The product has no Web, mobile, BFF, graphical page, or interactive CLI.
- Every published atomic capability must expose equivalent API, MCP Tool, and non-interactive CLI access from one Capability Contract.
- Do not invent validation, build, benchmark, security, cost, or elapsed-time results.

## Communication

- State what will be done before a material change.
- Record only a concise state delta and append a machine-readable run record after each run.
- Escalate ambiguous product or security decisions instead of guessing.
