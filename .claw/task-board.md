---
kind: task-board
version: 3
updated_at: 2026-08-04T04:07:09Z
updated_by: root after publishing cloudcc-semattice v1.4.1
board_status: active
---

# 任务看板

`task-board.md` 是执行任务、责任角色、依赖关系和交接说明的唯一事实源。

推荐状态值：`todo` / `ready` / `in_progress` / `blocked` / `review` / `done` / `canceled`  
推荐优先级：`critical` / `high` / `medium` / `low`

推荐 `owner_role`：

- `backend-agent`
- `frontend-agent`
- `fullstack-agent`
- `qa-agent`
- `release-agent`
- `project-manager`
- `integration-agent`
- `human`
- `shared`
- `unassigned`

## Active Tasks

### TASK-050 - Render published objects with no fields in the administration console

- status: `review`
- priority: `critical`
- owner_role: `fullstack-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `TASK-040`
- blocked_by: `none`
- related_issues: `ISSUE-002`
- scope_files: `internal/console reader and tests, console JavaScript, production release and browser verification`
- branch: `main`
- pr_url: `n/a`

#### Done When

- Published objects with zero fields are returned with `fields: []`, never `null`.
- The object list and field matrix render safely even if an older or malformed response contains a non-array `fields` value.
- Focused and repository regression checks pass, and production Chrome renders both `contact` and `large_backpack` without console errors.

#### Next Action

- 统一 release `20260803T051441Z-web-oidc-2329787b57ff` 已上线，HTTPS API 确认 `contact.fields=[]`、`large_backpack` 为 2 字段；待用户登录后补真实浏览器零错误验收。

### TASK-033 - Semattice 统一 Principal 投影与官方机器主体认证

- status: `in_progress`
- priority: `critical`
- owner_role: `integration-agent`
- claimed_by: `project-manager`
- spec_path: `docs/specs/FEAT-033-unified-principal-projection.md`
- depends_on: `TASK-026, AgentCiCi FEAT-145`
- blocked_by: `Keycloak SMTP、OACT 签名配置和受权 service client 凭据后执行真实人类/机器端到端验收`
- related_issues: `none`
- scope_files: `identity verifier, capability principal, official OACT contract, tests, migrations and production validation`
- branch: `main`
- pr_url: `n/a`

#### Done When

- Semattice maps new official OACT `principal_id` / `principal_type` locally without accepting raw Keycloak service tokens.
- Service OACT requires AgentCiCi-issued owner evidence and all API/MCP/CLI entrypoints bind the same Principal.
- Human compatibility, negative validation, local JWKS caching and production smoke evidence are recorded.

#### Next Action

- 已完成 Principal claim、API/MCP/CLI 回归和生产兼容发布；等待 AgentCiCi 新版上线后，以真实受权机器账户执行 OACT exchange、owner 失效和撤销投影验收。

### TASK-017 - Validate transactional outbox and workers

- status: `todo`
- priority: `high`
- owner_role: `integration-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-012`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `outbox events, idempotency, retries, dead-letter handling and workers`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 记录事务与事件写入原子，且 worker 重试、幂等和死信有测试证据

#### Next Action

- 创建独立 feature spec，定义事件契约与投递状态机。

#### Handoff Note

- 异步消息必须显式携带可验证的租户上下文和版本。

### TASK-018 - Build the Agent Tool Gateway proof of concept

- status: `todo`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-014, TASK-016`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `tool schemas, agent identity, authorization, budget and audit`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- Agent 可通过受控工具读取元数据并完成低风险 Changeset 闭环，且所有操作受审计

#### Next Action

- 创建独立 feature spec，定义工具契约、风险等级和审批边界。

#### Handoff Note

- Agent 不能接触底层数据库或任意网络能力。

### TASK-019 - Execute Phase 0 isolation, capacity and recovery verification

- status: `todo`
- priority: `critical`
- owner_role: `qa-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-011, TASK-012, TASK-014, TASK-015, TASK-016, TASK-017, TASK-018`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `isolation, capacity and hot-tenant test suites; production HA/recovery is out of Phase 0 scope`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 高风险架构假设的性能、隔离和热租户公平性测试均有真实、可复现的结果；生产 HA/恢复测试留待后续阶段

#### Next Action

- 创建独立 feature spec，定义数据集、环境、阈值与证据保留方式。

#### Handoff Note

- 测试结果必须记录在 `test-report.md`，不能以设计结论替代实测。

### TASK-027 - Implement phased usage metering and cost observability

- status: `in_progress`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-027-phased-usage-metering-and-cost-observability.md`
- depends_on: `TASK-017, TASK-012, TASK-015, TASK-020`
- blocked_by: `TASK-017`
- related_issues: `none`
- scope_files: `capability invocation/transports, PostgreSQL migrations and RLS, transactional outbox consumer, record runtime deltas, usage query capabilities, tests and capacity evidence`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- Phase 1 exposes tenant-authorized API/MCP/CLI request counts, unique capability executions, versioned RU, logical business-data bytes and effective record counts without duplicate metering
- ledger/outbox/current counters/rollups/physical samples have RLS, retention, calibration and real performance evidence
- shared-partition physical storage is explicitly presented as a platform size or allocation estimate, never as a falsely exact tenant value

#### Next Action

- Add PostgreSQL end-to-end metering/RLS/calibration tests and a metering-on/off performance baseline. Internal package race/vet/module checks passed on 2026-07-23; full repository validation remains blocked by two existing `cmd` tests (stdio EOF and disabled public `tenant.provision`).

#### Handoff Note

- User authorized Phase 1 implementation on 2026-07-23. Ledger/current buckets/hourly rollups, API/MCP/CLI entrypoint meter, RU, CRUD logical-byte/record deltas, summary/timeseries and shared physical-storage sample Capability are implemented locally. It intentionally does not enable pricing, invoicing, automatic suspension, AI/connector meters, external TSDB or a Web UI. Read FEAT-027 before implementation; current `audit_event` is not a usage ledger.

## Completed Tasks

### TASK-056 - Rewrite and publish the cloudcc-semattice quick start

- status: `done`
- priority: `high`
- owner_role: `release-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `TASK-055`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `skill/cloudcc-semattice README/VERSION, independent Skill release repository, validation and GitHub publication evidence`
- branch: `main`
- pr_url: `n/a`

#### Done When

- README quick start contains only the user-facing `$cloudcc-semattice` login and current-object-list prompt flow, with concise login and read-only safety context.
- The old environment-variable, Token, Python API, capability-discovery and dry-run quick-start content is absent.
- Version `1.4.1` is validated, synchronized to the independent release repository, tagged and pushed without rewriting history, and GitHub raw README is verified.

#### Next Action

- Completed. Future changes require a new SemVer release; do not move or reuse tag `v1.4.1`.

#### Handoff Note

- GitHub `main` and annotated tag `v1.4.1` both point to `69fdb8ddd872e565df11ddd4f441f464fc183a89`; raw `VERSION` and README were verified after publication.

### TASK-055 - Add cloudcc-semattice Skill installation and usage guidance to the website

- status: `done`
- priority: `high`
- owner_role: `frontend-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `none`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `deploy/semattice/www/index.html, deploy/semattice/www/styles.css, static page verification and website rollout`
- branch: `main`
- pr_url: `n/a`

#### Done When

- The `#cli` section links to the official `CloudCCAI/cloudcc-semattice` repository and shows the current published fixed-version installation command.
- Codex users can see how to invoke `$cloudcc-semattice` without exposing tokens or reusable secrets.
- Desktop and mobile layouts render without overflow, existing copy controls work, and the production page serves the new guidance.

#### Next Action

- Completed. Monitor the public GitHub link and update the fixed version only when a new Skill release is formally published.

#### Handoff Note

- Source commit `c74267e` is live under static rollout `20260804T035355Z-skill-c74267e`; rollback backup is `/var/www/semattice-backups/20260804T035355Z-skill-c74267e`.

### TASK-054 - Add governed Principal organization membership capability

- status: `done`
- priority: `high`
- owner_role: `integration-agent`
- claimed_by: `ai`
- spec_path: `docs/specs/FEAT-048-governed-principal-organization-membership.md`
- depends_on: `TASK-051, TASK-053`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `internal/principal, main wiring, capability catalog, production release and DEV Autopilot identity verification`
- branch: `main`
- pr_url: `n/a`

#### Done When

- `identity.principal.set-organization-membership` 作为正式高风险 Capability 发布，要求 HUMAN 管理主体、`authorization.manage` 和已验签独立审批。
- 能力可幂等建立或结束指定 Principal 的组织成员关系，并维持每个主体至多一个 active primary membership。
- 数据库集成测试、全量回归和生产能力发现通过；新增开发者哪吒通过该能力归属研发交付部。

#### Next Action

- 已完成；进入生产监控。

#### Handoff Note

- 提交 `3a0d9daa281a` 已发布为 `/opt/semattice/releases/20260804T030035Z-identity-membership-3a0d9daa281a`，线上能力发现为 55 项。
- 哪吒通过独立审批 `33f86ce8-6161-4ce0-a35b-60f527940556` 获得开发者角色和研发交付部 primary membership，随后被治理为 `suspended` 休息状态。

### TASK-053 - 调整 DEV Autopilot 研发身份花名册

- status: `done`
- priority: `high`
- owner_role: `integration-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-047-dev-autopilot-identity-roster.md`
- depends_on: `TASK-051, TASK-052`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `AgentCiCi Principal authority, Semattice identity/role/organization projections, console API and DEV Autopilot CLI verification`
- branch: `main`
- pr_url: `n/a`

#### Done When

- 管理中心显示 Oliver、大乔、悟空、后羿四名成员，机器主体 owner 均为 Oliver。
- 后羿拥有开发者角色和研发交付部主组织关系，并可使用 DEV Autopilot CLI 获取任务。
- 产品经理凭据不能冒充开发者调用 CLI。

#### Next Action

- 已完成；常规监控。

### TASK-052 - 修复管理中心成员与角色查询

- status: `done`
- priority: `high`
- owner_role: `fullstack-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `TASK-051`
- blocked_by: `none`
- related_issues: `ISSUE-003`
- scope_files: `internal/console/reader.go, internal/console/reader_test.go, project state and production release evidence`
- branch: `main`
- pr_url: `n/a`

#### Done When

- PostgreSQL 成员聚合查询不再违反 GROUP BY 规则，真实 reader 回归测试可捕获该错误。
- 最终统一 release 同时保留 CodeUp 主线能力，生产 `/console/api/members` 返回三类研发身份和三个角色。

#### Next Action

- 已完成并发布；常规监控。

### TASK-049 - Support manual confirmation for direct metadata publication

- status: `done`
- priority: `critical`
- owner_role: `backend-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-046-manual-direct-metadata-publish-confirmation.md`
- depends_on: `TASK-013, TASK-043`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `metadata publish service and audit, Capability contract, tests, cloudcc-semattice skill, production release and verification`
- branch: `main`
- pr_url: `n/a`

#### Done When

- `metadata.version.publish` accepts an explicitly supplied non-empty manual `approval_id` and audits it transactionally.
- All other approval-gated capabilities retain trusted OACT approval verification.
- Local regression, Skill validation, production deployment and health checks pass.

#### Next Action

- Completed. The user explicitly confirmed and published the complete two-object production version.

#### Handoff Note

- Commit `34023d0a55981761ed0642809b82a5f5b2f7db9f` is deployed as `/opt/semattice/releases/20260731T092534Z-web-oidc-34023d0a5598`. Version `019fb736-8c34-7f0c-a0e8-82f385ffd9b0` was published with both `contact` and `large_backpack`; publish audit ID is `audit:req-f7b19407-5c65-48b3-8eaa-fdd6369c063b`.

### TASK-051 - 建立 DEV Autopilot 研发身份投影与角色治理

- status: `done`
- priority: `critical`
- owner_role: `integration-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-035-governed-development-principal-projection.md`
- depends_on: `TASK-033, AgentCiCi TASK-262`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `identity projection capabilities, migration 0017, console reader, tests and production role bootstrap`
- branch: `main`
- pr_url: `n/a`

#### Done When

- HUMAN/SERVICE OACT can safely self-project without caller-supplied identity claims.
- Product director, product manager and developer projections, owner relation and least-privilege roles exist in the target tenant.
- Console shows the three real identities and no demo governance identities.

#### Next Action

- 已发布 migration 17、Principal 能力与控制台投影；目标租户三类身份、角色、组织、字段权限、5 个 enforced 策略和生命周期负例均完成生产回读。

### TASK-041 - Add Keycloak PKCE login to cloudcc-semattice skill

- status: `done`
- priority: `high`
- owner_role: `integration-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-040-skill-keycloak-pkce-login.md`
- depends_on: `TASK-029, FEAT-028`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `skill/cloudcc-semattice authentication helper and guidance, Keycloak semattice-cli registration, Python tests and project state`
- branch: `main`
- pr_url: `n/a`

#### Done When

- Human CLI login uses Keycloak Authorization Code + S256 PKCE and a loopback callback without collecting passwords.
- Refresh tokens use the operating-system credential store; only a short OACT and non-sensitive metadata use a user-only local cache.
- Semattice `/v1/auth/token` mints the current-organization OACT; API calls refresh automatically and never send raw Keycloak tokens to the Capability API.
- Legacy explicit-token invocation remains compatible, and automated security/refresh/contract tests plus skill validation pass.

#### Next Action

- Completed by TASK-042 and TASK-043: Semattice-owned token exchange, production `semattice-cli` registration and real PKCE/OACT read-only acceptance all passed.

#### Handoff Note

- Version `1.2.1` was the local PKCE implementation milestone. TASK-042 upgraded the final Skill to `1.2.2`, synchronized the local installed copy and removed the AgentCiCi exchange dependency; TASK-043 completed live login. The independent GitHub Skill release/tag remains a separate release action.

### TASK-039 - Publish Semattice guidance update to CodeUp and GitHub

- status: `done`
- priority: `high`
- owner_role: `release-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `TASK-038`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `project repository, skill/cloudcc-semattice, /Users/xuhm/Documents/cloudcc-semattice-skill, CodeUp origin, GitHub CloudCCAI/cloudcc-semattice, .claw/`
- branch: `main`
- pr_url: `n/a`

#### Done When

- The project guidance changes and release evidence are committed and pushed as a fast-forward to the Alibaba CodeUp `main` branch.
- The development skill is synchronized to the clean independent release repository after an inspected rsync dry-run.
- Skill validation, version consistency, secret/cache checks and release-repository diff checks pass.
- GitHub `main` and the new immutable annotated tag `v1.1.0` point to the same release commit and remote README/VERSION are verified.

#### Next Action

- Completed. Future skill changes start from the project development copy and require a new immutable SemVer tag.

#### Handoff Note

- CodeUp content commit is `a55d71d773446902598b28fb525c7562003f351b`. GitHub release commit is `3ac29afc34366d66a2e9320975dc3be498d55181`; GitHub main and `v1.1.0^{}` match. No force push or historical tag movement was used.

### TASK-038 - Separate Semattice product guidance from execution guidance

- status: `done`
- priority: `high`
- owner_role: `fullstack-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `TASK-037`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `skill/cloudcc-semattice product positioning, business module scenarios, metadata operation workflows, local skill installation and project state`
- branch: `main`
- pr_url: `n/a`

#### Done When

- AI can distinguish Semattice product/design questions from authorized API execution requests.
- Every current public capability domain is connected to its business problem, representative scenario and implementation entrypoint.
- Objects, fields and relations have explicit create/read/update/delete or evolution boundaries without inventing unsupported APIs.
- The skill validates successfully and the local installed copy matches the project development copy.

#### Next Action

- Completed locally as development version `1.1.0`; subsequent GitHub release is tracked by `TASK-039`.

#### Handoff Note

- Product positioning is grounded in goals, ADR-012 and FEAT-009. API availability remains grounded in the runtime registry and API catalog. The installed copy is `/Users/xuhm/.codex/skills/cloudcc-semattice`; `TASK-039` subsequently published immutable `v1.1.0`.

### TASK-037 - Rename the skill to cloudcc-semattice

- status: `done`
- priority: `high`
- owner_role: `release-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `TASK-036`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `AGENTS.md, skill/cloudcc-semattice, /Users/xuhm/Documents/cloudcc-semattice-skill, CloudCCAI GitHub repository, .claw/`
- branch: `main`
- pr_url: `n/a`

#### Done When

- 技能目录、SKILL frontmatter、README、UI 显示名和默认调用名统一为 `cloudcc-semattice`
- 不兼容的调用名变更以 `1.0.0` 管理，旧调用名不保留兼容别名
- 项目内开发副本、独立发布仓库、GitHub 仓库名称与 `v1.0.0` 发布结果均已验证

#### Next Action

- 已完成；后续版本从项目内 `skill/cloudcc-semattice` 开发副本按根 `AGENTS.md` 流程同步、校验并发布。

#### Handoff Note

- GitHub 仓库为 `https://github.com/CloudCCAI/cloudcc-semattice`；`main` 和 annotated tag `v1.0.0` 均指向提交 `5b156c057af7517c81f5892d1f8123ec74f00ea6`。本地独立发布仓库位于 `/Users/xuhm/Documents/cloudcc-semattice-skill`，项目内开发副本不含 `.git`。禁止 force push 或移动已发布标签。

### TASK-036 - Document and exercise the Semattice skill release workflow

- status: `done`
- priority: `high`
- owner_role: `release-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `TASK-035`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `AGENTS.md, skill/semattice-customization-expert-universal/README.md, VERSION, /Users/xuhm/Documents/semattice-customization-expert-universal, CloudCCAI GitHub repository, .claw/`
- branch: `main`
- pr_url: `n/a`

#### Done When

- 项目根 `AGENTS.md` 固化开发副本与发布仓库边界、SemVer、同步、校验、提交、标签、推送和远程验证流程；技能 README 不包含这些内部维护步骤
- 开发副本与独立仓库内容一致，且项目内技能目录不包含 `.git`
- `v0.1.1` 通过发布前校验并以原子 push 发布，远程分支、标签、VERSION 和 README 均经验证

#### Next Action

- 已完成文档边界纠正；后续修改技能时，只按项目根 `AGENTS.md` 的发布流程执行，每次创建新的不可变 `v<version>` 标签。当前本地 README 纠正未提交或推送。

#### Handoff Note

- 内部发布流程的唯一来源是项目根 `AGENTS.md`。项目内开发源为 `skill/semattice-customization-expert-universal`，不含 `.git`；独立发布仓库为 `/Users/xuhm/Documents/semattice-customization-expert-universal`。`v0.1.1` 对应提交 `228f6f737b53ce41cc3f51126ca58498d33a3f47`，禁止 force push 或移动已发布标签。

### TASK-035 - Version and publish the Semattice customization skill

- status: `done`
- priority: `high`
- owner_role: `release-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `none`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `skill/semattice-customization-expert-universal, /Users/xuhm/Documents/semattice-customization-expert-universal, CloudCCAI GitHub repository, .claw/`
- branch: `main`
- pr_url: `n/a`

#### Done When

- 根目录 `VERSION` 是唯一版本源，README 说明安装、使用、SemVer 和升级流程
- 独立仓库首次提交和 `v0.1.0` 标签通过技能与 CLI 校验
- `CloudCCAI/semattice-customization-expert-universal` 公开仓库存在，远程 `main`、标签和 README 均已验证

#### Next Action

- 已完成；后续升级先更新 `VERSION` 和内容、通过技能校验，再提交并创建新的不可变 `v<version>` 标签。

#### Handoff Note

- 远程仓库为 `https://github.com/CloudCCAI/semattice-customization-expert-universal`。本地独立仓库位于 `/Users/xuhm/Documents/semattice-customization-expert-universal`；main 与 `v0.1.0` 均发布提交 `93c2701`。禁止 force push 或移动已发布标签。

### TASK-034 - Rename the Semattice customization skill

- status: `done`
- priority: `medium`
- owner_role: `fullstack-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `none`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `skill/semattice-customization-expert-universal, .claw/`
- branch: `main`
- pr_url: `n/a`

#### Done When

- 技能目录、SKILL frontmatter 和默认 `$skill` 调用名统一为 `semattice-customization-expert-universal`
- UI 显示名与新技能名称一致，功能说明、参考资料和脚本行为不变
- 官方技能校验、旧名称残留和脚本语法检查通过

#### Next Action

- 已完成；后续安装或调用必须使用 `$semattice-customization-expert-universal`。

#### Handoff Note

- 本次仅重命名技能身份和显示标题，不改变 Capability API 操作边界。

### TASK-032 - Semattice 企业管理中心第一版

- status: `done`
- priority: `critical`
- owner_role: `fullstack-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-030-semattice-administration-console.md`
- depends_on: `TASK-029`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `console browser session, console read APIs, static administration UI, demo fixtures, tests, deployment assets and verification evidence`
- branch: `main`
- pr_url: `n/a`

#### Done When

- 企业系统管理员能以已验证的 OACT 安全跳转至 `/console/`，并只在拥有管理 scope 时查看演示治理数据。
- 六个只读页面、对象字段三栏检查器、模拟数据、无权/加载/空/错误状态和桌面浏览器验收完成。
- 应用与静态页面通过质量门并发布到授权线上环境，保留可恢复的上一 release 与真实 smoke 证据。

#### Next Action

- 已完成：使用用户提供的授权 ECS 身份发布 `/opt/semattice/releases/20260725T025439Z-console`。顶栏产品菜单可返回 AgentCiCi 管理端且不传递 OACT；控制台静态资源为 no-store，HTML 同时引用带版本的 CSS/JS，避免浏览器样式陈旧。Nginx 与服务均 active，匿名会话状态为 200、治理 API 无 Cookie 为 401，桌面浏览器菜单尺寸及回跳地址均已 smoke；旧应用 release、静态站和 Nginx 配置备份均已保留。

#### Handoff Note

- 不能使用 query 参数、localStorage、sessionStorage 或日志保存 OACT；不得改变 AgentCiCi 租户运营职责或创建任何业务记录。

### TASK-031 - Permit public MCP discovery while authenticating tool invocation

- status: `done`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-024-streamable-http-mcp.md`
- depends_on: `TASK-024, TASK-029`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `internal/mcp Streamable HTTP handler, MCP tests, endpoint contract documentation, production release evidence`
- branch: `main`
- pr_url: `n/a`

#### Done When

- Anonymous MCP clients can initialize and list the published tool descriptors without a token.
- Every tool invocation is authenticated and derives tenant, subject and scopes only from the JWT verified for that request.
- Unit/full regression, main-branch merge and production smoke evidence are recorded.

#### Next Action

- 已完成：提交 `9cbbd28` 已快进到并推送 CodeUp `main`；release `20260724T155000Z-mcp-public-discovery` 已在授权 ECS 验证匿名 discovery、拒绝匿名 tool call 与 edge health。

#### Handoff Note

- A discovery session ID is never an authorization grant. Anonymous allowlist is only `initialize`, `notifications/initialized` and `tools/list`; all other methods fail closed without Bearer authentication. Nginx must set upstream `Host` to `127.0.0.1` to preserve the SDK's loopback DNS-rebinding protection.

### TASK-030 - Publish the current project to Alibaba Cloud CodeUp

- status: `done`
- priority: `high`
- owner_role: `release-agent`
- claimed_by: `root`
- spec_path: `none`
- depends_on: `none`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `Git refs/remotes plus .claw/devops.md, .claw/current-status.md, .claw/task-board.md and .claw/test-report.md`
- branch: `agent/go-capability-platform-baseline`
- pr_url: `n/a`

#### Done When

- 当前工作树与提交历史通过发布前测试、状态、敏感信息和大文件门禁
- CodeUp 独立 remote 保留 GitHub origin，并将当前 HEAD 发布为远程 main
- 远程 main 与最终本地 HEAD 一致且工作树保持干净

#### Next Action

- 已完成；后续发布继续向 CodeUp `main` 推送普通快进提交，禁止使用 force/mirror 覆盖远端历史。

#### Handoff Note

- CodeUp 首次 `HEAD -> main` 已成功；GitHub origin、本地工作分支名和 upstream 未修改。不要用 `--mirror`、force push 或把 CodeUp 凭据写入仓库。

### TASK-029 - Deploy Keycloak production IdP and integrate Semattice resource-server authentication

- status: `done`
- priority: `critical`
- owner_role: `integration-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-029-keycloak-resource-server-and-production-idp.md`
- depends_on: `TASK-028`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `Keycloak production deployment, Nginx TLS vhost, dedicated PostgreSQL database/role, Semattice token verifier, principal projection, API/MCP/CLI authentication tests and release evidence`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- Keycloak is healthy at `https://sso.agentcici.com`, with fixed hostname, dedicated database/user, protected management secrets and production reverse-proxy configuration.
- Semattice validates official OACT and third-party Keycloak service tokens locally through JWKS, rejects missing/invalid/unbound tokens, and never calls Keycloak per request.
- AgentCiCi mapping/projection integration, production rollout and positive/negative smoke evidence are recorded without storing secrets in the repository.

#### Next Action

- 已完成 Keycloak 基础设施、AgentCiCi OIDC/OACT、Semattice 本地 JWKS 验签、真实公司绑定和 AgentCiCi 登录主题上线；主题直接复用 AgentCiCi `login_mode2` 的真实图片与立方体结构。后续官方应用按现有 OACT/JWKS 契约接入，不再改变 IdP 的认证边界。

#### Handoff Note

- This task operationalizes FEAT-028. It supersedes the previous design-only remote-change restriction for this narrowly authorized identity scope.

### TASK-028 - Specify Keycloak and official-application access architecture

- status: `done`
- priority: `high`
- owner_role: `integration-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-028-keycloak-first-party-application-access.md`
- depends_on: `ADR-006, ADR-009, ADR-013`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `identity and token architecture specification only; no runtime or remote changes`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- Keycloak IdP, official OACT, third-party service account, API/MCP/CLI, tenant mapping, expiry, revocation and future-app registration boundaries are explicit.

#### Next Action

- 设计文档已完成；任何 ACS、Keycloak、AgentCiCi、FollowUp 或数据平台实现必须另建任务并获得跨仓库/生产授权。

#### Handoff Note

- 以 FEAT-028 为唯一详细设计；Keycloak 登录 Token 不在官方服务间泛化转发，官方 OACT 和第三方 Token 是两条独立信任路径。

### TASK-025 - Rename provisioning global identity to company_id

- status: `done`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: `root`
- spec_path: `docs/specs/FEAT-025-company-identity-rename.md`
- depends_on: `TASK-011`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `tenant control models/capabilities, JWT principal, tenant_registry migration, current architecture specs and tests`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 开户、状态、JWT、operations port 与数据库全都只使用 `company_id`
- `organization_id` 授权组织树保持不变
- 新旧数据库路径、API/MCP/CLI 和回归门禁有真实验证

#### Next Action

- 已完成本地 v2 改名与 migration 13 验证；既有 20 位公司编号值未重键。远程 ECS 制品、外部运营端和 JWT 发行方的协议切换需单独授权和发布任务。

#### Handoff Note

- `company_id` 是统一运营控制面的全局企业标识；`organization_id` 只属于租户内 RBAC/共享组织树。旧 `org_id` 输入与 JWT claim 已 fail closed。

## 维护规则

- 这里只记录可执行任务，不记录完整 bug 细节或长篇设计。
- `owner_role` 是稳定责任角色，不依赖智能体自我身份。
- `claimed_by` 是可选运行时标签，环境知道就写，不知道可留空。
- 非平凡功能任务应填写 `spec_path` 并指向 `docs/specs/` 下真实文件。
- Brownfield 接入任务可先指向 `docs/specs/PROJECT-BASELINE.md`，后续再拆成具体 feature spec。
- `Completed Tasks` 最多保留最近 20 条任务卡，超过后将最旧的 `done` 或 `canceled` 任务移动到 `task-archive.md`。
- 任务状态、依赖和交接说明变化时立即更新。
