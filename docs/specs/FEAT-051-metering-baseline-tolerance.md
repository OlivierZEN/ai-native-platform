---
kind: feature-spec
feature_id: FEAT-051
title: Metering baseline tolerance for imported records
status: implemented
owner_role: backend-agent
task_ids: TASK-062
related_decisions: ADR-006
related_issues: none
updated_at: 2026-08-04T14:35:00Z
updated_by: ai
---

# FEAT-051 - 未计量历史记录的基线容错

## 问题

运行时记录更新会把 JSON 字节变化同步到 `tenant_usage_current_bucket`。对于通过受控初始化、导入或计量功能启用前形成的记录，当前用量桶可能尚未包含该记录；当第一次更新使 JSON 变小时，负增量会把物化计数降到零以下并触发数据库约束，进而错误地回滚正常业务更新。

## 决策

- 当前用量是可重建的物化视图，记录数和逻辑字节数始终以零为下界。
- 不改变不可变 `usage_ledger_event` 与小时增量中的真实变化量，保留审计和后续重算依据。
- 生产发布时按活动 `object_record` 重建当前租户的对象/分桶基线，消除历史欠计量。
- 领域记录更新和计量写入仍处于同一事务；除缺失历史基线外，不放宽配额和用量一致性约束。

## 验收

- 空基线上的负记录/字节增量不会产生负当前用量或阻断记录更新。
- 已有正基线仍按真实增减更新，且不会跨越零下界。
- 生产基线重算后，活动记录数与序列化 JSON 逻辑字节数和物化桶汇总一致。
