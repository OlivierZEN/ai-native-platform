---
kind: feature-spec
feature_id: FEAT-044
title: Semattice web Keycloak login
status: verified
owner_role: fullstack-agent
task_ids: TASK-045
related_decisions: ADR-014
related_issues: ISSUE-001
updated_at: 2026-07-31T07:34:17Z
updated_by: root after local web OIDC verification
---

# FEAT-044 - Semattice 网站 Keycloak 登录

## 背景与目标

Semattice 管理中心当前只能把已有 OACT 换成短期浏览器 Cookie，匿名用户无法从网站直接发起统一登录。用户要求保留 `semattice-cli`，并为网站增加 Keycloak Authorization Code 登录，成功后直接进入 `/console/`。

## 范围

### In Scope

- `GET /auth/oidc/login`：生成 `state`、`nonce` 和 S256 PKCE，写入短期签名状态 Cookie并跳转 Keycloak。
- `GET /auth/oidc/callback`：一次性校验状态 Cookie，以服务端 Client Secret + PKCE交换 authorization code，验证 Keycloak access/ID token、Organization和 active tenant映射。
- 登录成功后创建短期 `HttpOnly + Secure + SameSite=Lax` Semattice管理中心 Session Cookie并跳转 `/console/`。
- 管理中心匿名门页提供“使用 Keycloak登录”按钮；现有 OACT `POST /console/session` 保持兼容。
- Client Secret只从受保护文件读取，禁止进入前端、URL、日志、Git或状态文档。

### Out Of Scope

- 用户注册、密码、MFA或Keycloak主题管理。
- 浏览器保存 access token、ID token、refresh token或Client Secret。
- 自动授予写权限、Principal/RBAC重构或多组织切换页面。
- 本任务中的生产部署、Secret写入或Keycloak管理变更。

## 方案设计

1. `semattice-web` 是独立 confidential OIDC client；`semattice-cli` 保持 public native client。
2. Semattice后端使用精确 callback URI和 `client_secret_basic` 调用 Keycloak Token Endpoint。
3. 状态 Cookie只存储签名后的随机state；nonce、PKCE verifier和5分钟过期时间保存在服务端一次性、容量受限的状态表中，并使用与Session同一根密钥但不同签名域保护Cookie完整性。
4. 回调同时要求：state匹配、PKCE交换成功、ID token nonce/subject有效、access token `iss`/`aud=semattice-api`/`azp=semattice-web`有效、且只有一个 Organization。
5. Organization alias必须映射到 global/native均 active的 `tenant_registry.company_id`。浏览器只获得只读控制台Session Cookie，不获得Keycloak或OACT Token。

## 接口与配置

- `AI_NATIVE_CONSOLE_OIDC_CLIENT_ID=semattice-web`
- `AI_NATIVE_CONSOLE_OIDC_CLIENT_SECRET_FILE=/etc/semattice/secrets/semattice-web-client-secret`
- `AI_NATIVE_CONSOLE_OIDC_REDIRECT_URI=https://semattice.agentcici.com/auth/oidc/callback`
- Keycloak issuer、audience和JWKS复用现有 `AI_NATIVE_KEYCLOAK_*` 固定配置。

## 验收标准

- 登录跳转包含精确client/redirect、`openid organization`、随机state/nonce及S256 PKCE，不含Secret。
- 缺失/伪造/过期/重放state、Keycloak error、无code、Token Endpoint失败、nonce/subject不一致、多Organization、未开户或停用租户均fail closed且不创建Session。
- 成功回调只留下短期安全Session Cookie并303跳转 `/console/`。
- 前端匿名页显示登录按钮；Token和Secret不进入HTML、JavaScript、URL、日志或Git。
- 定向测试、全量Go race、vet、module/build、JS/bash、diff/secret和项目状态检查完成。

## 实施结果

- Keycloak `semattice-web` client、精确回调、Organization scope和`semattice-api` audience已由用户完成配置。
- 新增服务端 `/auth/oidc/login` 和 `/auth/oidc/callback`，完成state、nonce、S256 PKCE、confidential client token exchange、access/ID token双验签、subject一致性、唯一Organization与active tenant校验。
- Client Secret只从绝对路径的非符号链接普通文件读取，拒绝world-readable、空值、超限和含空白Secret；错误信息不包含Secret或文件路径。
- 登录成功只写入最长15分钟的签名 `Secure; HttpOnly; SameSite=Lax` Cookie，内容不包含Keycloak access/ID/refresh token；现有CLI/OACT `POST /console/session`保持兼容。
- Nginx路由、匿名登录按钮、失败提示和部署说明已更新。全量Go race、vet、module、Linux构建、JS/HTML、Shell和diff检查通过。
- 本任务没有读取生产Client Secret、修改Keycloak、部署服务器、提交或推送；线上启用需另行安全写入Secret文件和三个 `AI_NATIVE_CONSOLE_OIDC_*` 配置后发布。
