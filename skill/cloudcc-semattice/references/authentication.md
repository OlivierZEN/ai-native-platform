# 登录与短期 OACT

## 目录

- [人工 CLI 登录](#人工-cli-登录)
- [调用与自动续期](#调用与自动续期)
- [配置](#配置)
- [Semattice 换票契约](#semattice-换票契约)
- [安全边界](#安全边界)
- [无人服务](#无人服务)

## 人工 CLI 登录

从技能目录执行。默认请求当前51项公开Capability所需的全部26个唯一scope；`--scope`仍可用于追加未来已发布且被服务端允许的scope。scope只是Capability入口上限，最终权限仍由Semattice的Principal/RBAC、RLS、独立审批和审计收敛：

```bash
./scripts/semattice login
```

该命令执行 Keycloak OAuth 2.0 Authorization Code + S256 PKCE：

1. 从 Realm discovery 获取 authorization/token endpoint。
2. 在 `127.0.0.1` 随机端口启动一次性 loopback listener。
3. 打开系统浏览器；用户名、密码和 MFA 只提交给 Keycloak。
4. 请求 Keycloak Organization scope，校验回调 `state`，再用 code verifier换取 access/refresh token。
5. 只把 Keycloak access token发送到 Semattice `POST /v1/auth/token`；Semattice从已验签的 Organization alias映射自己的 tenant并签发短期 `semattice-api` OACT。
6. 把 refresh token保存到操作系统凭据库，把短期 OACT 和非敏感元数据保存到用户权限缓存。

需要明确选择组织时，传入 Keycloak Organization alias；CLI 会请求 `organization:<alias>`：

```bash
./scripts/semattice login --company-id '<keycloak-organization-alias>'
```

请求正文不能提交或切换 `tenant_id`、`company_id`、主体或最终权限。Keycloak账号属于多个组织且没有选出唯一组织时，Semattice会返回 `organization_selection_required`，此时应使用 `--company-id` 重新登录。

## 调用与自动续期

```bash
./scripts/semattice call \
  --capability system.capability.list \
  --input '{}'
```

脚本优先使用 `SEMATTICE_TOKEN`；未设置时读取登录缓存。短期 OACT 临近到期时，脚本使用系统凭据库中的 Keycloak refresh token刷新会话，再向 Semattice换取同一组织的新 OACT。组织发生变化时 fail closed并要求重新登录。Semattice Capability API返回 401 时最多刷新并重试一次完全相同的请求正文；写操作仍必须使用稳定幂等键。

查看或清除登录状态不会输出令牌：

```bash
./scripts/semattice status
./scripts/semattice logout
```

旧调用形式继续兼容：

```bash
python3 scripts/semattice_api.py \
  --capability system.capability.list \
  --input '{}'
```

## 配置

默认配置：

| 环境变量 | 默认值 | 用途 |
|---|---|---|
| `SEMATTICE_KEYCLOAK_ISSUER` | `https://sso.agentcici.com/realms/agentcici` | 已部署 Keycloak Realm issuer |
| `SEMATTICE_OIDC_CLIENT_ID` | `semattice-cli` | public PKCE client |
| `SEMATTICE_BASE_URL` | `https://semattice.agentcici.com`（登录时） | Semattice API与换票服务 |
| `SEMATTICE_CREDENTIALS_FILE` | 用户配置目录 | 短期 OACT 与非敏感元数据缓存 |

登录参数可覆盖相同配置。非 loopback 地址只允许 HTTPS；URL 不得包含用户凭据、query 或 fragment。

## Semattice 换票契约

`POST /v1/auth/token` 使用 Keycloak access token Bearer，请求：

```json
{
  "requested_scopes": [
    "system.capability.read",
    "tenant.status.read",
    "tenant.lifecycle.write",
    "tenant.entitlement.write",
    "tenant.decommission",
    "metadata.version.write",
    "metadata.definition.write",
    "metadata.publish",
    "metadata.read",
    "metadata.changeset.write",
    "metadata.changeset.read",
    "metadata.changeset.approve",
    "metadata.changeset.publish",
    "metadata.changeset.execute",
    "metadata.changeset.purge",
    "metadata.changeset.rollback",
    "usage.read",
    "usage.platform.read",
    "runtime.record.create",
    "runtime.record.read",
    "runtime.record.update",
    "runtime.record.delete",
    "authorization.manage",
    "record.share.manage",
    "organization.manage",
    "authorization.read"
  ]
}
```

成功响应：

```json
{
  "access_token": "<short-lived-oact>",
  "token_type": "Bearer",
  "expires_in": 600,
  "tenant_id": "<semattice-tenant-uuid>",
  "company_id": "<keycloak-organization-alias>"
}
```

Semattice固定校验 Keycloak issuer、`semattice-api` audience、JWKS签名、`azp=semattice-cli` 和唯一 Organization membership；再以 Organization alias查找现有 active tenant。请求不得接受 `sub`、`principal_id`、任意 `tenant_id`、`company_id` 或调用方声明的最终权限。未公开的 `tenant.provision`不在默认scope集合中；allowlist外scope仍然fail closed。

## 安全边界

- 不在终端、脚本参数、日志、仓库或普通配置文件中接收/保存用户名、密码、authorization code、refresh token、OACT 或 client secret。
- macOS refresh token只存 Keychain；Linux 只存 Secret Service（`secret-tool`）。安全凭据库不可用时 fail closed，可改用外部注入的短期 `SEMATTICE_TOKEN`。
- 短期 OACT 缓存文件拒绝符号链接，要求当前用户所有且权限为 `0600`；专用目录权限为 `0700`。
- 缓存中的 JWT `exp` 只用于本地续期时机，不作为验签或授权依据；Semattice仍执行完整 issuer/audience/signature/time/scope/RBAC/RLS校验。
- raw Keycloak access token只允许发送到 Semattice `/v1/auth/token`，绝不作为 Capability API、MCP或控制台的 Bearer。
- 旧版登录缓存不包含完整的默认scope集合或仍含已移除的外部换票字段，当前版本拒绝读取并要求重新登录。

## 无人服务

无人服务不执行浏览器登录，也不复用人类 CLI缓存。它们必须使用独立、受控的机器身份换取短期 OACT；本技能当前只实现人类 PKCE登录，不接收或保存 service client secret。
