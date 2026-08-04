---
kind: feature-spec
feature_id: FEAT-052
title: Governed create-time record assignment
status: implemented
owner_role: backend-agent
task_ids: TASK-061
related_decisions: ADR-006
related_issues: none
updated_at: 2026-08-04T09:10:00Z
updated_by: ai
---

# FEAT-052 - 受治理的创建时记录派发

## 背景

受保护对象同时存在业务负责人字段和 Semattice `owner_principal_id`。DEV Autopilot 产品经理创建任务时只写业务 `owner`，会形成开发者可查询但无法按 `own` 数据范围更新的记录，任务认领因而被 PDP 正确拒绝。

## 设计

- `runtime.record.create` 增加可选 `owner_principal_id`；未提供时继续使用调用主体，兼容现有调用。
- 显式受派人与调用主体不同时，目标对象必须已启用 enforced 策略；受派人必须在同租户为 active Principal，并已通过角色获得目标对象的 read 与 update 原子权限。
- 受保护对象的数据组织锚点取受派人的 primary organization，不取创建者组织。
- 字段权限、对象策略、RLS、幂等、审计、容量计量与后续记录范围判断保持不变。
- 本能力只允许在创建事务中指定初始所有者，不提供既有记录的任意转移接口。

## 验收

- 产品经理可创建以开发者为真实记录所有者的 `dev_task`。
- 受派开发者可通过 own 范围更新任务，其他开发者不能更新。
- 缺少 read/update 权限、非 active 或无 primary organization 的受派人失败关闭。
- 不带新字段的所有既有创建请求行为不变。
