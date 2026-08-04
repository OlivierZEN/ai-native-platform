---
kind: task
task_id: TASK-059
title: 补齐活动元数据首次权限引导并恢复 DEV Autopilot 权限发布
status: in_progress
priority: critical
owner_role: backend-agent
claimed_by: root
spec_path: docs/specs/FEAT-050-active-metadata-permission-bootstrap.md
depends_on: TASK-054
blocked_by: none
updated_at: 2026-08-04T07:20:00Z
updated_by: ai
---

# TASK-059 - 活动元数据首次权限引导

## 范围

- 修复 `permission-set.grant` 与 `role.attach-permission-set` 的新元数据首次授权死锁。
- 保留平台权限、未知/非活动元数据和无策略管理权主体的失败关闭行为。
- 修复对象通配访问权限被误用为再授权精确权限的问题，并提供正式原子权限撤销能力清理错误授权。
- 发布 Semattice 修复，并仅通过正式 Capability 完成 DEV Autopilot 事件对象权限和策略。

## 完成标准

- 定向与全量回归通过，生产发布可回滚。
- `dev_delivery_event` 保持 15 个字段，策略为 `enforced/private`。
- 产品总监、产品经理、开发者角色按最小权限绑定；哪吒 suspended 不可调用。
