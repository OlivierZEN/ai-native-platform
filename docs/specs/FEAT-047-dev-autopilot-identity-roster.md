---
kind: feature-spec
feature_id: FEAT-047
title: DEV Autopilot governed identity roster
status: verified
owner_role: integration-agent
task_ids: TASK-053
related_decisions: ADR-016
related_issues: none
updated_at: 2026-08-03T12:44:26Z
updated_by: root
---

# FEAT-047 - DEV Autopilot 研发身份花名册

## 背景与目标

目标租户需要把研发交付身份调整为一名人类产品总监、两名既有机器主体和一名新增开发者机器主体，并让管理中心、Semattice 授权和 DEV Autopilot CLI 使用同一套可信 Principal 数据。

## 身份定义

| 显示名称 | Principal 类型 | 账号 / client_id | 角色 | 人类负责人 |
| --- | --- | --- | --- | --- |
| Oliver | HUMAN | U20267MV3E4N7 | 产品总监 | 本人 |
| 大乔 | SERVICE | dev-autopilot-product-manager | 产品经理 | Oliver |
| 悟空 | SERVICE | dev-autopilot-developer | 开发者 | Oliver |
| 后羿 | SERVICE | dev-autopilot-developer-houyi | 开发者 | Oliver |

## 范围

- AgentCiCi 作为权威身份源维护显示名称、机器主体、owner 和生命周期。
- Semattice 投影四名 Principal，并为后羿复用现有开发者角色与研发交付部主组织成员关系。
- 所有高风险身份和授权写操作携带独立审批上下文并写入审计。
- DEV Autopilot 只接受显式允许的开发者 client_id，并继续以 Semattice PDP 做最终授权。

## 安全边界

- 产品总监 HUMAN 绑定全局用户 `18611892001`，只提供委托、确认和审批上下文。
- 大乔、悟空、后羿以自己的 SERVICE Principal 执行，不借用人类登录态。
- 机器凭据只保存在生产服务器受保护文件中，不写入 Git、文档或日志。
- 新增开发者不获得产品经理的项目创建权限。

## 验收标准

- 已认证控制台 members API 精确返回四名成员、三个角色和一个组织。
- 后羿为 active SERVICE Principal，owner 为 Oliver，拥有开发者角色和研发交付部 primary membership。
- 后羿与悟空均可通过 DEV Autopilot CLI 获取开发任务。
- 大乔凭据调用开发者 CLI 被拒绝。
- 生产健康检查保持 integrated，审计中可追踪本次身份、角色和组织投影。

