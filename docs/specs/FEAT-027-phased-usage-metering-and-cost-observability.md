---
kind: feature-spec
feature_id: FEAT-027
title: Phased usage metering and cost observability
status: approved
owner_role: backend-agent
task_ids: TASK-027
related_decisions: ADR-007, ADR-008, ADR-009, ADR-012
related_issues: none
updated_at: 2026-07-23T13:52:26Z
updated_by: ai after user-approved phased metering scope
---

# FEAT-027 - 分阶段用量计量与成本可观测性

## 背景与目标

CloudCC Semattice 已具备统一 Capability Contract、可信 TenantContext、记录运行时与持久审计基础，但尚未具备可作为配额、运营看板或账单证据的统一 usage ledger。已有 `audit_event` 是安全与操作审计，不等同于计费计量：它不保存入口类型、RU、数据大小增量、计量规则版本或账务去重语义。

本特性建立低开销、可审计且按租户隔离的计量层。用户已明确要求优先看到以下四项：

1. API、MCP、CLI 的请求次数；
2. Runtime Unit（RU）；
3. 数据占用存储；
4. 实际业务数据的记录数量。

第一阶段只实现这四项及其查询能力；性能、可靠性、安全、自动化、AI、连接器、物理成本归因和商业定价在后续阶段逐步加入。所有计量都必须保持 API、MCP 与 CLI 对同一次 Capability 执行只计一次的契约。

## 范围

### Phase 1 In Scope

- 按 `entrypoint=api|mcp|cli` 统计入口请求；MCP 协议握手/工具发现与实际 Tool/Capability 执行分别统计，禁止混合。
- 按租户、Capability、状态和入口统计首次物理执行次数；幂等重放只增加入口请求数，不增加执行数或 RU。
- 用版本化、可解释的规则计算每次首次执行的 RU，并记录规则版本与实际耗时、输入/输出字节等校准证据。
- 对有效（未软删除）业务记录维护按租户、对象的实时记录数和逻辑数据字节数；关系、元数据、审计与索引分别标识，不把它们伪装成业务 JSON 数据。
- 采样分片/分区的物理表、索引、TOAST、WAL 等大小，并以“平台物理成本估算”展示；共享 bucket 中不得声称能精确归属单一租户的物理页。
- 提供受授权的 `usage.summary.get` / `usage.timeseries.get` Capability，并从同一 Contract 投影 API、MCP、CLI；本项目不新增 Web 控制台。
- 建立不可变 usage ledger、准实时 current aggregate、小时/日汇总、保留与校准策略，以及 usage 事件的幂等、RLS 和对账测试。

### Phase 1 Out Of Scope

- 依据 RU 或存储直接开票、定价、税务、优惠、合同结算、欠费处理或自动停服。
- LLM token、模型供应商费用、外部连接器、出网、搜索/OLAP、自动化 worker 或导出用量的计费。
- 高基数 Prometheus 标签、全量 request payload/response、access token、敏感字段值或记录 JSON 的采集。
- 每次请求实时计算 PostgreSQL 物理占用，或在共享分区中把 `pg_total_relation_size` 错误地当作租户精确大小。
- 新增外部 TSDB、数据仓库、对象存储或生产部署；Phase 1 先复用当前 PostgreSQL 16.13 边界。

## 术语与计量口径

| 名称 | 口径 | 适用场景 |
|---|---|---|
| 入口请求 | 到达并完成协议处理的一次 API/MCP/CLI 请求，含拒绝和幂等重放 | 流量、错误、入口体验 |
| Capability 执行 | 一次首次进入领域 handler 的物理执行；幂等重放为 0 | 使用量、RU、成本归因 |
| RU | Capability 基础单位加上版本化的输入规模、查询扫描档位、输出规模、写入/批量规模等 | 运行额度与后续计费 |
| 业务记录数 | 有效业务记录数；软删除记录单列，不混入 live count | 套餐配额、数据增长 |
| 逻辑业务数据字节 | 权威业务 JSON 的规范化字节，另加被明确定义为业务数据的关系字节；不含索引/WAL/表碎片 | 客户可见容量、配额 |
| 物理存储 | 分区表、索引、TOAST、WAL、空闲页、备份等实际基础设施占用 | 内部成本与容量治理 |

`request_id` 是一次入口请求的追踪标识；`execution_id` 是首次物理执行的稳定标识。计量去重以 `execution_id + meter_version` 为准，不能只依赖可重试的 `request_id`。

## 方案设计

### 分层架构

```text
API / MCP / CLI transport
  -> 轻量入口计数（entrypoint、状态、request_id）
  -> shared Capability Invoker
       -> 首次执行、RU 计算、execution_id
       -> 领域事务：记录变更 + usage outbox / ledger event + 当前值增量
  -> outbox worker（幂等）
       -> usage ledger append-only event
       -> 小时/日 rollup、看板缓存、低基数平台指标
  -> authorized usage capability
       -> current / time series / physical sample
```

`TASK-017` 的 transactional outbox 是 Phase 1 的前置依赖。写业务记录时，记录变更、逻辑字节/记录数 delta 和 outbox 事件必须处于同一数据库事务；任何一方未提交就不得对看板或账单显示成功。只读调用和被入口拒绝的请求由调用层创建紧凑的 usage event，不写业务数据事务。

### 数据存储位置

Phase 1 在现有 PostgreSQL 集群创建专用 `metering` schema，并沿用 tenant bucket、事务级 TenantContext 和 FORCE RLS 原则；不把用量数据放在应用日志、浏览器缓存或 Prometheus 标签中。

| 表/数据集 | 写入方式 | 作用 | 建议索引与分区 |
|---|---|---|---|
| `usage_ledger_event` | 追加写、不可修改 | 一次计量事实：入口、执行、RU、状态、时间、版本与 delta | 按月 range partition；`tenant_bucket, tenant_id, occurred_at` 索引；紧凑去重键 |
| `usage_dedup` | 幂等写 | 防止 outbox 重试或重放重复入账 | `metering_key` 唯一；保留至少覆盖 raw ledger 在线期 |
| `tenant_usage_current_bucket` | 同业务事务增量写 | 当前记录数、逻辑字节、当前周期 RU/请求数 | `tenant + object + counter_bucket`；将热点写分散到固定小桶 |
| `usage_rollup_hourly` / `usage_rollup_daily` | worker 幂等 upsert | 趋势、额度、账期和长期查询 | 按时间分区，按租户/Capability/meter 查询索引 |
| `physical_storage_sample` | 后台采样 | 分片、bucket、表/索引/WAL 的实际占用与采样版本 | 仅存汇总样本，不保存行级数据 |

Usage ledger 只保存最小计量字段：租户、时间、入口、Capability、状态、执行/去重键、数值、计量规则版本和必要追踪关联。禁止存储认证令牌、完整请求/响应、提示词或敏感业务字段。`company_id` 由受控运营查询通过 `tenant_id` 关联展示，不在每条 usage event 冗余复制。

### 实时性与用户体验

| 数据 | 可见性目标 | 一致性策略 | 不可采用的实现 |
|---|---|---|---|
| 入口请求、执行次数、RU | 执行完成后秒级 | ledger/outbox 最终一致；配额判断使用当前事务内值 | 每请求远程发指标或同步聚合历史数据 |
| 记录数、逻辑业务字节 | 业务事务提交后立即可读 | 同事务 delta + 定期全量校准 | 每次页面/API 查询全表 `count`/`sum` |
| 物理存储 | 分钟至小时级 | 只读后台采样与时间序列 | 请求路径运行 `pg_total_relation_size` |

请求路径只允许常数复杂度的字段计算、一次本地/同事务增量和紧凑 outbox 写入。若单一租户或对象形成热点，使用固定 `counter_bucket` 写分片，查询时汇总有限桶；不得把所有写入串行化到一行总计数器。任何新增开销必须由 Phase 1 的 P95/P99、锁等待、WAL 与吞吐基线证明，而非只凭设计假定。

### RU 初版规则

每个已发布 Capability 声明 `ru_policy_version` 和基础 RU。动态部分来自经验证的输入大小、批量记录数、查询扫描/排序/关系档位与返回大小；规则及阈值版本化并随 ledger 保存。Phase 1 不使用 PostgreSQL `EXPLAIN` 成本直接开票：它仅用于预检/限流，后续以真实耗时、行数和资源证据校准 RU。

### 物理存储的归因限制

`object_record` 当前按 tenant bucket 共用 PostgreSQL 分区。因此 Phase 1 对租户提供精确的“逻辑业务数据大小”，对平台提供准确的“分区物理大小”，但只提供按明确权重分摊的“租户物理成本估算”。若合同要求物理容量逐字节独享和可审计，必须使用 dedicated shard/database，而不是通过共享分区估算伪造精度。

## 保留、压缩与成本控制

初始默认值是产品策略，需在真正收费或受监管数据上线前经商业/法务确认；它们不是已经验证的 SLA 或合规承诺。

| 数据 | Phase 1 默认在线保留 | 长期保留 | 成本控制 |
|---|---:|---:|---|
| 原始 `usage_ledger_event` | 90 天 | 暂不承诺；后续可归档到加密对象存储 | 月分区，到期直接 drop partition；不逐行 delete/vacuum |
| 小时汇总 | 90 天 | 不归档 | 到期按分区删除 |
| 日汇总与已结周期快照 | 25 个月 | 收费后按合同/法定要求决定 | 数据量远小于原始事件 |
| 当前计数器 | 当前值 | 不适用 | 只存非零对象/桶，不保存历史快照 |
| 物理存储样本 | 90 天 | 不归档 | 仅存分片/bucket 汇总 |

实际存储容量必须在实现后通过真实 PostgreSQL 16.13 负载测量，不能预先宣称固定字节数。容量和成本按下式预算：

```text
raw_ledger_GiB = daily_physical_executions × raw_retention_days × measured_bytes_per_event / 2^30
rollup_GiB     = distinct(tenant, capability, meter, time_bucket) × retained_buckets × measured_bytes_per_row / 2^30
monthly_cost   = stored_GiB × provider_GiB_month_price + backup/WAL/IOPS/compute increment
```

其中 `measured_bytes_per_event` 和 `measured_bytes_per_row` 必须以迁移后的真实索引、TOAST 与分区结果测得。原始 ledger 是主要增长项；current aggregate、物理样本和日汇总通常相对很小。容量验收须报告每百万 execution event 的物理大小、写入 WAL 放大、索引大小、rollup 后查询延迟以及按上述公式在目标日执行量下的 90 天存储预测。

## 分阶段交付计划

### Phase 0 - 可靠事件基础

- 由 `TASK-017` 实现 transactional outbox、worker 幂等、重试、死信与积压告警。
- 确立 execution_id、metering_key、租户隔离与事件投递失败语义。

### Phase 1 - 四项关键用量指标（TASK-027）

- 实现上述 ledger、current bucket、rollup、physical sample 与受授权查询 Capability。
- 实现 API/MCP/CLI 入口和 Capability 执行的去重统计、RU、逻辑字节、有效记录数。
- 完成实时性、RLS、幂等、软删除/恢复、批量写、热点桶、校准和容量基准验证。
- 仅展示用量与趋势；不启用收费或自动停服。

### Phase 2 - 运行体验与可靠性

- 延迟、错误、限流、队列、worker、outbox、慢查询、锁等待、缓存/WAL、热点公平性与计量完整性指标。
- 引入低基数监控系统指标；`tenant_id`、`request_id` 等高基数字段仍留在 ledger/trace 中。

### Phase 3 - 成本与扩展用量

- AI token/模型成本、连接器、出网、导出、搜索/OLAP、自动化/worker 和归档成本。
- 成本归因、预测、预算、套餐额度与 dedicated shard 计价策略；收费前另立商业与合规决策。

## 接口与数据影响

- 新增 `metering` schema、迁移、RLS policy、outbox event contract、后台 worker 和保留任务；所有 schema 变更需在 fresh PostgreSQL 中顺序验证。
- Capability Contract 增加受授权的 usage 查询能力以及每项 Capability 的 RU policy metadata。普通租户只可读自身汇总；运营角色可跨租户汇总；原始 ledger 默认不对普通调用方开放。
- API/MCP/CLI adapter 必须传入可信 `entrypoint`，且不能接受调用方 body/header 自报入口类型。
- 记录创建、更新、软删除、恢复、批量变更和 backfill 必须产生正确 delta；Changeset/worker 的执行需有稳定 execution_id。
- 过期 raw 分区可被安全删除，但日汇总与已结快照不得被自动重算覆盖；数据校准差异必须审计。

## 验收标准

- 三入口对同一 Capability 调用均可显示入口请求数；一次幂等重放不增加 execution/RU；MCP 发现不增加 Tool 执行数。
- 每个首次执行有且仅有一组可去重的 ledger 计量事实；worker 重试、服务重启和并发重放不重复计量。
- 记录 CRUD、软删除/恢复、批量变更与失败回滚后，`tenant_usage_current_bucket` 的有效记录数和逻辑字节与独立全量重算一致。
- 共享分区环境中，接口清楚区分 logical business storage、physical shard storage 和 physical allocation estimate。
- usage 查询不返回其他租户用量；缺少 TenantContext、伪造入口、错租户/错 bucket、无权限跨租户读取均 fail closed。
- 在目标 Phase 0 数据集和并发下，计量开启后的 P95/P99、吞吐、锁等待、WAL 和磁盘增长与未开启基线对比有真实可复现证据；是否达标由实施前写入的阈值决定。
- 月分区保留任务、日汇总和对账任务可重复执行；不会扫描全表或逐行删除过期 raw ledger。
- `go test ./...`、`go test -race ./...`、`go vet ./...`、`go mod verify`、fresh migration、状态 validator 与 `git diff --check` 均通过后，才能将 TASK-027 标记完成。

## 风险与回滚

- **写放大或热点锁竞争**：使用紧凑事件、固定 counter bucket、批量 rollup；若基准不达标，关闭展示与异步 rollup，不删除业务数据也不启用收费。
- **重复/遗漏计量**：outbox + deterministic metering key + ledger 去重 + 全量校准；差异进入审计，不静默修订已结数据。
- **共享物理存储误归因**：只显示估算并标注口径；客户配额以逻辑字节为准。
- **高基数监控成本失控**：高基数只写 PostgreSQL ledger/trace，平台监控只保留有限标签。
- **隐私泄露**：ledger 禁止 payload、token 和敏感字段；所有查询受 TenantContext、RLS 和最小权限保护。

回滚时先停用新 usage 查询 Capability 和 worker，再停止新事件写入；已写 ledger 保留为审计事实。不得因回滚删除租户业务数据或对既有账期进行无痕修改。

## 实现进展与交接说明

- 当前状态：`approved`（仅设计获准）；尚未创建 migration、代码、worker、Capability 或部署。
- 前置：先完成 `TASK-017` 的通用 transactional outbox；Phase 1 还需在开始实现前把性能阈值、RU 初版规则和最终 raw retention 值写入 TASK-027 的实施计划。
- 接手顺序：阅读本规格、`FEAT-009` 第 14/15 节、`FEAT-020`、`FEAT-012`、`TASK-017`，然后确认当前 PostgreSQL schema 与可用数据库角色。不得把当前 in-memory Invoker audit 或 `audit_event` 直接升级为账单事实而不补齐上述幂等、入口和保留语义。
