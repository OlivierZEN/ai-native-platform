---
kind: task-status
task_id: TASK-034
status: in_progress
updated_at: 2026-07-31T01:20:00Z
updated_by: root
assignee: root
owner_role: fullstack-agent
spec_path: docs/specs/FEAT-034-live-tenant-governance-console.md
---

# TASK-034 - 管理中心接入真实租户治理数据

## Current State

- 已核验线上公司 `org5nszpgj99jaysxv6y` 对应 tenant `cbcb9ad2-1ac1-50b2-a833-605884b566c1`：状态 `active`，metadata v1 为 `published`，有 5 个对象、37 个有效字段。
- 已核验本地 Semattice 角色、成员与组织投影均为 0；现有控制台 8 个 `example.demo` 成员及 12/84 元数据均为过时 fixture，必须移除。
- Next action: 以已验证会话上下文建立只读、RLS 约束的 console reader，更新 UI 空态并上线验证。

## Scope

- `internal/console/**`、`cmd/ai-native-platform/main.go`、控制台静态文件及本特性规格/状态。
- 不修改发布的研发交付元数据、业务记录、AgentCiCi 用户、角色或 Semattice 授权模型。
