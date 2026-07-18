# L2 Loop Budget — Go Capability Contract PoC

- Window: 2026-07-18 00:03:53–05:03:53 Asia/Shanghai (hard stop).
- Max active work item: 1.
- Max failed attempts per item: 3.
- Max dependency additions: 2, each with license and transitive-dependency evidence.
- Bootstrap checkpoint: 13 source files were recorded as a one-time, completed initial repository baseline. This exception is authorized only by the active user Goal to continue the bounded L2 PoC; it does not expand the path allowlist or delivery scope.
- Max source files changed before each subsequent checkpoint: 8.
- Max verifier agents: 1; the verifier must not modify implementation files.
- Remote write, publish and auto-merge budget: 0.
- At 80% elapsed window: stop adding scope and reserve time for verification and handoff.
- On a denylist violation, repeated failure, or incomplete dependency/security evidence: set `STATE.md` pause to `true`, append an escalation record, and stop source changes. A process-only budget exception requires a recorded checkpoint and the active user Goal must still authorize the same bounded work.
