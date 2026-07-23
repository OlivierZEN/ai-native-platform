---
kind: feature-spec
id: FEAT-020-DEPENDENCY-GATE
title: Go Capability Contract PoC Dependency Gate
status: superseded
superseded_by: FEAT-010
owner_role: shared
related_tasks: [TASK-020]
related_adrs: [ADR-007]
updated_at: 2026-07-17T16:35:47Z
---

# Go Capability Contract PoC Dependency Gate

## Scope

This record governs only the first L2 PoC dependency set. It does not authorize a production release, an internal module proxy configuration, or a broader dependency catalogue.

This is a historical snapshot of TASK-020 before PostgreSQL, identity and UUID dependencies were added. The current complete module graph and license gate are owned by `FEAT-010`; none of the counts or “no additional module” statements below describe the current repository.

## Direct dependency candidate

| Module | Version | Purpose | License review | Decision |
|---|---:|---|---|---|
| `github.com/modelcontextprotocol/go-sdk` | `v1.6.1` | Official Go Tier 1 MCP server/client SDK; stdio Tool projection | Repository license describes an Apache-2.0/MIT transition. The code licenses are within ADR-007's initial allowlist; SDK documentation is CC-BY-4.0 and is not incorporated into this binary. | Allowed for this PoC |

## Resolved module graph and license review

The following exact `GOTOOLCHAIN=go1.26.5 go list -m all` graph was reviewed from local module-cache license files on 2026-07-18. All code licenses are within ADR-007's MIT/BSD-2-Clause/BSD-3-Clause/Apache-2.0/ISC allowlist.

| Module | Version | Classification | Decision |
|---|---:|---|---|
| `cloud.google.com/go/compute/metadata` | `v0.3.0` | Apache-2.0 | allowed |
| `github.com/golang-jwt/jwt/v5` | `v5.3.1` | MIT | allowed |
| `github.com/google/go-cmp` | `v0.7.0` | BSD-3-Clause | allowed (test-only) |
| `github.com/google/jsonschema-go` | `v0.4.3` | MIT | allowed (MCP SDK transitive dependency) |
| `github.com/modelcontextprotocol/go-sdk` | `v1.6.1` | Apache-2.0/MIT transition | allowed; retain notices in a future release artifact |
| `github.com/segmentio/asm` | `v1.1.3` | MIT | allowed |
| `github.com/segmentio/encoding` | `v0.5.4` | MIT | allowed |
| `github.com/yosida95/uritemplate/v3` | `v3.0.2` | BSD-3-Clause | allowed |
| `golang.org/x/oauth2` | `v0.35.0` | BSD-3-Clause | allowed |
| `golang.org/x/sys` | `v0.41.0` | BSD-3-Clause | allowed |
| `golang.org/x/tools` | `v0.42.0` | BSD-3-Clause | allowed (test/build tooling) |

## Required evidence before MCP completion

1. `go.sum` contains checksums for the resolved module graph: satisfied.
2. Every direct and transitive module has a recorded license classification within ADR-007's allowlist: satisfied for this PoC.
3. `GOTOOLCHAIN=go1.26.5 go mod verify`: passed (`all modules verified`).
4. A release later emits the SBOM, notices, hashes, provenance and signature required by ADR-007. These release artifacts are not claimed by this PoC gate, and production CI/release must use an approved internal module proxy rather than the local public proxy.

## Source evidence

- MCP official SDK listing: Go is Tier 1.
- Go SDK `v1.6.1` license and module manifest reviewed on 2026-07-18.
- Local Go 1.26.5 toolchain verified with `GOTOOLCHAIN=go1.26.5 go version`.
- At the time of TASK-020, no additional module was introduced beyond the SDK's resolved graph. This historical statement was superseded when TASK-010/011/012/013 added approved dependencies.
