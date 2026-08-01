---
kind: feature-spec
feature_id: FEAT-043
title: Enable every published Semattice capability scope for skill login
status: implemented
owner_role: integration-agent
task_ids: TASK-044
related_decisions: ADR-014
related_issues: none
updated_at: 2026-07-31T05:26:40Z
updated_by: root after local and production acceptance
---

# FEAT-043 - Skill 登录开放全部已发布能力 Scope

## 目标

`cloudcc-semattice` 人类登录默认请求当前 51 项公开 Capability 所需的全部唯一
scope，生产 `AI_NATIVE_OACT_ALLOWED_SCOPES` 接受同一集合。scope 只作为 Capability
入口上限；Principal/RBAC、RLS、独立审批、幂等和审计继续生效。

## 范围

- 从当前运行时注册表和 Skill API 目录核对 51 项公开 Capability 的唯一
  `required_scope`；不包含未公开的 `tenant.provision`。
- Skill `semattice login` 默认请求完整 scope 集合，并在续期时保持同一集合。
- 更新认证说明、部署示例和 Skill 版本。
- 备份生产环境文件，只替换 `AI_NATIVE_OACT_ALLOWED_SCOPES`，重启并回读验证。
- 使用真实登录获得的新 OACT 验证 scope 集合和 `system.capability.list`；不调用任何
  业务写能力。

## 明确不做

- 不把 scope 当成业务授权，不自动创建 Principal、角色或 Permission Set。
- 不生成或伪造 `approval_id`，不绕过高风险能力的独立审批。
- 不创建、更新、删除业务数据、元数据、租户、组织或共享资源。
- 不修改其他 Keycloak client、用户、Organization 或会话。
- 不提交、推送、打标签或发布独立 Skill 仓库。

## 验收标准

- 当前 51 项公开 Capability 映射为 26 个唯一 scope，客户端默认值和生产 allowlist
  完全一致。
- `semattice login` 的换票请求包含全部 26 个 scope，重复值被消除。
- 服务端继续拒绝 allowlist 外 scope，现有认证、租户映射和令牌验证负例继续通过。
- Python 认证测试、Go access-context/config测试、Skill校验、YAML、脚本语法、
  version/README一致性和差异检查通过。
- 生产环境文件有可恢复备份，服务重启后 active，公网健康检查和匿名负向检查通过。
- 新 OACT 可调用 `system.capability.list` 并发现 51 项能力；验证过程无业务写入。

## 回滚

- Skill 默认 scope可回滚到上一版本。
- 生产环境恢复本次变更前的 `semattice.env` 备份并重启服务。
- scope回滚不修改数据库、Keycloak数据或已存在业务资源。

## 实施结果

- Skill版本升至 `1.3.0`；默认scope常量包含26个唯一值，测试从API目录逐项核对
  51项公开Capability和scope全集。登录缓存升至v3，旧v2单scope缓存要求重新登录。
- 生产环境文件备份为
  `/etc/semattice/semattice.env.backup.20260731T052514Z-all-capability-scopes`；只替换
  `AI_NATIVE_OACT_ALLOWED_SCOPES`并重启现有release。
- 生产服务active，公网健康为200，匿名换票为401，重启后日志无panic/fatal。
- 真实PKCE登录返回26个scope；线上`system.capability.list`返回51项能力和26个唯一
  scope，与Skill默认集合零差异。`tenant.get-status`只读调用成功且含审计标识。
- 验收未调用任何业务写能力，未修改Keycloak、业务数据、元数据或授权资源。
