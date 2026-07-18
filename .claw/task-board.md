---
kind: task-board
version: 3
updated_at: 2026-07-18T15:44:25Z
updated_by: ai + human-expanded Phase 0 L2 source authorization
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

### TASK-010 - Define the technology stack and repository baseline

- status: `ready`
- priority: `critical`
- owner_role: `shared`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-009`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md, .claw/decisions.md, repository root`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 后端、MCP/CLI、部署、CI/CD 和安全门禁的关键选型形成 ADR
- 仓库结构、编码规范与本地开发基线可执行

#### Next Action

- 创建独立 feature spec 和新阶段 L2 检查点，固化仓库结构、PostgreSQL 驱动/迁移/连接池、配置、日志、测试、构建和供应链门禁，然后开始已授权的本地源码工作。

#### Handoff Note

- `FEAT-009`、ADR-003、ADR-007、ADR-008 与 ADR-009 已接受，Go 1.26.5 和首个 MCP 依赖许可门禁已有 PoC 证据。用户已授权本仓库 PostgreSQL、租户控制面和身份集成的本地 L2 工作；后续新增依赖仍须逐项通过许可与供应链门禁，不得把后置组件选型自动写成已接受。

### TASK-011 - Integrate the unified tenant operations control plane

- status: `todo`
- priority: `high`
- owner_role: `integration-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-011-unified-tenant-operations-control-plane.md`
- depends_on: `TASK-010`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `existing operations provisioning adapter, UUIDv4 tenant identity, tenant registry, shard registry, lifecycle, quotas, routing and audit contract`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- Agent CC-only、Native-only 和双产品三种组合均可开户；开通首个产品时只分配一组 UUIDv4 `tenant_id` 与一对一 20 位 `org_id`
- 后续绑定第二个产品只能复用已有全局租户标识；registry、独立产品生命周期、配额和路由契约已实现，任一产品失败不影响另一产品
- 任一产品开通失败恢复、乱序产品修订和重复请求测试通过
- RLS、连接池上下文和查找/主从关系的跨租户拒绝测试通过
- 已发布租户原子能力通过 API、MCP 和无交互 CLI 等价性测试

#### Next Action

- 在 `TASK-010` 完成后盘点既有运营开户接口、共享身份依赖和 Agent CC 存量 `org_id`，输出产品无关的全局租户目录、独立产品订阅、绑定契约与回填差异报告，再在当前仓库实施已授权的 Native 适配器。

#### Handoff Note

- 全局 `tenant_id + org_id` 和生命周期由既有运营端唯一写入；Agent CC 与 Native 是同一全局租户下两个对等的 `0..1` 产品订阅，均可单独或按任意顺序开通，绑定时必须共享同一 UUIDv4。Native 仅以 `tenant_id -> shard_id + tenant_bucket + route_revision` 管理数据落点；任一产品 `not_provisioned` 都是合法状态。当前 L2 允许本仓库的 Native 适配器、共享身份边界和本地 PostgreSQL PoC，不允许直接修改 Agent CC/运营端仓库或连接生产系统。

### TASK-012 - Validate the PostgreSQL shard baseline

- status: `todo`
- priority: `critical`
- owner_role: `backend-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-010`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `PostgreSQL 16.13 Docker schema, 128 LIST partitions, RLS, pooling context and 8 GiB/16 GiB capacity validation`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- UUID `tenant_id` 的分区裁剪、RLS、连接池上下文清理、跨租户关系拒绝和两个内存档位容量指标均有真实验证证据；生产 HA/备份/恢复另行验证

#### Next Action

- 在 `TASK-010` 完成后创建独立 feature spec 与可重复运行的隔离/性能测试计划；冻结 CPU、IOPS、数据库参数、记录形状、读写比例和热租户分布，并在本机专用 Docker PostgreSQL 16.13 PoC 执行已授权迁移和测试。

#### Handoff Note

- 当前 L2 允许创建和运行本地 PoC schema/migration、RLS、连接池及容量验证；不允许连接或迁移生产/共享远程数据库，也不允许租户请求直接指定 shard 或 bucket。

### TASK-013 - Design the metadata core model

- status: `todo`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-010`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `object, field, relation, package, version and compiled metadata model`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 元数据模型、版本快照和编译契约具有确定性测试

#### Next Action

- 创建独立 feature spec，明确数据模型与依赖图规则。

#### Handoff Note

- 元数据是平台协议，不是仅供页面使用的配置。

### TASK-014 - Validate the Changeset publisher

- status: `todo`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-013`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `changeset validation, simulation, approval, publish and rollback`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- Changeset 可预检、审批、发布、审计与回滚，并有版本一致性测试

#### Next Action

- 创建独立 feature spec，定义治理状态机和失败语义。

#### Handoff Note

- API、MCP Tool 与 CLI 必须生成等价的 Changeset。

### TASK-015 - Benchmark object records and typed indexes

- status: `todo`
- priority: `critical`
- owner_role: `backend-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-012, TASK-013`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `object_record, typed dynamic indexes, relations and query planning`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 目标负载下的写放大、查询延迟和存储成本有可重复的基准证据

#### Next Action

- 创建独立 feature spec，冻结 PoC 负载、指标与退出准则。

#### Handoff Note

- 仅为声明的可查询字段建立类型化索引。

### TASK-016 - Validate authorization and record sharing

- status: `todo`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-012, TASK-013, TASK-015`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `object, field and record authorization; sharing; permission snapshots`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 对象、字段、记录权限及跨租户越权场景均通过验证

#### Next Action

- 创建独立 feature spec，定义授权决策链和共享计算边界。

#### Handoff Note

- 应用权限、RLS 与审计测试必须共同生效。

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

## Completed Tasks

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

## 维护规则

- 这里只记录可执行任务，不记录完整 bug 细节或长篇设计。
- `owner_role` 是稳定责任角色，不依赖智能体自我身份。
- `claimed_by` 是可选运行时标签，环境知道就写，不知道可留空。
- 非平凡功能任务应填写 `spec_path` 并指向 `docs/specs/` 下真实文件。
- Brownfield 接入任务可先指向 `docs/specs/PROJECT-BASELINE.md`，后续再拆成具体 feature spec。
- `Completed Tasks` 最多保留最近 20 条任务卡，超过后将最旧的 `done` 或 `canceled` 任务移动到 `task-archive.md`。
- 任务状态、依赖和交接说明变化时立即更新。
