---
kind: feature-spec
feature_id: FEAT-054
title: Metadata object and field CRUD
status: implemented
owner_role: backend-agent
task_ids: none
related_decisions: ADR-003
related_issues: none
updated_at: 2026-08-05T10:55:00+08:00
updated_by: codex
---

# FEAT-054 - 元数据对象与字段 CRUD

## 背景与目标

Semattice 原有对象和字段公开能力只有 `upsert`，单条读取依赖整版元数据回读，也没有草稿删除能力。商城后台需要明确的增、删、改、查契约，并需要删除误建且没有业务数据的 `large_backpack` 对象。

本功能为对象和字段各增加 create、get、list、update、delete Capability，同时保留既有 `upsert`。已发布版本继续不可变，删除已发布定义必须通过候选草稿和 Changeset。

## 接口范围

### 对象

- `metadata.object.create`
- `metadata.object.get`
- `metadata.object.list`
- `metadata.object.update`
- `metadata.object.delete`

### 字段

- `metadata.field.create`
- `metadata.field.get`
- `metadata.field.list`
- `metadata.field.update`
- `metadata.field.delete`

所有能力继续使用统一 `POST /v1/capabilities/{capability-id}/invoke` 入口。读取使用 `metadata.read`，写入使用 `metadata.definition.write`。`update` 是完整替换，不是合并补丁；调用方必须携带稳定资源 ID 和完整定义。

## 删除规则

- delete 只直接修改 `draft` 元数据版本；数据库触发器继续拒绝修改已发布或历史版本。
- 删除草稿对象会级联删除同版本字段；同版本关系仍引用对象时返回冲突，不自动删除关系。
- 已发布对象可从候选版本移除，但 Changeset 校验要求该对象不存在任何历史业务记录，且既有关系不能从候选版本静默消失。
- 已发布字段只有在所有记录中都不存在该字段 API key 时才能从候选版本直接移除。
- 字段仍有存量值时，继续使用弃用、purge 和 tombstone 生命周期，不能用新 delete 绕过数据清理审批。
- 对象或字段移除在 Changeset 计划中分别记录为 `object_removed` 和 `field_removed`，风险等级为 `high`；发布仍需显式手工确认。

## 兼容与非目标

- 保留 `metadata.object.upsert` 与 `metadata.field.upsert`，本功能不要求现有调用方立即迁移。
- 不增加关系 CRUD、元数据版本删除或数据库物理清除历史快照。
- 不允许直接改数据库、跳过租户隔离、Changeset、审批、幂等或审计。

## 验收标准

- 对象和字段 create/get/list/update/delete 均由线上 Capability 发现返回正确 Schema、scope 和风险等级。
- 重复 create 返回 `CONFLICT`，不存在的 get/update/delete 返回 `RESOURCE_NOT_FOUND`。
- 对象 delete 返回被删除对象和级联字段数量；关系引用会阻止删除。
- 空对象和无存量值字段可通过 Changeset 移除并发布。
- 存在任何记录的对象、存在任何存量值的字段均被 Changeset 校验拒绝。
- API、CLI 和 Capability 注册表保持同一契约；全量 PostgreSQL 16 元数据测试通过。
- 生产回读不再包含 `large_backpack`，当前对象总数为 27。

## 当前实现与验证

- Go 服务、Capability Schema、对象/字段服务和 Changeset 演进规则已经实现。
- PostgreSQL 16 集成测试已验证草稿 CRUD、重复创建、列表过滤、对象字段级联删除、空定义 Changeset 删除和存量记录阻断。
- 生产发布、线上能力发现以及 `large_backpack` 删除尚待完成；完成前本文状态保持 `implemented`，不得声称线上已生效。

## 风险与回滚

- 候选版本激活前可取消 Changeset；激活后若没有数据迁移，可将活动指针回滚到上一不可变版本。
- 应用发布失败时将 `/opt/semattice/current` 切回前一 release 并重启；本功能没有数据库迁移。
- 删除仍有数据或依赖的定义会失败关闭，不提供自动级联删除业务记录、关系、权限或共享配置的行为。
