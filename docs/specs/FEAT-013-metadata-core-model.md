---
kind: feature-spec
feature_id: FEAT-013
title: Metadata version object field and relation core model
status: approved
owner_role: backend-agent
task_ids: TASK-013
related_decisions: ADR-003, ADR-004
related_issues: none
updated_at: 2026-07-18T17:19:29Z
updated_by: ai maker after independent checker PASS
---

# FEAT-013 - 元数据核心模型

## 目标

实现租户级元数据版本、对象、字段、关系和不可变发布快照。元数据是 API/MCP/CLI 与 Agent 的平台协议，不依赖任何前端页面。所有定义在 draft 中变更，发布时确定性编译并以 digest 固化。

## 核心模型

- `metadata_version`：UUIDv7、tenant、sequence、status(`draft|published|retired`)、snapshot、SHA-256 digest、created/published actor/time。
- `object_definition`：UUIDv7、tenant/version、稳定 `api_name`、label/description、semantic JSON。
- `field_definition`：UUIDv7、tenant/version/object、`api_name`、data type、required/indexed、default/constraints/semantic JSON。
- `relation_definition`：UUIDv7、tenant/version、source/target object、`lookup|master_detail|many_to_many`、delete behavior、semantic JSON。
- 所有 FK 携带 tenant/version 复合键，数据库拒绝跨租户或跨版本引用。

## 不变量

- API name 使用小写字母开头的稳定 snake_case，在同一父范围唯一；保留平台前缀。
- 字段类型首批支持 `text|number|boolean|date|datetime|uuid|json`，发布前验证 required/default、索引适配和关系完整性。
- published/retired 版本及其定义不可修改或删除；新变更必须创建新 draft。
- UUIDv7 在应用边界生成；数据库仍使用原生 `uuid`。
- 编译按 API name、稳定 ID 排序，规范化 JSON；相同逻辑输入产生相同 snapshot bytes 和 SHA-256 digest。
- 发布在单个 TenantContext 数据库事务中完成验证、快照、状态切换和审计；失败不产生半发布版本。

## 原子能力

- `metadata.version.create`
- `metadata.object.upsert`
- `metadata.field.upsert`
- `metadata.object.create/get/list/update/delete`
- `metadata.field.create/get/list/update/delete`
- `metadata.relation.upsert`
- `metadata.version.publish`
- `metadata.version.get`

每项由同一 Capability Definition/Invoker 投影为功能 API、MCP Tool 和非交互 CLI。写能力要求元数据管理 scope；publish 为高风险能力，需要显式批准上下文和幂等 operation。

## 验收标准

- [x] migration 建立模型、复合约束、FORCE RLS 和 published 不可变触发器。
- [x] 对象/字段/关系的合法创建、upsert 和引用测试通过。
- [x] 跨租户、跨版本、重复 API name、非法类型和已发布修改全部拒绝。
- [x] 输入顺序不同但逻辑相同得到相同 snapshot 和 digest。
- [x] 发布失败原子回滚，成功后版本与定义不可变。
- [x] 六个原子能力的 API/MCP/CLI 成功与稳定错误 parity 通过。

## 完成证据

- migration 2 创建 `metadata_version/object_definition/field_definition/relation_definition`，所有 FK 携带 tenant/version，四表 FORCE RLS，runtime 非 owner；数据库触发器拒绝 published/retired 版本及定义的后续修改。
- 应用边界生成并验证 UUIDv7。测试复用同一组稳定定义 ID，在两个 draft 中以相反顺序写入，编译得到完全相同的 snapshot bytes 和 SHA-256 digest。
- 对象、字段、lookup 关系、类型、API name、source/target 引用均有正向用例；跨租户、跨版本、重复 API name、非 UUIDv7 和已发布修改均返回稳定错误。
- publish 要求 JWT principal 中存在与输入匹配的独立 approval ID，在一个 TenantContext 事务内锁定 draft、编译、写 snapshot/digest、发布并更新租户当前版本；无 approval 和空版本不发布。
- 六项定义进入同一 Registry；authenticated API、agent CLI、authenticated MCP 的 `metadata.version.get` 返回等价 Bundle，通用投影覆盖其余原子能力。
- 从空 PostgreSQL 16 容器执行全部 migration 和测试通过；真实数据库全量 race、vet、module verification 和差异检查通过。
- 独立 checker 首轮发现单 pool 预路由问题后，main 和测试均改为 control router + runtime data pool：只有 `ai_native_control` 可跨租户解析 route，所有元数据 CRUD/publish 仍由 `ai_native_runtime` 在事务级 TenantContext 和 FORCE RLS 下执行。
- 第三轮独立 checker 确认 fresh migrations、双角色 main 接线、确定性快照、不可变发布和六项能力 API/MCP/CLI parity 全部 `PASS`。

## 非目标

Changeset 审批/回滚、包依赖、公式、权限、记录运行时和自动化分别由 TASK-014 及后续任务实现；本任务不提前扩展。
