---
kind: task-board
version: 3
updated_at: 2026-07-16T15:34:20Z
updated_by: ai
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

### TASK-009 - Review the greenfield architecture baseline

- status: `review`
- priority: `high`
- owner_role: `shared`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `none`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md, .claw/goals.md, .claw/decisions.md`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 绿地边界、目标架构、阶段路线和验收标准已完成架构评审
- 评审结论和任何架构修订均写回 feature spec 与 ADR
- 后续 Phase 0 任务的前置决策已明确

#### Next Action

- 评审 `FEAT-009`；批准后将其状态改为 `approved` 并开始 `TASK-010`

#### Handoff Note

- 本项目已完成治理初始化。实现前先读 `FEAT-009` 第 21、22、24、27 节；不要跳过 Phase 0 直接建设完整 CRM。

### TASK-010 - Define the technology stack and repository baseline

- status: `todo`
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

- 后端、前端、部署、CI/CD 和安全门禁的关键选型形成 ADR
- 仓库结构、编码规范与本地开发基线可执行

#### Next Action

- 在 TASK-009 获批后创建该任务的独立 feature spec，并完成选型比较

#### Handoff Note

- 不把尚未验证的技术选型写成已接受决策。

### TASK-011 - Build the tenant control-plane contract

- status: `todo`
- priority: `high`
- owner_role: `backend-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-010`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `tenant registry, shard registry, provisioning, quotas, routing contract`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- registry、开户、配额和路由契约已实现并通过测试

#### Next Action

- 创建独立 feature spec，细化控制面数据和 API 合约。

#### Handoff Note

- `tenant_id -> shard_id + tenant_bucket` 是控制面唯一事实源。

### TASK-012 - Validate the PostgreSQL shard baseline

- status: `todo`
- priority: `critical`
- owner_role: `backend-agent`
- claimed_by: ``
- spec_path: `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`
- depends_on: `TASK-010`
- blocked_by: `none`
- related_issues: `none`
- scope_files: `PostgreSQL schema, 128 LIST partitions, RLS, pooling context, HA and backup validation`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 分区裁剪、RLS、连接池上下文清理和故障恢复均有真实验证证据

#### Next Action

- 创建独立 feature spec 与可重复运行的隔离/性能测试计划。

#### Handoff Note

- 不允许租户请求直接指定 shard 或 bucket。

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

- 人工 UI 与 Agent 工具必须生成等价的 Changeset。

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
- scope_files: `isolation, capacity, hot-tenant, failure and recovery test suites`
- branch: `n/a`
- pr_url: `n/a`

#### Done When

- 高风险架构假设的性能、隔离和故障测试均有真实、可复现的结果

#### Next Action

- 创建独立 feature spec，定义数据集、环境、阈值与证据保留方式。

#### Handoff Note

- 测试结果必须记录在 `test-report.md`，不能以设计结论替代实测。

## Completed Tasks

- 暂无已完成任务。

## 维护规则

- 这里只记录可执行任务，不记录完整 bug 细节或长篇设计。
- `owner_role` 是稳定责任角色，不依赖智能体自我身份。
- `claimed_by` 是可选运行时标签，环境知道就写，不知道可留空。
- 非平凡功能任务应填写 `spec_path` 并指向 `docs/specs/` 下真实文件。
- Brownfield 接入任务可先指向 `docs/specs/PROJECT-BASELINE.md`，后续再拆成具体 feature spec。
- `Completed Tasks` 最多保留最近 20 条任务卡，超过后将最旧的 `done` 或 `canceled` 任务移动到 `task-archive.md`。
- 任务状态、依赖和交接说明变化时立即更新。
