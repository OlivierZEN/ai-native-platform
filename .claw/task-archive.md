---
kind: task-archive
version: 3
updated_at: 2026-08-05T06:27:00Z
updated_by: codex after archiving TASK-062
archive_status: active
---

# 任务归档

`task-archive.md` 保存从 `task-board.md` 中移出的已完成或已取消任务卡。

## Archived Tasks

### TASK-062 - Exchange commerce Keycloak tokens for Semattice OACT

- status: `done`
- owner_role: `integration-agent`
- spec_path: `docs/specs/FEAT-055-commerce-keycloak-token-exchange.md`
- handoff: Release `20260805T061911Z-web-oidc-66ab21f50d5f` is active. Human commerce clients are server-allowlisted, the responsible commerce service binding passed raw-token rejection, OACT exchange and real product query smoke, and rollback artifacts are recorded in FEAT-055 and devops.md.

### TASK-060 - Add current metadata discovery and publish Skill v1.5.0

- status: `done`
- owner_role: `integration-agent`
- spec_path: `docs/specs/FEAT-051-current-metadata-discovery.md`
- handoff: `metadata.version.get-current` 已在生产以低风险同步 `metadata.read` 能力发布，空输入返回当前 published Bundle；生产目录为 56 项，真实当前对象回读通过。Skill `v1.5.0` 的 GitHub main、不可移动标签和本机安装副本一致，发布提交为 `685687e71e41d2e08fd157e8fce9729b8d25c174`。

### TASK-023 - Add the platform capability matrix to the deployed guide

- status: `done`
- owner_role: `frontend-agent`
- spec_path: `docs/specs/FEAT-022-semattice-single-node-https-deployment.md`
- handoff: 官网首版能力矩阵以当时运行时 Registry 的 49 项 Capability 为准，分组为 `6 tenant + 6 semantic metadata + 10 changeset + 5 record runtime + 12 authorization + 9 sharing/organization + 1 system`；后续矩阵必须随正式发布能力同步更新并重复 parity 验证。

### TASK-022 - Deploy CloudCC Semattice to the authorized ECS

- status: `done`
- owner_role: `release-agent`
- spec_path: `docs/specs/FEAT-022-semattice-single-node-https-deployment.md`
- handoff: PostgreSQL 16.13、Nginx 1.30.2 和 Semattice systemd 已启用；公网 HTTPS/API、CLI 下载、短期 JWT、MCP stdio 与服务重启均验证通过。后续版本由不可变 release 延续，凭据不进入仓库。

### TASK-012 - Validate the PostgreSQL shard baseline

- status: `done`
- owner_role: `backend-agent`
- spec_path: `docs/specs/FEAT-012-postgresql-shard-isolation-poc.md`
- handoff: PostgreSQL 16.13、128 bucket、分区裁剪、FORCE RLS、事务级 TenantContext、连接池清理、跨租户关系拒绝及 runtime/control 最小权限已验证；完整容量验收由 TASK-019 承接。

### TASK-048 - Deploy tenant name in the console topbar

- status: `done`
- priority: `high`
- owner_role: `release-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `TASK-047`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `committed console static change, immutable Semattice release and production verification evidence`
- branch: `main`
- pr_url: `n/a`

#### Done When

- The verified TASK-047 change is committed without credentials.
- A new immutable release is active while the prior release and static/Nginx backups remain available.
- Public health and authentication negative checks pass.
- A real authenticated browser displays the overview `tenant_name` and no Keycloak subject in the topbar.

#### Next Action

- Completed. Future display changes require a new committed immutable release.

#### Handoff Note

- Commit`ffdbec4fada7aa0169d75dd785bac8607cf927b8`is active as`/opt/semattice/releases/20260731T080337Z-web-oidc-ffdbec4fada7`. Chrome showed“应用开发组织”in`#tenant-name`with zero console errors; rollback evidence passed.

### TASK-047 - Show tenant name in the console topbar

- status: `done`
- priority: `medium`
- owner_role: `frontend-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `TASK-040, TASK-046`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `console static HTML/JavaScript and focused verification evidence`
- branch: `main`
- pr_url: `n/a`

#### Done When

- The topbar displays the authenticated tenant's `tenant_name` from `/console/api/overview`.
- The Keycloak user UUID is no longer rendered in the topbar.
- Session, tenant isolation and other console pages remain unchanged.

#### Next Action

- Completed by TASK-048 production rollout.

#### Handoff Note

- The overview response is the tenant-name source of truth. The topbar falls back to“当前租户”and never renders the Keycloak subject. Production browser acceptance passed.

### TASK-046 - Deploy Semattice web Keycloak login

- status: `done`
- priority: `critical`
- owner_role: `release-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-045-production-web-oidc-rollout.md`
- depends_on: `TASK-040, TASK-043, TASK-045`
- blocked_by: `none`
- related_issues: `ISSUE-001`
- scope_files: `committed repository release, protected server secret/config, immutable Semattice release, static console, Nginx and production verification evidence`
- branch: `main`
- pr_url: `n/a`

#### Done When

- The complete verified worktree is committed without credentials.
- The existing `semattice-web` Client Secret is stored only in the protected server file and referenced by exact environment configuration.
- One immutable release from the committed SHA serves the combined console, CLI access context and web OIDC login.
- Service, public negative checks, real browser login and rollback evidence pass.

#### Next Action

- Completed. Keep the existing protected Secret file and exact callback URI when rotating or rebuilding the web client; use a new immutable release for future changes.

#### Handoff Note

- Commit `dcf2b811b7ec88d0685938f6d6564c818ba24314` was deployed as `/opt/semattice/releases/20260731T074549Z-web-oidc-dcf2b811b7ec`. Real Chrome login returned to the authenticated console with zero browser-console errors. The Secret was never regenerated or printed.

### TASK-045 - Add Keycloak login to the Semattice website

- status: `done`
- priority: `critical`
- owner_role: `fullstack-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-044-semattice-web-keycloak-login.md`
- depends_on: `TASK-040, TASK-042, TASK-044`
- blocked_by: `none`
- related_issues: `ISSUE-001`
- scope_files: `console OIDC routes and session, identity verifier, runtime configuration, nginx route, console login UI and tests`
- branch: `main`
- pr_url: `n/a`

#### Done When

- `/auth/oidc/login` starts Authorization Code + S256 PKCE without exposing secrets or tokens.
- `/auth/oidc/callback` validates state, nonce, Keycloak tokens and active Organization mapping before creating a secure console Cookie.
- The anonymous console shows a Keycloak login button while the existing CLI/OACT session exchange remains compatible.
- Local security, configuration, handler, frontend and full repository verification pass.

#### Next Action

- Local implementation is complete. A separate authorized release must create the protected Client Secret file, add the three `AI_NATIVE_CONSOLE_OIDC_*` settings, deploy one combined binary/static/Nginx release, and run a real browser smoke test.

#### Handoff Note

- `semattice-web` is a confidential server-side web client; `semattice-cli` remains an independent public loopback client. No production Secret was read or written and no deployment, commit or push occurred in TASK-045.

### TASK-044 - Enable all published capability scopes for skill login

- status: `done`
- priority: `critical`
- owner_role: `integration-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-043-skill-all-capability-scopes.md`
- depends_on: `TASK-042, TASK-043`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `skill login defaults, production OACT allowlist, authentication tests, deployment guidance and rollout evidence`
- branch: `main`
- pr_url: `n/a`

#### Done When

- Skill login and renewal request all unique scopes required by the 51 published Capability APIs.
- Production allowlist matches the same scope set without bypassing RBAC, RLS, approval or audit.
- Local tests and a read-only production login/capability discovery smoke pass.

#### Next Action

- Completed. Continue TASK-033 Principal projection work; future Capability additions must update both the Skill default scope set and production allowlist.

#### Handoff Note

- Skill `1.3.0` and production allowlist contain all 26 scopes required by the 51 published capabilities. The independent GitHub Skill release/tag remains a separate release action.

### TASK-043 - Deploy Semattice-owned Keycloak access context

- status: `done`
- priority: `critical`
- owner_role: `release-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-042-production-standalone-auth-rollout.md`
- depends_on: `TASK-042`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `production release binary, /etc/semattice/semattice.env, semattice-cli audience mapper, rollout evidence`
- branch: `main`
- pr_url: `n/a`

#### Done When

- A new immutable release is active and the previous release plus environment backup are retained.
- Semattice owns `/v1/auth/token`, old AgentCiCi provisioning environment keys are absent, and the public negative smoke is fail closed.
- The `semattice-cli` Keycloak token is configured for `semattice-api` audience without changing unrelated applications.
- Service health, logs, checksums and rollback evidence are recorded.

#### Next Action

- 发布和真实只读登录验收已完成。业务读写前单独设计并批准 Principal 到 OACT scope/RBAC 的授予边界。

#### Handoff Note

- TASK-043完成时 production allowlist仅含 `system.capability.read`；TASK-044随后按用户要求扩展为全部26个公开能力scope。scope仍不替代Principal、角色、Permission Set、RLS和审批控制。

### TASK-042 - Remove AgentCiCi from Semattice authentication and runtime

- status: `done`
- priority: `critical`
- owner_role: `integration-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-041-semattice-owned-keycloak-access-context.md`
- depends_on: `TASK-029, TASK-041`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `access-context endpoint, Keycloak verifier, OACT signer, tenant mapping, runtime configuration, cloudcc-semattice skill and tests`
- branch: `main`
- pr_url: `n/a`

#### Done When

- Keycloak Organization membership maps directly to an active Semattice tenant.
- Semattice signs its own short-lived OACT and the Skill never calls AgentCiCi.
- AgentCiCi provisioning configuration and route are absent from the active runtime.
- Go/Python tests, skill validation and local skill installation pass.

#### Next Action

- 本地实现和本机 Skill `1.2.2` 更新已完成；生产发布和真实 PKCE/OACT只读验收已由 TASK-043 完成。

#### Handoff Note

- 已部署的 Keycloak Realm/域名是基础设施兼容标识，本任务没有删除其他应用 client、用户、Organization或会话。旧 `1.2.1` 登录缓存必须重新登录。

### TASK-026 - Enforce AgentCiCi-controlled company provisioning

- status: `canceled`
- priority: `critical`
- owner_role: `integration-agent`
- claimed_by: `project-manager`
- spec_path: `docs/specs/FEAT-026-agentcici-controlled-company-provisioning.md`
- depends_on: `TASK-011, TASK-025`
- blocked_by: `superseded by TASK-042`
- related_issues: `none`
- scope_files: `removed AgentCiCi reservation/completion client, HMAC route and provisioning configuration`
- branch: `agent/go-capability-platform-baseline`
- pr_url: `n/a`

#### Done When

- Historical AgentCiCi-controlled provisioning is no longer an active Semattice runtime dependency.

#### Next Action

- 不恢复旧 `/internal/v1/company-provisionings`、reservation/complete或 HMAC配置。未来需要新建 tenant时，另开独立、产品中立的管理员控制面规格。

#### Handoff Note

- 2026-07-24 的联调和生产证据保留为历史记录；TASK-042按用户要求删除了对应活动代码和启动门禁。

### TASK-040 - 管理中心接入真实租户治理数据

- status: `done`
- priority: `critical`
- owner_role: `fullstack-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-034-live-tenant-governance-console.md`
- depends_on: `TASK-032, TASK-033`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `console reader, console API/static view, tests and production validation`
- branch: `main`
- pr_url: `n/a`

#### Done When

- 控制台由已验证会话的 tenant context 读取真实 published metadata、RBAC、组织和审计投影。
- `org5nszpgj99jaysxv6y` 显示 5 个研发交付对象与 37 个有效字段，未投影身份数据明确为空态。
- 演示 fixture 不再进入生产控制台响应，安全边界和线上 read-only 验证通过。

#### Next Action

- 已发布 `20260731T012059Z-console`；真实租户读取返回 5 对象、37 有效字段及 0 本地成员/角色/组织投影。

### TASK-021 - Establish the Phase 0 Loop Engineering controls

- status: `done`
- priority: `critical`
- owner_role: `project-manager`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `none`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `LOOP.md, STATE.md, loop-constraints.md, loop-budget.md, loop-run-log.md, .claw/`
- branch: `main`
- pr_url: `n/a`

#### Done When

- L1 loop state, budget, constraints and append-only run log are present and internally consistent
- Five-hour local automation ran with no remote-write or source-edit permission
- Initial readiness audit and project-state validation have real recorded results
- Five-hour handoff identifies the L2 promotion gate and next verified action

#### Next Action

- 已完成；不要恢复该 L1 bootstrap。后续 L2 授权和任务状态以 `current-status.md` 与活跃任务卡为准。

#### Handoff Note

- This was the L1 report-only bootstrap. Its original source-code gate has since been satisfied and expanded by the user; do not reuse this completed task as the current authorization record.
- Local L1 evidence is ahead of `origin/main` and deliberately unpushed; publishing remains a human-approved action.
- Five-hour handoff: state validation passed; 23 pre-handoff logs were monotonic, L1-only, with zero source actions. This line records the historical gate at handoff; subsequent approvals, Go evidence and expanded L2 scope are recorded in `current-status.md` and later task cards.

### TASK-020 - Implement the pure-Agent capability contract PoC

- status: `done`
- priority: `critical`
- owner_role: `shared`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-020-pure-agent-capability-contract.md`
- depends_on: `TASK-010` (user-authorized narrow L2 exception for the Go PoC only)
- blocked_by: `none`
- related_issues: `none`
- scope_files: `capability registry, API gateway, MCP server, non-interactive CLI, contract tests`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 每个已发布原子能力从统一 Capability Contract 派生 API、MCP Tool 与 CLI
- CLI 仅支持结构化输入和 JSON/JSON Lines 输出，无菜单、提示或终端状态依赖
- 三入口通过等价性、权限、幂等、审计和错误码契约测试

#### Next Action

- 已完成受限 L2 PoC；后续 PostgreSQL、租户控制面与身份集成已由用户另行扩大授权，必须转入 `TASK-010/011/012` 的独立规格与检查点，不在本任务继续追加源码。

#### Handoff Note

- `system.capability.list` 通过同一 Go Registry/Invoker 暴露 API、MCP 和无交互 CLI，独立 checker 已验证 test/race/vet/module verify、四目标纯 Go cross-build、无 TTY、MCP stdout 和 denylist。本任务完成时尚未授权数据库与身份集成；后续扩大授权以 `current-status.md` 为准。生产部署、CI、发布仍未授权，高风险异步 `operation_id`、持久审计与通用输出 Schema 校验仍未实现。

### TASK-010 - Define the technology stack and repository baseline

- status: `done`
- priority: `critical`
- owner_role: `backend-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-010-go-engineering-baseline.md`
- depends_on: `TASK-009`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `go.mod, go.sum, cmd/**, internal/config/**, internal/observability/**, internal/database/**, scripts/**, docs/**`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- Go 工程、配置、结构化日志、pgx 连接、显式 checksum migration runner 和测试目录基线可执行
- migrator/control/runtime 三种连接身份分离，26 个外部模块许可和 checksum 已审计

#### Next Action

- 已完成；后续任务复用同一配置、日志、连接池和迁移 runner，新增依赖继续执行许可门禁。

#### Handoff Note

- Go 1.26.5 单运行时、CGO-free 四目标构建和 PostgreSQL 16 基线已由 maker 与独立 checker 验证。生产 CI、发布、SBOM/provenance/signature 和部署仍不在本任务授权范围。

### TASK-011 - Integrate the unified tenant operations control plane

- status: `done`
- priority: `high`
- owner_role: `integration-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-011-unified-tenant-operations-control-plane.md`
- depends_on: `TASK-010`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `Native operations port, JWT identity, tenant registry, lifecycle, routing, operation, audit, API/MCP/CLI`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- Native 可独立开户；与 Agent CC 绑定时复用运营控制面提供的 UUIDv4 `tenant_id + 20 位 company_id`
- 生命周期、修订、幂等、失败恢复、审计与可信身份负向测试通过
- 六个租户能力由同一 Registry/Invoker 投影为 authenticated API、MCP 和无交互 CLI

#### Next Action

- 已完成当前仓库 Native 适配器与契约；未来跨仓库接入须由运营端实现版本化 `operations.Port`，不在本任务伪造完成。

#### Handoff Note

- `ai_native_control` 是非 owner、非 superuser、非 BYPASSRLS 的独立跨租户身份，仅在 tenant registry/operation/audit 三表获得精确权限；真实 main 双连接接线测试通过。

### TASK-009 - Review the greenfield architecture baseline

- status: `done`
- priority: `high`
- owner_role: `shared`
- claimed_by: `human + root`
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `none`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md, .claw/goals.md, .claw/decisions.md`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 绿地边界、目标架构、阶段路线和验收标准已完成架构评审
- 评审结论和架构修订均已写回 feature spec 与 ADR
- Phase 0 核心编码前置决策已明确；后置组件选型不阻塞开工

#### Next Action

- 已完成。后续从 `TASK-010` 开始执行 Phase 0，不重新打开本任务；架构变更通过对应 feature spec 和 ADR 管理。

#### Handoff Note

- 用户于 2026-07-18 正式批准 `FEAT-009`，规格状态已改为 `approved`，ADR-003 已接受。Event Bus、Search/OLAP、Wasm/流程、数据驻留和计费仍为独立后置 ADR，不阻塞 `TASK-010`。

### TASK-015 - Benchmark object records and typed indexes

- status: `done`
- priority: `critical`
- owner_role: `backend-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-015-record-runtime-and-typed-indexes.md`
- depends_on: `TASK-012, TASK-013`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `object_record, record_index_*, record_relation, internal/record/**, runtime.record.* API/MCP/CLI, benchmark harness`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 元数据驱动的 create/get/update/delete/query 通过 API、MCP 和无交互 CLI 等价暴露
- 乐观锁、软删除、类型校验、声明式 typed index、关系约束和受限查询 DSL 在真实 PostgreSQL 16 下通过
- 1,000,000 记录下的写放大、查询计划、延迟和存储成本有可重复证据；生产容量验收仍留给 TASK-019

#### Next Action

- 已完成；不要在本任务追加对象/字段/记录共享权限或 outbox。分别进入 TASK-016 和 TASK-017 的独立规格。

#### Handoff Note

- 五项 `runtime.record.*` 能力、migration 4、五类 640 typed partitions、durable write idempotency 和真实 bounded query planner 已通过本地 maker 验证。100 万记录为单机物理路径证据；50 并发与 8/16 GiB 容量仍由 TASK-019 验收。

### TASK-014 - Validate the Changeset publisher

- status: `done`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-014-changeset-and-dynamic-field-evolution.md`
- depends_on: `TASK-013, TASK-015`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `changeset state machine, dynamic field/index lifecycle, versioned quota gates, candidate projection, resumable backfill/coverage, unique/reference validation, purge/tombstone`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- Changeset 可预检、模拟、审批、发布、审计与受限回滚，并有版本一致性测试
- required/indexed/unique/reference、改名、改类型和删除均通过分阶段迁移，覆盖率未达 100% 时不能激活
- 每对象动态字段/索引和记录 JSONB 边界在 API、MCP、CLI 中一致执行，回填可恢复且不绕过 TenantContext/RLS

#### Next Action

- 已完成。下一阶段创建并批准 `TASK-016` 的授权与共享独立规格；`TASK-019` 继续负责 8/16 GiB 容量认证，不重新打开本任务。

#### Handoff Note

- migrations 5/6、十项 Changeset 能力、候选版本新写投影、按对象有界回填、revision 冲突恢复、required/index/unique/reference coverage、predecessor 转换、purge/tombstone 和版本化 service-tier 配额均已在真实 PostgreSQL 16 下通过。通用 worker/outbox 属于 `TASK-017`；最终容量数值属于 `TASK-019`。

### TASK-013 - Design the metadata core model

- status: `done`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-013-metadata-core-model.md`
- depends_on: `TASK-010`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `metadata version, object, field, relation, immutable publication, snapshot, API/MCP/CLI`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- UUIDv7 元数据版本、对象、字段和关系模型具有同租户/同版本复合约束及 FORCE RLS
- draft-only 变更、批准发布、发布后不可变、确定性 snapshot/digest 与六能力三入口 parity 测试通过

#### Next Action

- 已完成；`TASK-014` 已在此不可变元数据版本之上实现 Changeset 治理，后续任务继续保持本任务核心约束。

#### Handoff Note

- 元数据 CRUD/publish 始终使用严格 `ai_native_runtime` TenantContext；只有路由解析使用独立 control pool。跨租户、跨版本和已发布修改均 fail closed。

### TASK-016 - Implement role-centered authorization and record sharing

- status: `done`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-016-role-centered-rbac-organization-data-sharing.md`
- depends_on: `TASK-012, TASK-013, TASK-015`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `principal/organization/role/permission-set models, object/field/record PDP, organization data scopes, Owner, teams, share grants/rules, snapshots and audit`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 角色中心的对象、字段、Capability 和平台管理权限由 Permission Set 可复用组合，并强制最小权限与职责分离
- 记录权限由 Owner、数据归属组织范围、团队、显式共享和规则共享联合计算；组织层级不传播功能权限
- 对象、字段、记录、共享、调岗、组织合并和跨租户越权场景均有真实 PostgreSQL 与三入口验证

#### Next Action

- 已完成：角色中心 RBAC、组织数据范围、Owner、组/团队/直接共享、规则 `record × group` 投影及失败恢复、条件策略、职责分离、重组、解释审计、三入口、跨租户负向和本地百万记录验证均通过。通用 worker/outbox 与 200 活跃用户/热点公平性容量验收是 TASK-017/TASK-019 的独立工作。

#### Handoff Note

- 已完成并由本地 PostgreSQL 证据验证。Profile 不进入业务授权模型；组织树只计算数据范围与记录共享，禁止 `record × user` ACL 物化。规则投影只存 record-to-group 边且未 ready 时 fail closed；构建结束会检查缺边，access group disabled 会撤销其共享路径；组织合并不物理删除。通用自动运行/告警与完整容量验收已明确拆分至 TASK-017/TASK-019。

### TASK-024 - Add authenticated Streamable HTTP MCP transport

- status: `done`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-024-streamable-http-mcp.md`
- depends_on: `TASK-020`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `cmd/ai-native-platform/main.go, internal/identity/jwt.go, internal/mcp/**, README.md`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- `serve` exposes authenticated Streamable HTTP MCP at `/mcp` while stdio MCP and Capability API remain compatible
- every MCP HTTP request verifies bearer identity and sessions cannot cross tenant/principal boundaries
- SDK Streamable HTTP client tool invocation, authentication rejection, full test/race/vet/module/build checks pass

#### Next Action

- 2026-07-24 已在授权 ECS 配置并验证 Nginx Streamable HTTP `/mcp` 代理；公网未认证 POST 返回 401。后续只需由支持 Bearer header 的客户端携带有效短期 JWT 做真实工具发现；MCP OAuth resource metadata 仍需独立任务。

#### Handoff Note

- 该 endpoint 使用 MCP Streamable HTTP，不是旧 HTTP+SSE；默认 stateful session 空闲 5 分钟关闭，未配置 EventStore，因此不承诺断线重放。

## 维护规则

- 这里只归档 `done` 或 `canceled` 的任务。
- 当 `task-board.md` 中的 `Completed Tasks` 超过 20 条时，将最旧的任务卡移动到这里。
- 保留任务 ID、最终状态、相关 issue、主要变更范围和必要交接信息。
- 归档是移动，不是删除；不要让任务历史凭空消失。
