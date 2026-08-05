---
kind: feature-spec
feature_id: FEAT-055
title: Commerce Keycloak token exchange
status: verified
owner_role: integration-agent
task_ids: TASK-062
related_decisions: ADR-014
related_issues: none
updated_at: 2026-08-05T06:27:00Z
updated_by: codex after production verification
---

# FEAT-055 - 商城 Keycloak Token 换取 Semattice OACT

## 背景与目标

商城 `storefront-web` 与 `admin-web` 登录后把 Keycloak access token 发送给
`commerce-service`。服务端必须只把原始 Keycloak token 发送到 Semattice
`POST /v1/auth/token`，取得短期、单租户 OACT 后再调用 Capability API。浏览器不持有
OACT，也不直接访问 Semattice。

商城公开目录同时存在无浏览器会话的 Next.js SSR 请求。该路径使用独立
`commerce-service` Client Credentials；Semattice 依据服务器固定的 client、company 与
人类负责人绑定签发 SERVICE OACT，不接受请求正文传入租户或负责人。

## 契约与安全边界

- `/v1/auth/token` 固定验证 Keycloak RS256 issuer、`semattice-api` audience、JWKS、`azp`
  allowlist 与有效期。
- 人类 client allowlist 为 `semattice-cli`、`storefront-web`、`admin-web`。人类 token
  必须且只能包含一个 Organization alias，并映射到 active tenant。
- 服务 client 通过 `AI_NATIVE_KEYCLOAK_SERVICE_BINDINGS` 固定绑定
  `client_id=company_id@owner_principal_id`；`commerce-service` token 不允许从正文选择
  company 或 owner。
- `storefront-web`、`admin-web` 与 `commerce-service` 的 access token 均由 Keycloak
  client mapper加入 `semattice-api` audience。原始 token 只进入 `/v1/auth/token`，不能
  进入 `/v1/capabilities/*/invoke`。
- 人类换票签发 HUMAN OACT；受控服务绑定签发包含 `principal_type=SERVICE`、
  `owner_principal_id` 和 `client_id` 的 SERVICE OACT。scope allowlist、Principal/RBAC、
  RLS、审批、幂等与审计继续独立生效。

## 配置

- `AI_NATIVE_KEYCLOAK_CLIENT_IDS`：逗号分隔的人类 client allowlist。
- `AI_NATIVE_KEYCLOAK_SERVICE_BINDINGS`：逗号分隔的受控服务绑定，格式为
  `client_id=company_id@owner_principal_id`。
- 旧单值 `AI_NATIVE_KEYCLOAK_CLIENT_ID` 仅作为未配置复数变量时的回退，便于原子发布；
  生产发布后使用复数变量作为事实源。

## 验收标准

- `storefront-web` 与 `admin-web` 的真实、单 Organization token 可换取 OACT；错误
  audience、未知 `azp`、零个或多个 Organization 均失败关闭。
- `commerce-service` Client Credentials token 可按固定绑定换取 SERVICE OACT；请求
  不能覆盖 company、owner 或 scope allowlist。
- 换取的 HUMAN/SERVICE OACT 可调用允许的 runtime record Capability；原始 Keycloak
  token 直接调用 Capability 仍为 401。
- Semattice、Keycloak、Nginx 与 PostgreSQL 发布后保持 active；匿名 token endpoint 为
  401，旧 `semattice-cli` 登录继续可用。
- 定向 Go 测试、全量 Go 测试、race/vet 与生产 HTTPS smoke 有真实证据。

## 回滚

- 应用失败时恢复发布前 `/etc/semattice/semattice.env`，原子切回上一不可变 release 并
  重启 Semattice。
- Keycloak mapper 变更前导出三个 client；回滚时恢复导出，不删除 client、用户、组织或
  session。
- 本功能没有数据库 migration，不修改业务记录或元数据。

## 实现与生产证据

- 实现提交 `66ab21f50d5f` 已发布为
  `/opt/semattice/releases/20260805T061911Z-web-oidc-66ab21f50d5f`，二进制 SHA-256
  为 `6e3f47af5f57e2c1b8f18cac5903e2bfc56b26567f1e866e7d89d2a2246b4e18`。
- 发布前环境备份为
  `/etc/semattice/semattice.env.backup.20260805T061845Z-commerce-token-exchange`；
  `storefront-web`、`admin-web`、`commerce-service` Client 导出保存在
  `/opt/keycloak/backups/20260805T061845Z-commerce-token-exchange`。三个 Client 均精确
  存在一个 `semattice-api-audience` mapper。
- 真实 `commerce-service` Client Credentials token 包含 `azp=commerce-service` 与
  `semattice-api` audience；直接调用 Capability 返回 401，经 `/v1/auth/token` 换得固定
  company/owner 的 SERVICE OACT 后，`runtime.record.query` 成功回读一个商品，审计标识
  为 `audit:req-commerce-service-product-smoke`。
- Semattice、Keycloak、Nginx、PostgreSQL 均 active，Semattice `NRestarts=0`、发布后
  warning 日志为空。全量 Go、定向 race、vet、状态校验与脚本语法均通过。
