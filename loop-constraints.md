# L2 Loop Constraints — Go Capability Contract PoC

## Allowed paths

- `go.mod`, `go.sum`
- `cmd/ai-native-platform/**`
- `internal/capability/**`, `internal/api/**`, `internal/cli/**`, `internal/mcp/**`
- `schemas/**`, `docs/**`, `.claw/**`
- `LOOP.md`, `STATE.md`, `loop-constraints.md`, `loop-budget.md`, `loop-run-log.md`

## Denylist and human gates

- Never edit `.env`, `.env.*`, `**/secrets/**`, `**/credentials/**`, `**/*_key*`, `**/*_secret*`, `auth/**`, `payments/**`, `billing/**`, `k8s/**`, `.terraform/**`, `Dockerfile*`, `docker-compose*`, `.github/**`, `**/migrations/**`, or any production configuration.
- Never access or write a production database; this loop does not create PostgreSQL schema, migrations, containers, volumes, HA, backup, or recovery artifacts.
- Never push, merge, open a PR, publish a module/binary/image, send external messages, install system packages, or store credentials.
- A dependency may enter `go.mod` only after recording its exact version, direct/transitive license review, checksum behavior and reason for use in the active task handoff.
- At most three failed attempts per work item. Do not disable tests, inflate timeouts, or replace a protocol dependency with an incompatible custom protocol merely to pass a test.
- Stop and escalate on scope ambiguity, a denylist path, a non-allowlisted license, any credential/PII exposure, public module resolution required for CI/release, or a missing independent verifier.
