---
kind: task-status
task_id: TASK-040
status: done
updated_at: 2026-07-31T01:30:00Z
updated_by: root
assignee: root
owner_role: fullstack-agent
spec_path: docs/specs/FEAT-034-live-tenant-governance-console.md
---

# TASK-040 - 管理中心接入真实租户治理数据

## Current State

- 已完成：控制台以已验证的短时会话建立 TenantContext，所有治理 API 改为 runtime RLS 下的真实只读查询；内存 fixture 已删除。
- 生产已发布 `/opt/semattice/releases/20260731T012059Z-console`。真实受控会话读取目标公司返回对象/字段/成员/角色/组织为 `5 / 37 / 0 / 0 / 0`，对象目录为 `dev_change:7、dev_project:8、dev_requirement:8、dev_task:8、dev_worklog:6`。
- 已移除“演示环境 / 模拟治理数据”及 `example.demo` 成员；未投影的身份治理数据现在明确显示为空态。

## Scope

- `internal/console/**`、`cmd/ai-native-platform/main.go`、控制台静态文件及本特性规格/状态。
- 不修改发布的研发交付元数据、业务记录、AgentCiCi 用户、角色或 Semattice 授权模型。
