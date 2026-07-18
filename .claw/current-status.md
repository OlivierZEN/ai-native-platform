---
kind: current-status
version: 3
updated_at: 2026-07-18T15:51:53Z
updated_by: ai + human-expanded Phase 0 L2 source authorization
phase: phase0_l2_scope_authorized
active_task: "TASK-010 ready; not claimed"
next_action: "Create the TASK-010 feature spec and a fresh L2 implementation checkpoint, then begin the authorized local Phase 0 source work."
read_next:
  goals: true
  decisions: true
  issue_list: false
  task_board: true
  test_report: true
  devops: false
---

# 项目当前状态

## 快照

- 当前技术栈：Go 1.26.5 单运行时与签名二进制交付（ADR-007）。API、MCP 与无交互 CLI 从同一 Capability Contract 投影。
- 当前数据 PoC：Docker PostgreSQL 16.13，单可用区、单 writer，8 GiB/16 GiB 内存档位；50 并发、200 活跃用户、100 万记录；不做 HA、备份或恢复演练（ADR-008）。
- 当前租户设计：既有 Agent CC 运营管理端扩展为产品无关的统一租户控制面；一个全局租户分别拥有 Agent CC `0..1` 与 Native `0..1` 产品订阅，任一产品都可单独或按任意顺序开通。只有需要绑定时，两者才必须归属同一全局租户并共享 UUIDv4 `tenant_id + 20 位 org_id`（ADR-009、FEAT-011）。
- 当前隔离原则：UUID 提供不可枚举和误命中缓冲，但不是授权边界；受信 TenantContext、强制 RLS、同租户关系约束、事务级数据库上下文和连接池残留测试必须共同生效。
- 架构评审状态：`FEAT-009` 已于 2026-07-18 由用户正式批准，`TASK-009` 已关闭，ADR-003 已接受；`TASK-010` 已解除依赖并进入 `ready`。
- 当前任务：`TASK-020` 的五小时 L2 Go Capability Contract PoC 已独立验证完成：`system.capability.list` 从同一 Go Registry/Invoker 投影为 API、MCP 与无交互 CLI。
- 实现状态：用户于 2026-07-18 将 L2 源码授权扩大到本仓库的 PostgreSQL、本地租户控制面和共享身份集成；`TASK-010` 已可建立新阶段检查点并开始实施。旧五小时 Capability PoC 的文件预算和 denylist 仍作为历史证据保留，不再代表新授权范围。

## 当前 L2 授权边界

- 允许：本仓库 Go 源码、规格、迁移、测试和本地工具；本机专用 Docker PostgreSQL 16.13 PoC 的 schema/migration、128 bucket、RLS、TenantContext、连接池、路由和容量验证。
- 允许：Native 租户控制面、产品生命周期、配额/路由投影、Capability API/MCP/CLI、幂等与审计；统一运营控制面使用版本化 adapter/port 接入。
- 允许：共享身份或企业 IdP 的当前仓库集成边界，包括令牌验证、audience/issuer/scope 校验、全局主体与最小成员投影、TenantContext 解析及负向测试；不复制密码或长期凭据。
- 仍禁止：访问或修改生产/共享远程数据库、生产身份系统或真实租户数据；生产/预生产部署、HA/备份演练、CI/镜像/制品发布、远端 Git push/PR/merge，以及向 Agent CC 或运营端其他仓库写入代码。扩大这些边界需要再次明确授权。

## 下一步

1. 为 `TASK-010` 创建独立 feature spec 和新阶段 L2 计划/预算/检查点，固化仓库与工程基线后开始已授权源码工作。
2. Event Bus、Search/OLAP、Wasm/流程、数据驻留和计费作为后置 ADR，在对应功能任务启动前决策，不阻塞 Phase 0 核心编码。
3. `TASK-011` 实施前先盘点既有运营开户接口、共享身份依赖和 Agent CC 存量 `org_id`，形成产品无关全局租户目录、独立产品订阅、绑定契约和 UUIDv4 回填差异报告。
4. PostgreSQL、租户控制面和身份集成已获本地 L2 授权；高风险/异步 operation、持久审计和通用输出 Schema 校验必须在对应 feature spec 中明确状态机、失败语义和独立验证，不得扩展到生产或远端系统。

## 证据与归档

- 已完成 PoC 计划：`docs/superpowers/plans/2026-07-18-five-hour-go-l2-loop.md`；它不覆盖本次扩大授权，新阶段计划将在 `TASK-010` 启动时创建。
- 统一租户设计：`docs/specs/FEAT-011-unified-tenant-operations-control-plane.md`。
- 已完成 L1 的状态、日志、预算、约束、计划和测试证据：`docs/archive/loop-engineering/` 与 `docs/archive/plans/`。
- 已替代的运行时 ADR：`docs/archive/decisions/ADR-005-superseded-runtime-proposal.md`。

## 已验证事实

- 本次统一租户及独立产品订阅文档治理校验：`passed`（2026-07-18T15:29:56Z）；状态 validator、已跟踪文件 `git diff --check` 与现行文档矛盾措辞检索均通过。本次只修改文档和项目状态，不修改或重新验证应用源码。此前独立验证仍已通过 Go 全量测试、race、vet、模块校验、四目标 `CGO_ENABLED=0` 构建和 denylist 审查。
- 本次 `FEAT-009` 批准与 `TASK-009` 关闭治理校验：`passed`（2026-07-18T15:43:29Z）；状态 validator、`git diff --check` 与状态冲突检索均通过。本次不修改或重新验证应用源码。
- 本次 L2 授权扩展治理校验：`passed`（2026-07-18T15:46:48Z）；状态 validator、`git diff --check` 与现行授权冲突检索均通过。此次只记录授权边界，尚未开始新阶段源码或数据库操作。
- 本次远端发布前校验：`passed`（2026-07-18T15:51:53Z）；Go 1.26.5 全量测试、race、vet、模块校验、四目标纯 Go 构建、状态 validator、差异检查、敏感凭据模式和大文件检查均通过。
- 历史验证的完整输出与运行记录在归档中；它们不是当前 Go 工具链的实现证据。
