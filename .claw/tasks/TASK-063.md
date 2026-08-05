---
kind: task
task_id: TASK-063
title: Prevent Keycloak theme deployment from resetting username login policy
status: done
priority: critical
owner_role: integration-agent
claimed_by: root
spec_path: docs/specs/FEAT-029-keycloak-resource-server-and-production-idp.md
depends_on: none
blocked_by: none
updated_at: 2026-08-05T10:32:48Z
updated_by: codex
---

# TASK-063 - 固化 Keycloak 用户名与邮箱登录策略

## 范围

- 修复 AgentCiCi Keycloak 登录主题部署与 realm 配置脚本，禁止主题发布将用户名策略回退为“邮箱即用户名”。
- 明确手机号/公共编号为 `username`，同一 Keycloak 用户的真实邮箱为第二登录标识。
- 验证脚本语法、受管 Realm 更新参数与线上发布后的 Keycloak API 回读。

## 完成标准

- 两个受管脚本均显式设置并验证 `registrationEmailAsUsername=false`、`loginWithEmailAllowed=true`、`duplicateEmailsAllowed=false`、`editUsernameAllowed=false`。
- 登录主题发布不会改写用户、凭据、角色、MFA 或绑定，仅保留 realm 认证策略。
- 线上主题部署后上述 realm 策略与手机号 username 回读正确。

## 完成证据

- 提交 `cbc769f` 显式固定 realm 的用户名/邮箱登录策略；`5574b15` 使部署脚本兼容当前版本化主题样式表。
- 线上以当前主题文件执行新版本脚本，Keycloak 就绪检查通过；realm 回读为 `registrationEmailAsUsername=false`、`loginWithEmailAllowed=true`、`duplicateEmailsAllowed=false`、`editUsernameAllowed=false`。
- 用户 `18611892001` 已通过 Keycloak Admin API 恢复为同值 username；API、精确搜索与持久化数据回读一致。未修改密码、邮箱、MFA、角色或 AgentCiCi 外部身份绑定。
