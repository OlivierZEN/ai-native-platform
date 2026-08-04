---
kind: feature-spec
feature_id: FEAT-048
title: Governed Principal organization membership
status: verified
owner_role: integration-agent
task_ids: TASK-054
related_decisions: ADR-014
related_issues: none
updated_at: 2026-08-04T03:25:00Z
updated_by: ai
---

# FEAT-048 - 受治理 Principal 组织成员关系

## 背景

Semattice 已具备 Principal 投影、角色分配和状态治理能力，但缺少组织成员关系的正式写入 Capability。新增机器开发者时只能由运维直接写数据库，无法复用审批、幂等和审计门禁。

## 能力契约

新增 `identity.principal.set-organization-membership`：

- required scope：`authorization.manage`；
- risk：`high`，异步审批型；
- 调用者必须是已验证的 HUMAN 管理主体；
- `approval_id` 必须存在于已验签 OACT 的 approvals 声明中；
- 输入：`principal_id`、`organization_id`、`active`、`primary`、`approval_id`；
- 输出：目标成员关系的主体、组织、状态和 primary 标记。

## 写入语义

- `active=true` 时验证目标 Principal 与 active Organization 均存在；已存在关系则更新，缺失则创建。
- 设置新的 primary 关系前，结束同一 Principal 的其他 active primary 关系，保证最多一个主组织。
- `active=false` 时结束目标 active 关系并清除 primary 标记。
- 每次成功变更后删除目标 Principal 的 permission snapshot，并写入租户审计。
- 数据写入只通过 runtime RLS 事务执行，不接受调用方提供 tenant、actor 或 owner 身份。

## 验收

- 未携带已验签审批、机器主体调用、未知 Principal 或非 active Organization 均失败关闭。
- 建立、切换 primary、结束关系和权限快照失效均由 PostgreSQL 集成测试覆盖。
- API、MCP、CLI 从同一 Registry 发现新能力，生产能力发现与健康检查通过。
