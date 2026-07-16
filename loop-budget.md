# Loop Budget — AI-Native Platform

## Window

- Start: 2026-07-16T16:02:28Z
- End: 2026-07-16T21:02:28Z
- Maximum scheduled runs: 10, plus the initial bootstrap run
- Run cadence: 30 minutes

## Limits

- Level: L1 report-only
- Active work items per run: 1
- Subagents per run: 0
- Evidence-command attempts per item: 3
- Source-code edits per run: 0
- Remote write actions per run: 0

## Pause Conditions

- `STATE.md` sets `Pause: true`.
- The window ends.
- A command would require secrets, infrastructure, remote write access, or an unapproved L2 code change.
- The same ambiguity or failed evidence command reaches the third attempt.

## Cost Observation

The environment does not expose a reliable per-run model-token meter or an end-to-end run timer. The loop therefore limits cadence, items, attempts, and subagent count mechanically. `duration_s` is `null` unless a complete run duration was measured; command outcomes remain the source of truth. It must not fabricate token, currency, or elapsed-time usage.
