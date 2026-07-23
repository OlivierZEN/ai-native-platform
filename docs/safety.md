# Loop Safety Policy — AI-Native Platform

## Status and Scope

This policy governs the `phase0-capability-contract` loop described in `LOOP.md`. It applies to every automated or manually resumed loop run during the five-hour bootstrap window.

The active level is L1 report-only. An L1 run may inspect repository state and update only the durable loop, planning, ADR, and `.claw` records explicitly allowed by `loop-constraints.md`.

## Absolute Denylist

Without a fresh, explicit user authorization, a loop must not:

- push, merge, rebase, open or update a pull request, tag, release, publish, or otherwise write to remote Git;
- send messages, create external tickets, alter cloud resources, or call production services;
- read, print, create, rotate, or modify credentials, secrets, environment files, or access policies;
- edit application source, tests, package manifests, lockfiles, CI, deployment, or infrastructure files while L1 is active;
- install project dependencies or use a generated artifact as evidence that an application capability works;
- auto-approve a capability, deployment, security decision, or L2 promotion.

## Required Checks Before a Run

1. Read `STATE.md`, `loop-constraints.md`, `loop-budget.md`, and the current `.claw` state.
2. Stop with no changes if `Pause: true`, the time window has ended, or the requested action is outside L1 scope.
3. Select exactly one smallest verifiable item; use at most three attempts to collect evidence for it.
4. Record only command-backed observations. A failed or unavailable command is evidence of a gap, never proof of success.

## Stall and Escalation Rule

A run is stalled when it produces the same unresolved finding as the immediately preceding run and has no new command evidence or safe documentation delta.

- On the first occurrence, record the finding and identify one different safe evidence source for the next run.
- On the second consecutive occurrence, record `stalled: true` in the run log and add the item to `STATE.md` under **Escalations**.
- On the third consecutive occurrence, set `Pause: true`; do not retry automatically. State the exact missing authority, evidence, or external change required to resume.

Any conflict between project specifications, an attempted source edit during L1, or a request for secret/remote/infrastructure access is an immediate escalation and pause condition.

## Evidence and Handoff

- Append one JSON object to `loop-run-log.md` for every run, including no-op or paused outcomes.
- Keep `STATE.md` and `.claw/current-status.md` concise and current.
- An independent verifier must review any future L2 implementation; the author of a change cannot self-approve it.
- At the window end, write a factual handoff: completed evidence, unresolved risks, L2 gate status, and the smallest authorized next action.

## Relationship to Product Invariants

This safety policy does not weaken the product contract: there is no graphical interface, and every released atomic capability must retain equivalent API, MCP Tool, and non-interactive CLI access through one Capability Contract.
