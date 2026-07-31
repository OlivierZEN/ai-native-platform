---
kind: feature-spec
feature_id: FEAT-046
title: Manual confirmation for direct metadata publication
status: approved
owner_role: backend-agent
task_ids: TASK-049
related_decisions: ADR-003
related_issues: none
updated_at: 2026-07-31T09:24:41Z
updated_by: root after local implementation and verification
---

# FEAT-046 - 元数据直接发布支持手动确认

## 背景与目标

当前 `metadata.version.publish` 要求 `approval_id` 同时存在于可信 OACT 的 `approvals` 声明中。Semattice 人工 CLI 登录只签发短期 scope，不提供审批声明，导致租户首个元数据版本无法从受控 CLI 发布。

用户于 2026-07-31 明确要求仅把直接发布调整为内测期手动确认：调用人仍需提供非空 `approval_id`，服务端记录该值和操作审计，但不再要求它出现在 OACT 的 `approvals` 声明中。

## 范围

### In Scope

- `metadata.version.publish` 接受调用人手动填写的非空 `approval_id`。
- 在发布元数据的同一事务中，把 `approval_id`、确认模式和元数据版本 ID 写入租户隔离的 `audit_event`。
- 保留发布 scope、RBAC、租户隔离、幂等、不可变快照、仅首版本可直接发布和高风险异步契约。
- 更新 Capability 描述、Skill 契约、能力目录和对象发布流程。
- 增加回归测试并部署到现有 Semattice 生产环境。

### Out Of Scope

- 不放宽 `metadata.changeset.*`、授权、共享、组织合并或其他高风险能力的可信审批校验。
- 不增加管理控制台写入口、审批中心或第二人审批工作流。
- 不迁移数据库，也不修改 OACT claim 格式。
- 不在未核对完整草稿内容时执行生产发布。

## 方案设计

1. `metadata.version.publish` 保持必填 `approval_id` Schema，并在服务层拒绝空白值。
2. 只移除该能力对 `Principal.Approvals` 的成员校验；其他能力继续按原实现 fail closed。
3. 发布事务在切换租户活动版本指针后写入 `audit_event`，事件数据包含：
   - `approval_id`
   - `approval_mode: manual`
   - `metadata_version_id`
4. 对调用方而言请求和响应结构不变，属于向后兼容的行为放宽。

## 交付计划

- `TASK-049`
- owner：`backend-agent`
- scope：元数据发布服务、Capability 契约、测试、Skill 文档、生产发布和验证证据。

## 接口与数据影响

- `metadata.version.publish` 继续要求：

```json
{
  "metadata_version_id": "<uuid>",
  "approval_id": "<non-empty-manual-confirmation-id>"
}
```

- 不新增字段、不新增迁移。
- 回滚时切回上一不可变应用 release；未执行实际元数据发布时没有业务数据回滚。

## 验收标准

- 缺失或空白 `approval_id` 的直接发布失败。
- 不在 OACT `approvals` 声明中的非空手动 `approval_id` 可完成首版本直接发布。
- 审计事件持久记录手动 `approval_id` 和 `approval_mode=manual`。
- 已有活动版本仍拒绝绕过 Changeset 直接发布。
- Changeset、授权、共享和组织合并的可信审批门禁不变。
- API、MCP 和 CLI 继续从同一 Capability 定义投影。
- 定向测试、全量 Go 回归、Skill 校验和生产健康检查通过。

## 风险与回滚

- 风险：调用人可自行填写确认标识，弱于独立审批，仅适用于当前内测直接发布场景。
- 缓解：能力仍为高风险、要求明确用户授权、完整草稿回读、非空确认标识和持久审计；范围严格限制在首版本直接发布。
- 回滚：原子切换回上一生产 release；后续恢复可信 OACT 审批校验时无需数据迁移。

## 实现进展

- 已获得用户对行为边界和生产部署的明确授权。
- 服务层已只放宽直接发布的审批声明成员校验，并在同一事务持久审计手动确认标识。
- 定向 PostgreSQL、全仓 PostgreSQL、全量 race、vet、module、Skill 测试、官方 Skill 校验和无 Token dry-run 均通过。
- 待完成生产发布、线上契约与健康验证，以及安装 Skill 副本同步。

## 交接说明

- 发布代码前确认全量测试和 Skill 校验通过。
- 生产元数据草稿可能被并行修改；实际发布前必须重新读取并向用户确认整个版本内容。
