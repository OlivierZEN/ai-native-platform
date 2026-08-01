---
kind: feature-spec
feature_id: FEAT-035
title: 研发身份投影、负责人关系与租户角色治理
status: approved
owner_role: integration-agent
task_ids: TASK-049
related_decisions: ADR-012, FEAT-033
related_issues: none
updated_at: 2026-08-01T13:39:26Z
updated_by: root
---

# FEAT-035 - 研发身份投影、负责人关系与租户角色治理

## 背景与目标

DEV Autopilot 需要在租户 `cbcb9ad2-1ac1-50b2-a833-605884b566c1` 中形成三类可治理账号：绑定 AgentCiCi 全局用户 `18611892001` 的产品总监 HUMAN Principal，以及由该产品总监负责的产品经理、开发者 SERVICE Principal。AgentCiCi 是全局身份和凭据权威；Semattice 保存租户内 Principal 投影、负责人关系、角色、权限和审计事实。

FEAT-033 已验证 OACT 的 HUMAN/SERVICE claims，但当前 `principal_projection` 只能由测试或运维 SQL写入，管理中心也只能显示 principal ID。本功能补齐安全的自同步、状态治理和真实展示。

## 范围

### In Scope

- 扩展 `principal_projection`：显示名、公共编号、SERVICE owner、client ID、权威来源和最后同步时间。
- `identity.principal.sync` 只根据已验签 OACT 投影当前调用者，禁止请求体指定另一个 principal ID、类型、owner 或 client ID。
- SERVICE 同步必须携带 AgentCiCi 签发的 `owner_principal_id` 和 `client_id`，且 owner 已是同租户有效 HUMAN 投影。
- `identity.principal.list` 和 `identity.principal.set-status` 供持有 `authorization.manage` 的人类管理主体查询、暂停、恢复或禁用投影。
- 管理中心展示真实账号名称、类型、公共编号/client ID、负责人、角色和状态。
- 生产建立产品总监、产品经理、开发者投影，配置产品总监、产品经理、开发者三个最小权限角色并完成审计。

### Out Of Scope

- Semattice 不创建或保存 Keycloak client secret，不签发身份令牌，不成为全局账号权威。
- 不允许原始 Keycloak token；所有入口继续只接受 AgentCiCi 短时 OACT。
- 不把 DEV Autopilot 项目业务数据复制进身份表。

## 安全模型

1. API/MCP/CLI 入口先由现有 JWKS verifier 验证 OACT。
2. `identity.principal.sync` 从 `request.Principal` 取 tenant、principal type、owner、client 和 scope；输入只允许非安全显示字段。
3. HUMAN 首次同步创建 `user` 投影；SERVICE 首次同步创建 `service` 投影并以同租户 active HUMAN owner 为外键。
4. 已暂停/禁用投影不能通过同步自行恢复；只有 HUMAN 管理主体可调用状态治理能力。
5. DevAutopilot 在每个 CLI 任务请求前调用 sync，因此被暂停的机器主体无法继续读写研发任务。
6. 所有能力使用现有 request/idempotency/audit/RLS 边界。

## 能力契约

- `identity.principal.sync`，scope `identity.principal.sync`，输入：`display_name`、`public_id`（可选显示元数据）；输出当前投影。
- `identity.principal.list`，scope `authorization.manage`，输入可按 type/status 过滤。
- `identity.principal.set-status`，scope `authorization.manage`，高风险审批能力，输入 `principal_id`、`status=active|suspended|disabled`、`reason`、`approval_id`；审批必须出现在已验签 OACT `approvals` 中。

投影状态变化不修改 AgentCiCi 的全局 Principal。全局恢复后仍须由 Semattice 管理主体显式恢复租户投影，形成双层停用保护。

## 角色与权限基线

- 产品总监：身份/角色治理、研发交付对象读写和审计读取。
- 产品经理：项目、需求、任务、变更的读写；工时只读。
- 开发者：任务读取/更新、工时创建/读取；不得创建项目、需求、角色或其他 Principal。

租户角色以现有 `authorization_role`、`permission_set`、`principal_role_assignment` 保存；高风险 grant/attach/assign 继续要求可验证 approval。

## 数据迁移与兼容

- 新增 migration `0017`，所有列 nullable/default compatible，历史投影保留。
- `principal_type` 继续使用物理值 `user|service|group`，API 映射为 `HUMAN|SERVICE|GROUP`。
- owner 复合外键限定同租户；SERVICE client ID 在同租户唯一。
- 回滚应用不会删除新列；migration 不回滚历史授权事实。

## 验收标准

- HUMAN 与 SERVICE OACT 可分别自同步；SERVICE 缺 owner/client、owner 非 HUMAN/非 active、跨租户时失败。
- 请求体不能冒充其他 principal；暂停主体不能自行恢复。
- 管理中心显示三个真实研发账号、负责人和角色，不出现演示账号。
- 研发角色权限符合最小权限矩阵，开发者无法执行治理能力。
- Go tests、race、vet、迁移/RLS 测试、状态校验、生产同步和控制台回读通过。

## 风险与回滚

- OACT 已签发后最多在 TTL 内继续有效；DevAutopilot 的每请求 sync/status gate 将缩小这一窗口。
- 若投影能力异常，可回滚应用 release；已有投影保留但不扩大权限。

## 实现进展

- 已完成现状审计和规格批准，等待 TASK-049 实现与生产开户。
