---
kind: task
task_id: TASK-062
title: 修复未计量历史记录缩小时阻断运行时更新
status: in_progress
priority: critical
owner_role: backend-agent
claimed_by: root
spec_path: docs/specs/FEAT-051-metering-baseline-tolerance.md
depends_on: TASK-059
blocked_by: none
updated_at: 2026-08-04T14:35:00Z
updated_by: ai
---

# TASK-062 - 计量基线容错

## 范围

- 保证导入、历史种子或启用计量前创建的记录在缩小 JSON 数据时仍可更新。
- 当前用量物化计数不得低于零，真实增量继续写入不可变计量账本。
- 回填生产租户的当前记录数与逻辑数据字节基线，并完成 DEV Autopilot 评审更新验收。

## 完成标准

- 全量测试、race、vet 与 module verify 通过。
- 生产发布可回滚，当前用量与活动记录重算结果一致。
- 产品经理将任务从“设计待确认”更新为“设计驳回”时不再触发计量约束错误。
