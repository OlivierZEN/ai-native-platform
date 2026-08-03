---
kind: task-status
task_id: TASK-052
status: in_progress
updated_at: 2026-08-03T05:13:45Z
updated_by: root
assignee: root
owner_role: fullstack-agent
spec_path: none
---

# TASK-052 - 修复管理中心成员与角色查询

## Current State

- 已验证根因为聚合查询按未分组的 `p.created_at` 排序，不是登录、租户映射、权限或数据缺失。
- `p.created_at` 已纳入 GROUP BY，新增测试通过真实 control/runtime 角色、TenantContext 和 RLS 调用 members reader。
- PostgreSQL 16 全量集成、vet、module verify 和 Linux/amd64 构建已在修复分支通过。
- 临时 release 已线上验证 members=200；CodeUp 生产主线已合并，统一全量验证通过，等待提交和最终 release。
- Blocked: none

## Evidence

- 目标企业：`org5nszpgj99jaysxv6y`；租户：`cbcb9ad2-1ac1-50b2-a833-605884b566c1`。
- PostgreSQL 原始错误：`column "p.created_at" must appear in the GROUP BY clause or be used in an aggregate function`。
- `./scripts/test-postgres.sh run`：修复分支全仓库通过，`internal/console` 8.781s。
- 临时 production release：`/opt/semattice/releases/20260803T045726Z-web-oidc-2b29dc5efb47`，二进制 SHA-256 `36fbd1451d211865599ba6e0b0bcbb0d0de435cd35fddf57505f80e09bbb4749`。
- 临时 release 线上 members=200，返回三名研发主体和三个角色；overview 为 3/3/1/5/42。
- 合并主线：CodeUp `df308b1` + members 修复提交线；无冲突标记，状态 validator 通过。
- 合并后 `./scripts/test-postgres.sh run`、`go test -race ./...`、vet、module verify、16 项 Skill Python 测试、JS/Shell 和 Linux/amd64 构建全部通过；构建 SHA-256 `b9e79c0b43645a274fcb69e8f6d844b2e926553fda2d99e722abab6405952425`。

## Next Action

- 提交合并，发布最终统一 release 并重新验收。
