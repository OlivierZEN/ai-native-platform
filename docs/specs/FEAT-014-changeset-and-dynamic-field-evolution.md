---
kind: feature-spec
feature_id: FEAT-014
title: Changeset publisher and safe dynamic field evolution
status: approved
owner_role: backend-agent
task_ids: TASK-014
related_decisions: ADR-003, ADR-007, ADR-008, ADR-010
related_issues: none
updated_at: 2026-07-20T07:09:11Z
updated_by: ai after TASK-014 implementation and final PostgreSQL verification
---

# FEAT-014 - Changeset 发布器与动态字段安全演进

## 背景与目标

`TASK-013` 已实现不可变元数据版本，`TASK-015` 已实现以 `object_record.data JSONB` 为权威值的记录 CRUD 和按需类型化索引。动态字段不需要修改业务表 DDL，但字段定义的新增、必填、索引、改名、改类型和删除仍会影响存量记录、查询完整性、写放大和回滚能力。

本规格定义 Changeset 发布器必须执行的安全演进协议和 Phase 0/1 产品边界。目标不是追求 PostgreSQL 的理论最大值，而是在 8 GiB/16 GiB、50 并发、200 活跃用户、1,000,000 条业务记录的 PoC 基线内，使每次字段变更都可预检、可观测、可中断恢复、可验证且不返回部分查询结果。

## 设计结论

### 动态字段与 JSONB 边界

以下是首轮产品护栏，不是 PostgreSQL 物理极限。`TASK-019` 必须按真实字段分布和读写比例验证，之后只能通过版本化配额策略调整。

| 维度 | 默认/目标 | 硬边界 | 超限处理 |
|---|---:|---:|---|
| 每对象在线动态字段定义数 | 300 | 500 | Changeset 预检拒绝；专属容量方案不得直接修改公共默认值 |
| 8 GiB 档每对象 active 类型化索引字段 | 10 | 20 | 新索引进入容量审批，不得边回填边开放查询 |
| 16 GiB 档每对象 active 类型化索引字段 | 20 | 40 | 超过默认值需提供对象记录量、选择性和写放大评估 |
| 平台绝对 active 类型化索引字段 | - | 50 | 仅专属分片和专项压测可批准，普通共享档拒绝 |
| 单条记录规范化业务 JSON 目标 | 平均不超过 4 KiB，P95 不超过 16 KiB | 256 KiB | 64 KiB 告警；超过 256 KiB 的最终记录拒绝写入 |
| 单个 `json` 类型动态字段 | 建议不超过 16 KiB | 64 KiB | 超限拆成子对象、关系记录或外部大值 |
| JSON 嵌套深度 | 不超过 4 层 | 8 层 | 解析和元数据校验拒绝 |
| 单数组元素数 | 建议不超过 100 | 1,000 | 超限使用关系边或子对象，不内嵌无界集合 |
| 文件、图片、富文本附件和二进制 | JSONB 只存不可伪造引用及必要元数据 | 不允许内联二进制/Base64 | 转对象存储或专用大值存储 |

计数规则：

- “在线动态字段”统计 `active`、`deprecated`、`purging` 状态；不能通过反复弃用字段绕过配额。完成数据清理并只保留审计 tombstone 后才释放名额。
- 查找、主从和唯一字段同时消耗动态字段配额与类型化索引配额；多对多关系还必须使用 `record_relation`，不能用 JSON ID 数组规避关系和配额治理。
- 索引配额按对象的 active 字段数计算，不按“某条记录是否有值”计算，因为容量与最坏写放大必须在发布前可预测。
- 记录尺寸按服务端解析、默认值和规范化完成后的 UTF-8 JSON 字节数检查，不以压缩后的 TOAST 大小或客户端请求压缩大小计算。

### 边界评估依据

1. PostgreSQL 16 的 TOAST-able 单值理论上限约为 1 GiB，但该数字只说明能否表示，不代表适合作为 OLTP 业务记录上限。JSONB 更新仍锁定整行；文档越大，序列化、校验、WAL、网络、TOAST 读取和 vacuum 成本越高。
2. 当前 1,000,000 记录实测中，两个类型化索引树合计 `737,198,080` bytes，约为每个“记录 × 索引字段” `368.60` bytes。按相同数据形状线性估算，20 个满覆盖索引字段约占 `7.37 GB`，40 个约占 `14.74 GB`，50 个约占 `18.43 GB`；尚未包含权威记录、唯一表、关系、WAL、膨胀和维护空间。因此 500 个字段定义可行，不代表 500 个字段可全部建索引。
3. 500 个字段定义对稀疏 JSONB 不会产生 500 个空槽位，但会增加元数据编译、权限矩阵、Agent schema、校验和依赖分析成本，因此保留 300 默认值和 500 硬边界。
4. 平均 4 KiB、P95 16 KiB 是容量规划目标；256 KiB 是异常记录的产品拒绝线。大文本、长集合和文件拆出后，绝大多数 CRM/业务对象无需接近硬边界。

参考 PostgreSQL 16 官方说明：[JSONB 文档设计与整行锁](https://www.postgresql.org/docs/16/datatype-json.html#JSON-DOC-DESIGN)、[TOAST 存储](https://www.postgresql.org/docs/16/storage-toast.html)、[PostgreSQL Limits](https://www.postgresql.org/docs/16/limits.html)。

## 字段生命周期

发布后的 `field_id` 和 `api_name` 均为稳定技术标识。显示 `label`、描述和语义可通过新元数据版本调整；技术改名必须创建新字段并迁移，禁止直接改 JSONB key。

```text
draft -> active -> deprecated_read_write -> deprecated_read_only
      -> hidden -> purging -> tombstone
```

- `active`：可由当前版本读写；是否可查询由独立 index state 决定。
- `deprecated_read_write`：仍兼容旧调用，但新能力描述发出弃用信息。
- `deprecated_read_only`：允许读取和迁移，拒绝新的业务写入。
- `hidden`：从普通发现结果隐藏，但运行时字典仍能识别存量 JSON key，避免把它误判为未知字段。
- `purging`：后台删除权威 JSON key、typed index、关系/唯一值和派生副本；不可重新激活。
- `tombstone`：只保留 ID、API name、审计和历史版本引用；名称不得在保留期内复用。

索引使用独立状态机：

```text
none -> building -> validating -> active -> retiring -> none
                     \-> failed
```

只有 `active` 索引可被查询规划器使用。`building`、`validating`、`failed` 或 `retiring` 状态均不得返回部分命中结果，也不得静默回退为无预算 JSONB 全表扫描。

## 变更安全矩阵

| 变更 | 是否可立即发布 | 强制流程 |
|---|---|---|
| 新增 optional、无默认值、无索引字段 | 是 | 发布新版本；旧记录缺失该 key 与 `null` 语义必须明确区分 |
| 新增仅作用于新记录的默认值 | 是 | `default_semantics=on_create`；不得声称存量记录已拥有该值 |
| 新增需要覆盖存量数据的默认值 | 否 | optional 发布、分批回填、覆盖率校验、再激活依赖配置 |
| 新增 required 字段 | 否 | 先以 optional 引入，回填并验证 100%，下一元数据版本才设 required |
| 新增 indexed/unique/reference 字段 | 否 | 先进入 `building` 并对新写双写，回填历史记录，验证覆盖率后切 `active` |
| 放宽约束 | 通常可以 | Changeset 仍需做依赖和兼容性检查 |
| 收紧长度、范围、枚举或 null 约束 | 否 | 扫描存量值，修复/回填，验证无违规后发布 |
| 修改字段显示名或说明 | 是 | 保持 `field_id` 与 `api_name` 不变 |
| 修改字段 API name | 禁止原地修改 | 新字段、新 API name、复制/转换、双读兼容、切换后弃用旧字段 |
| 修改字段数据类型 | 禁止危险原地强转 | 新字段 ID、显式转换器、错误隔离、回填校验、切换和旧字段保留期 |
| 删除字段 | 否 | 弃用、阻断依赖、停止新写、清理所有数据与派生状态，最后保留 tombstone |

## 分阶段发布协议

1. **Validate**：校验配额、名称稳定性、类型兼容、权限、依赖图、索引预算、记录尺寸风险和回滚条件。
2. **Simulate**：基于统计信息和受控样本输出受影响记录数、预计 typed rows、磁盘/WAL 放大、失败样本、预计耗时和是否需要专属分片。
3. **Prepare**：创建不可变候选元数据版本。需要回填的字段保持 optional，需要索引的字段保持 `building`；新写入按候选迁移协议维护新旧派生状态。
4. **Backfill**：按 `tenant_bucket + object_id + record_id` 稳定游标分批处理，保存 checkpoint；每批使用 TenantContext、幂等 operation ID、乐观版本检查和租户公平调度。
5. **Validate coverage**：至少验证 eligible records、processed、succeeded、skipped_conflict、failed、typed rows、关系/唯一冲突和抽样 digest。必填或索引激活要求 100% 合格记录覆盖且失败数为 0。
6. **Activate**：以控制面事务切换当前元数据版本和字段/index state。查询能力只能从此时开始公开新过滤、排序或唯一语义。
7. **Observe**：观察写错误、查询计划、延迟、WAL、表膨胀和回填后数据质量；超过门限停止推广。
8. **Retire/Purge**：经过兼容保留期后停止旧字段写入，再按相同可恢复协议清理；破坏性清理后只允许前向修复，不承诺指针回滚可恢复已删除值。

回填不得长时间持有跨批事务，不得关闭 RLS、使用 BYPASSRLS/superuser，也不得通过共享连接遗留租户上下文。发生并发更新时跳过并重试最新 revision，禁止回填覆盖用户的新写入。

## Changeset 数据与能力契约

Changeset 至少保存：`changeset_id`、tenant、base/candidate metadata version、state、operation digest、approval、risk level、quota snapshot、migration plan、checkpoints、coverage、失败样本引用、actor 和完整审计时间线。

建议原子能力：

- `metadata.changeset.validate`
- `metadata.changeset.simulate`
- `metadata.changeset.approve`
- `metadata.changeset.publish`
- `metadata.changeset.get-status`
- `metadata.changeset.backfill`
- `metadata.changeset.validate-coverage`
- `metadata.changeset.purge`
- `metadata.changeset.cancel`
- `metadata.changeset.rollback`

这些能力必须从同一 Capability Contract 等价投影为 API、MCP Tool 与非交互式 CLI。`backfill/purge` 每次只执行一个有界批次并返回 checkpoint/coverage，Agent 或后续 `TASK-017` worker 通过重复调用和 `get-status` 推进，不在单个数据库事务中承载整个长任务。CLI 只接受 flags/stdin JSON 并输出 JSON/JSON Lines，不提供菜单或确认提示。

## 当前实现边界

2026-07-20 `TASK-014` 已完成：

- migrations 5/6 已增加字段 lifecycle/index/default/unique/predecessor 语义、Changeset 与按对象 checkpoint、版本化 `governance_policy`、唯一值表及字段 tombstone；所有租户执行表均启用并强制 RLS。
- 十项 `metadata.changeset.*` 能力从同一 Capability Contract 投影 API、MCP 和 CLI。审批、发布、取消与回滚为状态幂等操作；破坏性 purge 需要独立受信审批。
- 回填按对象和 UUIDv7 record ID 有界处理，每条记录使用 savepoint 与 revision 乐观条件。并发写入发生时当前批次记录 conflict/retry，不覆盖用户值；下一批读取最新 revision 恢复。
- approved/backfilling/ready 候选存在时，新 create/update 会投影候选字段、唯一值、typed index 和 relation edge；普通读取仍只暴露当前 active 元数据视图。
- coverage 同时核对记录版本、required、typed index、unique、reference 和 purge key，只有 100% 覆盖且无失败时才能进入 `ready`；publish 在切换指针前持有 tenant route 排他锁并重新验证 coverage/base pointer。
- API name/data type 不能在原 `field_id` 上修改；技术改名或改类型通过新字段加 `predecessor_field_id` 显式复制/转换。删除必须沿单向 lifecycle 进入 purging/tombstone，物理删除 JSON key 和全部派生状态，墓碑名称不可复用，之后只允许前向修复。
- service tier 配额在 Changeset validate 时以 `policy_version` 冻结；standard/8 GiB 使用 500 字段与 20 active index 上限，dedicated-16g 使用 500/40。记录写入执行 256 KiB 总尺寸、64 KiB 单 JSON 字段、8 层和 1,000 数组元素硬限制，并在 API/MCP/CLI 返回一致稳定错误。
- `TASK-017` 的通用异步调度、死信和跨能力 worker 平台以及 `TASK-019` 的 8/16 GiB 最终容量认证仍不属于本任务；本任务已提供其可重复调用的有界批次能力与状态证据。

## 范围

### In Scope

- Changeset validate/simulate/approve/publish/status/cancel/rollback 状态机。
- 动态字段生命周期、索引生命周期、分批回填、覆盖率门禁和可恢复 checkpoint。
- 字段/索引/JSONB 尺寸配额的元数据预检和记录写入时硬校验。
- API/MCP/CLI 三入口的能力等价性、幂等、稳定错误和审计。

### Out Of Scope

- `TASK-016` 的业务数据权限和共享计算。
- `TASK-017` 的通用 transactional outbox/worker 平台；本任务可以定义所需 job port，但不得提前把该任务标记完成。
- `TASK-019` 的最终 8/16 GiB 并发容量认证。本文数值在该任务完成前属于保守产品护栏和待验证目标。
- 文件对象存储、全文搜索与 OLAP 的具体产品选型。

## 验收标准

- [x] 500 字段、索引分档、256 KiB 记录和 JSON 深度/数组边界均可由版本化策略配置，并在 API/MCP/CLI 返回同一稳定错误。
- [x] 新增 optional 字段不重写历史记录；`on_create` 默认值不会被错误解释为已回填。
- [x] required、indexed、unique、reference 和约束收紧在 coverage=100% 前无法激活。
- [x] 索引构建期间查询被明确拒绝且不返回部分结果、不做无界 JSONB 扫描。
- [x] API name/type 原地危险变更被拒绝；改名、改类型和删除路径有完整迁移与 tombstone 测试。
- [x] 回填可暂停、恢复、重试和处理并发 revision 冲突，始终使用 TenantContext 与 FORCE RLS。
- [x] 发布和回滚具有幂等状态转换、审批、审计、故障注入和三入口 parity 测试。
- [ ] `TASK-019` 对默认记录尺寸和 10/20 个 active typed-index 字段档位提供真实 8/16 GiB 容量证据。

## 风险与回滚

- 元数据指针回滚只适用于数据仍与旧版本兼容的阶段；进入 `purging` 后只能暂停和前向修复，不能承诺找回已物理删除的数据。
- 若容量模拟不可信，发布器应选择拒绝或要求专属压测，不得以 PostgreSQL 1 GiB 单值理论上限作为准入依据。
- 若回填失败，保持旧版本 active、新字段/index 不可查询；修复后从 checkpoint 重试，不清空或覆盖已经验证的数据。

## 实现进展与交接

- 当前状态：`approved / implemented`；`TASK-014` 已完成。只有外部依赖的 `TASK-019` 容量认证仍未勾选，不影响本任务代码关闭。
- 已验证：fresh PostgreSQL 16 migrations 1–6、全量测试与 race；有界回填暂停/恢复、真实锁竞争 revision conflict、unique 冲突修复、candidate 双写、relation coverage、predecessor 类型转换、purge/tombstone、破坏性回滚拒绝、版本化配额和 API/MCP/CLI parity。
- 下一阶段 `TASK-016` 在本实现之上加入对象/字段/记录权限；不得让权限过滤绕过 Changeset coverage、TenantContext 或 active-index 查询门禁。
- `TASK-019` 必须独立完成 8/16 GiB、50 并发、200 活跃用户和热点公平性认证，不能把本任务功能测试当成容量结论。
