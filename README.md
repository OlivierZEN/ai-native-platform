# CloudCC Semattice

**CloudCC Semattice（语义格）** is a greenfield AI-native multi-tenant business data and semantic runtime. Its category descriptor is **Agentic Business Data Runtime**, and its Chinese positioning is **面向智能体的业务数据与语义运行底座**. The architecture baseline is maintained in `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`; the approved extension of the existing Agent CC operations client into a product-neutral unified tenant control plane is defined in `docs/specs/FEAT-011-unified-tenant-operations-control-plane.md` and ADR-009 in `.claw/decisions.md`.

`Semattice` is the project-created brand blend `Sema(ntic) + (La)ttice`: semantic metadata, tenant data, relationships, policy, and atomic capabilities form a governed business lattice. ADR-012 in `.claw/decisions.md` is the naming source of truth. Existing protocol identifiers such as `native_platform`, the `ai-native-platform` compatibility binary, `AI_NATIVE_*` environment variables, and the current Go module path remain unchanged until a separately approved compatibility migration is implemented.

The Phase 0 Go implementation is a headless agent platform: every published atomic capability is projected from one registry/invoker into a functional HTTP API, authenticated MCP Tool, and non-interactive JSON CLI. The current core includes:

- Go 1.26.5, structured `slog`, env-only configuration, pgxpool, and checksum-protected explicit migrations.
- PostgreSQL 16 control tables, 128 tenant buckets, FORCE RLS, typed transaction-scoped TenantContext, and same-tenant relation constraints.
- Native tenant provisioning/lifecycle projection with persistent operations/audit and a versioned unified-operations port.
- UUIDv7 metadata versions, objects, fields, relations, immutable publication, and deterministic SHA-256 snapshots.
- Metadata-driven business-record create/get/update/delete/query, optimistic revisions, soft delete, durable write idempotency, relation edges, and bucket-partitioned typed indexes for bounded Agent queries.
- Changeset validation/simulation/approval/activation/cancel/status/restricted rollback, candidate-version write projection, resumable backfill and coverage gates, unique/reference validation, purge/tombstone, and versioned JSONB/field/index governance limits.

With database roles configured, the registry currently publishes 49 atomic capabilities. Each is projected to the functional API, authenticated MCP Tool, and non-interactive JSON CLI from the same definition. MCP is available both as local `mcp stdio` and, when the service is running, as an authenticated Streamable HTTP endpoint at `/mcp`; the existing capability API remains under `/v1/capabilities/*`.

Local verification uses an isolated PostgreSQL container bound only to `127.0.0.1:55432`:

```sh
./scripts/test-postgres.sh run
GOTOOLCHAIN=go1.26.5 go test -race ./...
GOTOOLCHAIN=go1.26.5 go vet ./...
```

Database changes are never applied implicitly. `AI_NATIVE_DATABASE_URL` is migrator-only and is used by the current `ai-native-platform db migrate` compatibility command. Application mode requires both `AI_NATIVE_CONTROL_DATABASE_URL` (dedicated control role) and `AI_NATIVE_RUNTIME_DATABASE_URL` (strict TenantContext runtime role); one URL is never reused for both trust domains. HTTP service mode and Streamable HTTP MCP require issuer/audience/algorithm/key verification configuration; MCP stdio and agent CLI bind the process to `AI_NATIVE_IDENTITY_TOKEN`. No interactive menu or prompt is provided. The target branded CLI command is `semattice`; introducing it without breaking existing automation requires a separate versioned migration.

<!-- agentic-project-guidelines:begin -->
## AI Development Protocol

This project follows the `agentic-project-guidelines` skill.

- All AI agents must automatically use this skill before project analysis, planning, implementation, testing, or maintenance.
- If the current agent does not have this skill installed, install it first from GitHub: https://github.com/CloudCCAI/agentic-project-guidelines
- When the environment supports `AGENTS.md`, treat both this README section and `AGENTS.md` as mandatory project instructions.

本项目遵循 `agentic-project-guidelines` 技能规范。

- 所有 AI 智能体在进行项目分析、规划、编码、测试和维护前，必须自动使用此技能。
- 如果当前智能体尚未安装此技能，必须先从 GitHub 安装：https://github.com/CloudCCAI/agentic-project-guidelines
- 如果当前环境支持 `AGENTS.md`，应将 README 中的本节和 `AGENTS.md` 同时视为必须遵守的项目级指令。
<!-- agentic-project-guidelines:end -->
