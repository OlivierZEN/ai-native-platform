---
kind: feature-spec
feature_id: FEAT-015
title: Metadata-driven record CRUD query and typed indexes
status: approved
owner_role: backend-agent
task_ids: TASK-015
related_decisions: ADR-003, ADR-007, ADR-008, ADR-009, ADR-010
related_issues: none
updated_at: 2026-07-20T07:09:11Z
updated_by: ai after TASK-014 record-runtime evolution integration
---

# FEAT-015 - 元数据驱动记录 CRUD、查询与类型索引

## 背景与目标

`TASK-012` 已建立 `object_record`、128 bucket、FORCE RLS 和同租户关系约束；`TASK-013` 已建立可发布、不可变的对象/字段/关系元数据。当前缺少把两者连接起来的业务记录运行时。

本任务交付可由 Agent 直接使用的记录 create/get/update/delete/query 原子能力。所有能力只接受结构化 JSON，经同一 Capability Registry/Invoker 投影为功能 API、MCP Tool 与非交互 CLI；不存在通用 SQL 或绕过元数据的数据库工具。

## 范围

### In Scope

- `runtime.record.create/get/update/delete/query` 五项 v1 Capability。
- 使用 JWT 绑定的 TenantContext 和租户当前 `metadata_version_id`，只允许已发布元数据参与运行时。
- 当前 published metadata 通过稳定 object/field ID 解释既有记录；发布兼容新版本不会隐藏旧版本记录或 typed rows，记录在后续 update 时原子迁移到当前版本。
- 严格对象/字段白名单、字段类型、required/default 和基础约束校验；未知字段 fail closed。
- UUIDv7 record ID、递增 revision、expected revision 乐观锁和默认软删除。
- `object_record` 补齐 metadata version、owner、actor、lifecycle 和 deleted time；保留 JSONB 为权威业务值。
- 仅为 `field_definition.indexed=true` 的 text/number/boolean/date/datetime/uuid 字段维护声明式类型索引；JSON 字段不建立通用索引。
- lookup/master-detail/many-to-many 关系值按 relation API name 写入 `record_relation`，目标记录必须是同租户 active 记录。
- 受限查询 DSL：AND predicates、操作符白名单、最大 100 条、record UUID 游标；动态字段过滤必须命中声明式类型索引，不静默 JSONB 全表扫描。
- 真实 PostgreSQL 16 CRUD、跨租户、元数据版本、乐观锁、软删除、关系、索引、查询计划、API/MCP/CLI parity 和 race 测试。
- 可重复的 1,000,000 记录物理基准脚本，报告写放大、表/索引存储、索引查询延迟和查询计划；该结果不是生产 SLA。

### Out Of Scope

- `TASK-016` 的对象级、字段级、记录级权限、共享规则和 permission snapshot。
- `TASK-017` 的 transactional outbox、事件 relay、重试和死信。
- Changeset、动态字段安全演进/回填、全文搜索、模糊搜索、OLAP、公式、汇总、自动编号、唯一字段治理、批量导入和物理清除。
- 8/16 GiB、50 并发、热点公平性和 HA/恢复验收；这些仍属于 `TASK-019`。

## 能力契约

| Capability | Scope | 语义 |
|---|---|---|
| `runtime.record.create` | `runtime.record.create` | 在当前已发布对象模型下创建 active 记录，生成或验证 UUIDv7 record ID，revision=1 |
| `runtime.record.get` | `runtime.record.read` | 按 object API name + record ID 读取；默认隐藏 deleted |
| `runtime.record.update` | `runtime.record.update` | 对 data 做 merge patch，要求 expected_revision，成功后 revision+1 |
| `runtime.record.delete` | `runtime.record.delete` | 要求 expected_revision，软删除并清理可查询索引/出边，revision+1 |
| `runtime.record.query` | `runtime.record.read` | 受限 AND DSL；无过滤时只做有界对象列表，动态过滤必须使用 typed index |

认证入口只要求 MCP/API/CLI payload 提供 `request_id` 和 `input`；tenant、actor 和 scopes 由 verified JWT 注入。所有能力支持统一 `idempotency_key`，create 还通过指定 record ID + 数据库主键防止重复写入。

## 数据设计

- `object_record` 新增 `metadata_version_id`、`owner_id`、`lifecycle_state`、`created_by`、`updated_by`、`deleted_at`，并把 `(tenant, metadata version, object)` 绑定到 `object_definition`。
- 类型索引按值类型拆表并按 128 tenant bucket 分区：`record_index_text/number/boolean/datetime/uuid`。`date` 与 `datetime` 共用 datetime 表并保留 `value_kind`。
- 每个类型索引行同时携带 tenant、metadata version、object、field、record；FK 同时约束字段定义和业务记录。
- `record_relation` 新增 nullable `metadata_version_id`，运行时新关系必须绑定已发布 relation definition；兼容 migration 1 的隔离测试骨架行。
- runtime role 仅获得上述业务表所需 DML，control role 不获得任何业务记录或索引权限。

## 动态字段边界与后续演进契约

`TASK-015` 的原始交付只覆盖“当前已发布模型下”的记录运行时；后续已完成的 `TASK-014` 按 [FEAT-014](./FEAT-014-changeset-and-dynamic-field-evolution.md) 增加候选投影、安全回填和 coverage。当前边界为：每对象在线动态字段默认/硬上限 300/500；8 GiB 与 16 GiB 档 active typed-index 字段默认值分别为 10/20，平台绝对上限 50；单条规范化业务 JSON 平均目标不超过 4 KiB、P95 不超过 16 KiB，64 KiB 告警，超过 256 KiB 拒绝。

这些边界尚未在当前源码中全部强制，不能误写为 `TASK-015` 已完成功能。当前实现的已知演进约束如下：

- 记录 JSON key 使用 field API name，因此首次发布后的 `api_name` 必须不可变；技术改名使用新字段与显式数据迁移。
- 当前更新会按当前对象模型校验完整记录，不能直接对存量对象新增 required 字段；必须 optional 引入、回填、验证后再激活 required。
- 当前 typed rows 只在 create/update 时维护，不能直接把新 indexed 字段设为可查询；必须先双写、回填并验证 100% coverage。
- 字段从当前模型消失后，旧 JSON key 会成为未知字段并可能阻断记录更新；删除必须在兼容字典仍有效时先清理权威值与全部派生状态，再保留 tombstone。
- typed row 与记录自己的 `metadata_version_id` 通过复合 FK 绑定；回填新版本字段时必须原子迁移记录版本并重建派生状态。

## 写入与查询规则

1. control router 解析并验证 tenant + org、active Native 状态、bucket 和当前 metadata version。
2. runtime pool 开启事务并设置 typed TenantContext。
3. 在同一事务读取 published object/field/relation definitions、校验 payload、写权威记录、重建该记录的 typed indexes 和 outgoing relation edges。
4. read/query 通过当前定义中的稳定 object/field ID 覆盖兼容的历史记录版本；update 先移除旧版本派生行，再使用 `revision = expected_revision` 原子更新版本并重建，0 行影响区分 not found 与 conflict。
5. query builder 只从固定表名、列名和操作符白名单生成参数化 SQL。同字段范围条件合并后由 materialized typed-index candidate set 驱动，其余字段使用 typed-index `EXISTS` 求交；不接受 SQL 字符串、任意 JSON path 或未索引字段。

## 基准方法

- 基准数据集：单租户、单 published object、1,000,000 active records；每条包含一个 indexed text 和一个 indexed number 字段。
- 数据装载：可重复脚本在专用本地 PostgreSQL 16 容器创建隔离 schema，通过集合 SQL 写入与运行时一致的物理行。
- 输出：object rows、typed-index rows、写入耗时、关系总尺寸、平均每记录字节、索引行/记录写放大、200 次等值和范围查询 p50/p95，以及 `EXPLAIN` 是否使用 typed index。
- 退出条件：行数与预期一致；写放大精确等于已声明索引字段数；等值和范围查询计划不得回退为 `object_record` 全表扫描。延迟和尺寸为本机证据，不宣称生产 SLA。

## 验收标准

- [x] migrations 可从空库执行并可重复校验 checksum。
- [x] 五项能力均从 Registry 投影为 API/MCP/CLI，成功和稳定错误 parity 通过。
- [x] 未发布/错误对象、未知字段、类型错误、required 缺失、非 UUIDv7、revision 冲突全部拒绝。
- [x] create/get/update/delete/query、默认值、软删除隐藏、跨租户隔离和连接池复用测试通过。
- [x] 只有 indexed 字段产生 typed rows；更新/删除同步维护索引；未索引 filter 被拒绝。
- [x] relation target 同租户 active 校验和数据库复合 FK 测试通过。
- [x] 1,000,000 记录基准具有真实、可重复的命令和结果记录。
- [x] 全量 test/race/vet/module verify、状态 validator 和差异检查通过。

## 风险与回滚

- migration 4 只增加列、表、索引、约束和 grants，不删除 migration 1 的数据；新列对历史骨架行保持 nullable 兼容，只有新 runtime 写入强制完整值。
- 若 typed index 路径错误，禁止降级到无预算 JSONB 扫描；能力返回稳定 precondition/internal error。
- 若任务无法满足隔离或元数据约束，保持 Capability 未发布并回到 `review`，不得放宽 RLS 或使用 owner/superuser 连接规避。

## 实现进展

- 当前状态：`completed`；`TASK-015` 已于 2026-07-19 本地验证完成。
- migration 4 补齐业务记录生命周期字段、版本绑定、持久幂等表，以及 text/number/boolean/datetime/uuid 五类共 640 个 bucket partitions；父表和子分区均 FORCE RLS。typed index 与 relation source 通过复合外键绑定同 tenant、metadata version 和 record。
- `internal/record` 实现五项原子能力，共享 published metadata 校验、UUIDv7、revision 乐观锁、软删除、derived-state 重建和稳定错误映射；写操作的 `idempotency_key` 可跨 Invoker 重启重放。
- API、MCP、CLI 对五项能力的成功结果和失败错误码/消息 parity 均通过；实际 `main` 双连接接线的 capability list 包含全部五项。
- PostgreSQL 16 真实测试覆盖 draft 对象拒绝、字段/类型/default/required、typed rows、关系 target、被引用删除、跨租户、无 TenantContext、权限、640 分区、游标和查询计划。

## 100 万记录实测证据

命令：

```sh
AI_NATIVE_RUN_RECORD_BENCHMARK=1 \
TEST_DATABASE_URL='postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable' \
go test ./internal/record -run '^TestRecordPhysicalBenchmark$' -count=1 -v -timeout 10m
```

- 记录：1,000,000；typed-index rows：2,000,000，精确为 2 行/记录。
- 集合写入：记录 6.493 s；两个 typed indexes 61.228 s。
- 物理尺寸：object 580,608,000 bytes；text index tree 416,006,144 bytes；number index tree 321,191,936 bytes；合计 1,317,806,080 bytes，即 1,317.81 bytes/record。
- 真实 bounded query builder 的 200 次等值/范围查询：p50 0.503 ms，p95 1.400 ms。
- `EXPLAIN (FORMAT JSON)` 命中 `record_index_number_b007_value_idx`，未对 `object_record_b007` 做全表扫描。

以上是当前本机专用容器的物理设计证据，不是 8/16 GiB、50 并发或生产 SLA；完整容量验收仍由 `TASK-019` 承担。

## 交接说明

先读取 FEAT-012、FEAT-013、本规格和 FEAT-014。业务数据运行时必须继续使用 runtime pool + TenantContext；control pool 只用于租户路由。`TASK-014` 已有 candidate/backfill/coverage 实现必须保持；不得提前把 `TASK-016/017` 标记完成，也不得通过添加通用 SQL Tool、允许部分索引查询或放宽未知字段校验来掩盖权限或字段演进问题。
