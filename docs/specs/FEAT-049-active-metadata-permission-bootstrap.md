---
kind: feature-spec
feature_id: FEAT-049
title: Governed active metadata permission bootstrap
status: implemented
owner_role: backend-agent
task_ids: TASK-058
related_decisions: ADR-006
related_issues: none
updated_at: 2026-08-04T06:24:00Z
updated_by: ai
---

# FEAT-049 - 活动元数据首次权限引导

## 背景

Semattice 的权限委托默认要求授权人已持有准备授出的同一项原子权限，以阻止权限管理员把管理权扩大为任意业务数据权限。该规则对已有对象有效，但新元数据版本首次发布后，新对象和字段尚无任何授权，因而没有主体能够满足“先持有、再授出”的条件，形成正式 Capability 无法解除的首次授权死锁。

DEV Autopilot 的 `dev_delivery_event` 已通过独立审批发布，随后在最小权限初始化时真实触发该问题。修复必须保留平台权限的严格禁止升级语义，且不能通过直接数据库写入绕过 Capability、审批、RLS 和审计。

## 决策

- `authorization.permission-set.grant` 仍优先执行现有精确权限委托校验。
- 只有调用主体已经持有独立的 `platform / authorization.policy / update` 权限时，才可为租户**当前活动元数据版本**中的对象或在线字段引导对象/字段权限。
- 平台权限、未知 UUID、历史元数据版本、已 tombstone/purging 的字段均不适用引导规则，继续失败关闭。
- `authorization.role.attach-permission-set` 对同一受限场景使用一致校验；Permission Set 中任何未持有的平台权限或非活动元数据权限都会阻断整个绑定。
- 既有 verified approval、`authorization.manage` scope、管理权限、租户 RLS、幂等和持久审计要求全部保持不变。

## 验收

- 策略管理员可以首次授予并绑定当前活动元数据对象和字段权限。
- 随机对象 UUID、历史/离线字段和未持有的平台权限仍返回 `UNAUTHORIZED`。
- 既有“不能把授权管理权扩大为任意平台权限”的回归不变。
- PostgreSQL 定向测试、全量测试、race、vet、module verify 和生产健康/匿名负例通过。
