---
kind: feature-spec
feature_id: FEAT-053
title: Development manual confirmation for metadata Changesets
status: verified
owner_role: backend-agent
task_ids: none
related_decisions: ADR-003
related_issues: none
updated_at: 2026-08-05T10:11:00+08:00
updated_by: codex
---

# FEAT-053 - Changeset 开发期手工确认

## 背景与目标

当前人工 CLI 签发的短期 OACT 不包含 `approvals` 声明，项目也尚未实现审批申请服务，因此已有活动元数据版本后的 Changeset 无法通过公开能力继续发布。项目仍处于开发阶段，本次允许 `metadata.changeset.approve` 使用显式、非空的手工确认标识，行为与首版本 `metadata.version.publish` 的手工确认一致。

## 范围

### In Scope

- `metadata.changeset.approve` 接受调用方明确提供的非空手工 `approval_id`。
- 服务端去除首尾空白后保存审批标识，并在 Changeset 审批事务中记录 `approval_mode=manual`。
- 保留 Changeset 状态机、Scope、租户隔离、幂等和审计要求。
- 更新公开 Capability 描述和 Semattice Skill 操作契约。

### Out Of Scope

- 不放宽 `metadata.changeset.purge`、`metadata.changeset.rollback`、授权、共享和组织合并等其他高风险能力。
- 不实现审批中心、第二人审批、审批待办或 OACT `approvals` 签发。
- 不新增数据库迁移，不处理历史数据或兼容逻辑。

## 接口行为

调用方在核对 Changeset 模拟结果并明确授权后调用：

```json
{
  "changeset_id": "<uuid>",
  "approval_id": "<non-empty-manual-confirmation-id>"
}
```

- 缺失 `approval_id` 仍由输入 Schema 拒绝。
- 纯空白 `approval_id` 返回 `FAILED_PRECONDITION`。
- 非空手工标识不要求存在于 OACT 的 `approvals` 声明中。
- 成功后 Changeset 进入 `approved`，并写入 `approval_id`、`approved_by`、`approved_at` 和审计事件。
- 审计事件数据包含 `approval_id` 与 `approval_mode=manual`。

## 验收标准

- 无 `approvals` claim 的可信主体可用非空手工确认 ID 审批 `validated` Changeset。
- 空白手工确认 ID 被拒绝。
- 审计中能回读手工确认 ID 和 `manual` 模式。
- 回滚使用未验证审批 ID 仍失败，证明其他审批门禁未被放宽。
- Linux/amd64 生产构建、原子发布、线上能力发现、真实 Changeset 审批与发布回读通过。

## 风险与回滚

手工确认弱于独立第二人审批，只适用于当前开发阶段。发布仍必须由明确授权的用户发起，并在调用前展示完整候选模型或模拟结果。服务回滚时把 `/opt/semattice/current` 原子切回上一不可变 release 并重启；本次没有数据库迁移。

## 实现进展

- 提交 `5f5bdcea97d2` 已发布为 `/opt/semattice/releases/20260805T020849Z-web-oidc-5f5bdcea97d2`；Semattice、Nginx 和 PostgreSQL 均为 active，Semattice `NRestarts=0`、发布后 warning 日志为 0。
- 线上能力发现已返回新的显式手工 Changeset 审批描述，审计标识为 `audit:req-3228301b-cdfe-488a-860c-1235afe75bb4`。
- Changeset `019fcd45-5e1f-77fb-86b2-792f7f4b2d6b` 使用手工确认 `manual-commerce-changeset-20260805-019fcd45` 审批成功，审批审计标识为 `audit:req-69a6464e-a1a3-4d04-ad68-95a352da869a`。
- 同一 Changeset 随后发布为 `active`，发布审计标识为 `audit:req-04f7dcf4-0c58-44bc-8920-1661bec27855`。当前版本回读为 `019fcd43-8fe1-7e0d-bd71-f0757aa91af1`、sequence 3、snapshot digest `8fc42d39c0eab1d2fe44cb4af9f324964cc60fa1a4607d85b56600ea3f0ae030`，包含 28 个对象、391 个字段和 70 个关系。
- 线上空白手工确认负例返回 `FAILED_PRECONDITION / a manual approval id is required`，审计标识为 `audit:req-2ebc4f1c-346d-417c-adcc-32830eab6de4`；负例后回读 Changeset 仍为 `active`。
