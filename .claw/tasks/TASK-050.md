---
kind: task-status
task_id: TASK-050
status: done
updated_at: 2026-08-03T05:02:04Z
updated_by: root
assignee: root
owner_role: fullstack-agent
spec_path: none
---

# TASK-050 - 修复管理中心成员与角色查询

## Current State

- 生产 `/console/api/members` 返回 500，其他治理页面正常。
- 已验证根因为聚合查询按未分组的 `p.created_at` 排序，不是登录、租户映射、权限或数据缺失。
- `p.created_at` 已纳入 GROUP BY，新增测试通过真实 control/runtime 角色、TenantContext 和 RLS 调用 members reader。
- PostgreSQL 16 全量集成、vet、module verify 和 Linux/amd64 构建通过。
- 生产 release 已上线，HTTPS members 接口与控制台其他只读接口通过；发布后无 warning 日志。
- Blocked: none

## Evidence

- 目标企业：`org5nszpgj99jaysxv6y`；租户：`cbcb9ad2-1ac1-50b2-a833-605884b566c1`。
- PostgreSQL 原始错误：`column "p.created_at" must appear in the GROUP BY clause or be used in an aggregate function`。
- `./scripts/test-postgres.sh run`：全仓库通过，`internal/console` 8.781s。
- `go vet ./...`、`go mod verify`、Linux/amd64 build：通过；验证制品 SHA-256 `08b654e8cfceca3c31b5dc446ed8bd5a6c7b587127f004a75922ff03fc705c68`。
- production release：`/opt/semattice/releases/20260803T045726Z-web-oidc-2b29dc5efb47`，二进制 SHA-256 `36fbd1451d211865599ba6e0b0bcbb0d0de435cd35fddf57505f80e09bbb4749`。
- 线上 members=200，返回产品总监 Olivier、研发交付产品经理、研发交付开发者和产品总监/产品经理/开发者三个角色；overview 为 3/3/1/5/42。

## Next Action

- 已完成；常规监控成员接口和当前 release。
