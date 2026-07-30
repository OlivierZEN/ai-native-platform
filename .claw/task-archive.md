---
kind: task-archive
version: 3
updated_at: 2026-07-30T06:31:23Z
updated_by: root after TASK-038 completion exceeded active-board retention
archive_status: active
---

# 任务归档

`task-archive.md` 保存从 `task-board.md` 中移出的已完成或已取消任务卡。

## Archived Tasks

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

## 维护规则

- 这里只归档 `done` 或 `canceled` 的任务。
- 当 `task-board.md` 中的 `Completed Tasks` 超过 20 条时，将最旧的任务卡移动到这里。
- 保留任务 ID、最终状态、相关 issue、主要变更范围和必要交接信息。
- 归档是移动，不是删除；不要让任务历史凭空消失。
