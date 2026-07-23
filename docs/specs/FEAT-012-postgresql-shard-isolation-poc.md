---
kind: feature-spec
feature_id: FEAT-012
title: PostgreSQL shard and tenant isolation proof of concept
status: approved
owner_role: backend-agent
task_ids: TASK-012
related_decisions: ADR-008, ADR-009
related_issues: none
updated_at: 2026-07-18T17:19:29Z
updated_by: ai maker after independent checker PASS
---

# FEAT-012 - PostgreSQL 分片与租户隔离 PoC

## 目标

在一个专用 PostgreSQL 16 实例验证逻辑 OneDatabase 的最小物理基线：控制表、单 shard、固定 128 个 bucket、强制 RLS、受信 TenantContext、同租户关系约束和连接池复用隔离。当前只证明安全/正确性基线，不宣称生产容量或 HA。

## 表与角色

- 控制面：`shard_registry`、`tenant_registry`、`tenant_operation`、`audit_event`。
- 租户数据面：`object_record` 按 `tenant_bucket` LIST 分成 128 个分区；`record_relation` 保存 source/target 并强制同一 `tenant_id + tenant_bucket`。
- owner/migrator 拥有 DDL；runtime role 为 `NOLOGIN/NOBYPASSRLS` 或测试可登录非 owner，不授予表 owner/BYPASSRLS。
- 跨租户控制面使用独立 `ai_native_control` 角色；它同样非 owner、非 superuser、非 BYPASSRLS，只在 registry/operation/audit 三表获得显式 RLS policy 和最小 grants。runtime role 不继承该 policy。
- 所有含租户数据的表使用 PostgreSQL `uuid tenant_id NOT NULL`，执行 `ENABLE ROW LEVEL SECURITY` 与 `FORCE ROW LEVEL SECURITY`。

## TenantContext

数据库事务开始后以 `set_config(..., true)` 写入：

- `app.tenant_id`：规范 UUID。
- `app.tenant_bucket`：服务端 registry 解析的 `0..127`。
- `app.actor_id`：已验证主体。

RLS policy 同时核对 tenant 与 bucket。缺失、非法、不匹配或调用方自报 bucket 均 fail closed。上下文只在当前事务有效；commit、rollback、handler error 和 panic 恢复后不得残留到复用连接。

## 放置与分区

- Phase 0 只有 `shard-001`，但路由仍通过 registry。
- 新租户从 128 个 bucket 中选择当前租户数最少者，同数时选择较小 bucket；请求不得指定 shard/bucket。
- 查询必须同时携带 tenant 和 bucket，使 planner 可裁剪至单分区。

## 安全测试矩阵

- 同租户读写成功；另一有效 UUID 不可见、不可更新、不可关联。
- 缺少 TenantContext、错误 bucket、伪造 payload/header tenant 均拒绝。
- MaxConns=1 下跨两个租户连续执行 commit、rollback、error 与 panic 路径，无上下文残留。
- 查找/主从/多对多 source-target 跨租户和跨 bucket 均由约束或确定性事务检查拒绝。
- `pg_class.relrowsecurity/relforcerowsecurity` 和角色 `rolbypassrls=false` 有断言。
- `EXPLAIN` 证明带 tenant+bucket 的记录查询只访问一个 bucket 分区。

## 验收标准

- [x] 控制表和 128 个分区可由版本迁移从空库创建并重放。
- [x] runtime role 不拥有表、不 BYPASSRLS，所有租户表 FORCE RLS。
- [x] 安全测试矩阵在真实 PostgreSQL 16 通过。
- [x] TenantContext API 只接受 typed UUID、服务端 bucket 与 actor，并始终在事务中使用。
- [x] 分区裁剪和均衡放置具有可重复测试证据。

## 完成证据

- migration 1 从空库创建 shard/tenant/operation/audit 表、`object_record` 的 128 个 LIST partitions、`record_relation` 同租户 source/target 复合外键，以及 runtime grants。
- 真实 PostgreSQL 16 断言 runtime role 非 owner、非 superuser、`rolbypassrls=false`；5 个父表和 128 个分区均启用并强制 RLS。
- `MaxConns=1` 下依次覆盖 tenant A/B、错误 bucket、缺失上下文、commit、业务 error rollback 与 panic rollback；连接上的 tenant/bucket/actor 均无残留。
- 另一有效 UUID 的写入被 RLS 拒绝，跨租户 target relation 被复合 FK 拒绝；合法同租户关系成功。
- `EXPLAIN FORMAT JSON` 只访问指定 bucket 分区；least-used placement 在 bucket 0 使用前后确定性返回 0、1。
- 带真实数据库的全量 race tests、vet、module verification 和差异检查通过；专用容器已清理。
- 独立 checker 首轮发现单 pool/超级用户绕过后，migration 3 新增受限 control role，配置和 main 强制 control/runtime 两个 URL 同时出现；真实二进制接线测试以 control role 完成 provision，再以 runtime role + control router 创建 metadata draft。
- 第三轮独立 checker 从 fresh PostgreSQL 16.13 空库执行 migrations 1/2/3，确认 128 分区、FORCE RLS、runtime 无上下文 fail-closed、control 精确三表权限和全量数据库 race 均 `PASS`。

## 非目标

8/16 GiB 的完整 50 并发、200 活跃用户、100 万记录容量验收仍属于 TASK-019；本任务只建立其 schema 和隔离前提。不执行 HA、备份、恢复或生产调优。
