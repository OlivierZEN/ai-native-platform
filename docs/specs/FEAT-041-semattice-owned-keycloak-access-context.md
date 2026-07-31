---
kind: feature-spec
feature_id: FEAT-041
title: Semattice-owned Keycloak organization access context
status: implemented
owner_role: integration-agent
task_ids: TASK-041
related_decisions: ADR-014
related_issues: none
updated_at: 2026-07-31T04:05:56Z
updated_by: root after local implementation and validation
---

# FEAT-041 - Semattice 自主管理 Keycloak 组织访问上下文

## 背景与目标

现有 `cloudcc-semattice` 人类登录在 Keycloak Authorization Code + PKCE 完成后，仍调用外部 AgentCiCi 的公司目录和 OACT 换票接口。线上未发布这些接口时，登录只能停在换票阶段。

本功能完全移除 Semattice 运行时和 Skill/CLI 对 AgentCiCi 应用接口、开户回调及 OACT 签发服务的依赖：

1. Keycloak 继续作为唯一 IdP，并在签名 access token 中提供用户 `sub` 和 Organization membership。
2. Semattice 根据 Keycloak Organization alias 查找自己的 `tenant_registry.company_id`。
3. Semattice 自己签发仅面向 `semattice-api` 的短期 OACT。
4. Capability API 继续只接受 OACT，不接受 raw Keycloak token。

现有 Keycloak Realm 名、登录域名和 Semattice 公网域名属于已部署基础设施标识，不在本功能中重命名或破坏。

## 范围

### In Scope

- 新增 `POST /v1/auth/token`，验证 Keycloak RS256 access token 的固定 issuer、audience、JWKS 和 `azp`。
- 只接受恰好一个已签名 Organization alias，并映射到已存在、双重 active 的 Semattice tenant。
- 服务端按 allowlist裁剪请求 scope，使用 Semattice 自有 HS256 identity key签发短期 OACT。
- Skill/CLI 只调用 Keycloak discovery/token endpoint和 Semattice `/v1/auth/token`。
- 移除 Skill/CLI 中的 AgentCiCi base URL、可用公司查询和外部 mint 调用。
- 移除 Semattice `serve` 对 AgentCiCi 开户配置、反向 reservation/complete 和内部开户路由的依赖。
- 旧版包含 AgentCiCi 字段的本地登录缓存 fail closed，要求重新登录。
- 更新测试、配置说明、技能版本和本机安装副本。

### Out of Scope

- 重命名现有 Keycloak Realm、`sso.agentcici.com`、`semattice.agentcici.com` 或 TLS 文件。
- 删除 Keycloak 中属于其他应用的 client、用户、Organization 或会话。
- 自动创建 Semattice tenant；Keycloak Organization 只有在 alias 已与 `tenant_registry.company_id` 对应时才能换票。
- 自动授予业务 RBAC。OACT scope是入口上限，Semattice 内部 Principal/角色/Permission Set仍独立执行。
- 生产部署、远程配置修改、Git push、tag 或 release。

## 安全契约

- Keycloak access token必须为 RS256，且 `iss`、`aud`、JWKS URL和 `azp=semattice-cli` 均来自服务端固定配置。
- `organization` claim必须来自已验证 Token；请求正文不得提交 `tenant_id`、`company_id`、`sub` 或最终 scope。
- `requested_scopes` 必须是非空、去重、受限字符串列表，并全部包含在服务端 allowlist中。
- OACT 固定 `iss`、`aud=semattice-api`、短 TTL、随机 `jti`，并写入 `tenant_id`、`company_id`、`principal_id=sub`、`principal_type=HUMAN`。
- Bearer 请求不跟随 redirect；错误响应不得包含 Token、密钥、数据库信息或未经限制的上游错误。

## 请求流程

```text
semattice login [--company-id <keycloak-org-alias>]
  -> Keycloak Authorization Code + S256 PKCE
  -> Keycloak access token + refresh token
  -> POST Semattice /v1/auth/token
       Authorization: Bearer <keycloak-access-token>
       {"requested_scopes":[...]}
  -> Semattice 验签并读取 organization alias
  -> tenant_registry.company_id 映射 active tenant
  -> Semattice 签发短期 OACT
  -> OS credential store(refresh token) + 0600 cache(short OACT)
```

续期时 CLI 只向 Keycloak刷新 access token，再调用同一个 Semattice换票端点。用户需要切换组织时重新执行登录，并通过 Keycloak Organization scope完成选择。

## 配置

- `AI_NATIVE_KEYCLOAK_ISSUER`
- `AI_NATIVE_KEYCLOAK_AUDIENCE`
- `AI_NATIVE_KEYCLOAK_JWKS_URL`
- `AI_NATIVE_KEYCLOAK_CLIENT_ID`
- `AI_NATIVE_OACT_ALLOWED_SCOPES`
- `AI_NATIVE_OACT_TTL`（默认 `10m`）
- 既有 `AI_NATIVE_IDENTITY_ISSUER`、`AI_NATIVE_IDENTITY_AUDIENCE`、`AI_NATIVE_IDENTITY_ALGORITHM=HS256`、`AI_NATIVE_IDENTITY_HMAC_KEY` 同时作为 Semattice OACT 的签发和验证配置。

Keycloak 的 `semattice-cli` access token必须通过 client audience mapper包含 `semattice-api`，并通过 Organization Membership mapper包含 Organization alias/id。

## 验收标准

- 仓库活动 Go/Python代码和 Skill说明中不再存在 AgentCiCi API、换票或受控开户依赖。
- 有效 Keycloak Token + 单个已映射 active Organization 可获得可被现有 verifier接受的短期 OACT。
- 错误 issuer/audience/azp/签名、缺失或多个 Organization、未映射/停用 tenant、越权 scope均 fail closed。
- CLI 登录、续期、401 单次重试、凭据存储和旧缓存拒绝均有测试。
- `go test ./...`、Python测试、技能校验、YAML、CLI help/dry-run、diff/secret检查通过。
