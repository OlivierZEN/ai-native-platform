---
kind: feature-spec
feature_id: FEAT-025
title: Rename global provisioning company identity to company_id
status: verified
owner_role: backend-agent
task_ids: TASK-025
related_decisions: ADR-009, ADR-013
related_issues: none
updated_at: 2026-07-23T07:43:33Z
updated_by: ai after company_id protocol/database verification
---

# FEAT-025 - 全局开户身份改名为 `company_id`

## 背景与目标

统一运营控制面分配的 20 位企业业务编号此前称为 `org_id`，容易与租户内 RBAC/共享所使用的 `organization_id` 组织树混淆。本功能将前者统一改称 `company_id`，让“公司/全局租户身份”与“租户内权限组织”在 API、JWT、代码及数据库中有明确边界。

## 范围

### In Scope

- `tenant.provision`、`tenant.get-status` 和本地统一运营 port 的字段/模型改为 `company_id` / `CompanyID`。
- JWT claim、可信 principal、租户路由查询和 `tenant_registry` 列改为 `company_id`。
- 新增 checksum migration，将既有 `tenant_registry.org_id` 无损重命名为 `company_id`，并同步约束名称。
- 更新现行架构规格、状态与测试向量。

### Out Of Scope

- 改动 RBAC/共享的 `organization_id`、`organization_node`、组织合并或数据范围语义。
- 重编现有 20 位公司编号的值，或迁移 Agent CC/统一运营控制面外部数据。
- 在未获部署授权时修改 ECS、Nginx、远程数据库或远程 JWT 发行方。

## 兼容性与迁移

这是 Capability、JWT 与 operations-port 的破坏性 v2 字段变更：新请求和令牌必须使用 `company_id`；`org_id` 不再接受。数据库 migration 仅重命名列和约束，保留已存在的编号值与唯一性。当前编号格式继续兼容 `^org[a-z0-9]{17}$`；把值格式本身改为 `company...` 属于全局身份重键，需要外部运营系统共同迁移，未包含在本功能。

## 验收标准

- [x] 新 JWT `company_id` 绑定 tenant principal；`org_id` claim 被拒绝。
- [x] 13 个 migrations 从空库顺序执行；升级后 `tenant_registry.company_id` 仍唯一且路由可用。
- [x] API/MCP/CLI tenant capability schemas 与结果只使用 `company_id`。
- [x] `organization_id` 权限组织树保持不变，完整 PostgreSQL 与 race 回归通过。

## 实现与验证

- `0013_company_identity_rename.sql` 只重命名 `tenant_registry` 的列和相关约束，不改写已分配编号。
- tenant capability descriptors 升至 v2；旧输入及旧 JWT `org_id` claim 均 fail closed。
- 本地 fresh PostgreSQL、全量单元/集成、race、vet、模块校验、状态校验和差异检查通过。未部署到 ECS；已部署制品仍需独立升级。
