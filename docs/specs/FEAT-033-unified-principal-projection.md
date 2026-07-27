---
kind: feature-spec
id: FEAT-033
status: released_with_controlled_activation_pending
title: Semattice 统一 Principal 投影与官方机器主体认证
---

# FEAT-033：Semattice 统一 Principal 投影与官方机器主体认证

## 目标

将 Semattice 的受信调用主体从“JWT `sub` 字符串”演进为兼容的人类/机器 Principal 模型，同时保持现有 AgentCiCi OACT 与 JWKS 本地验签路径。Semattice 不保存密码、Keycloak 管理密钥或 Client Secret，也不逐请求回调 Keycloak。

## 当前事实与缺口

- `internal/identity/jwt.go` 已以缓存五分钟的 JWKS 完成本地 RS256 验签，固定 issuer、audience、tenant 和 company 边界。
- 当前 `sub` 被直接写为 `capability.Actor.ID`。这对历史 OACT 兼容，但不能表达 AgentCiCi 全局 Principal UUID、类型或机器责任链。
- AgentCiCi V98 已有 `principal` / `principal_identity` / `service_principal` / `service_principal_owner`，并要求机器主体有有效人类责任人。
- Keycloak Service Account 不应被 Semattice 直接、无投影地接受；它必须经 AgentCiCi 控制面确认 company、scope、service Principal、状态与 owner 后，获得面向 Semattice 的短期官方 token。

## 契约

### OACT 人类与机器 claim

Semattice 接受由 AgentCiCi 官方 issuer 签发、受现有 JWKS 信任的 RS256 token。现有 claim 保持兼容，并增加：

| Claim | 必需 | 含义 |
| --- | --- | --- |
| `sub` | 是 | 兼容主体标识；新 token 等于 `principal_id` |
| `principal_id` | 新 token 必需 | AgentCiCi Principal UUID |
| `principal_type` | 新 token 必需 | `HUMAN` 或 `SERVICE` |
| `company_id` / `tenant_id` | 是 | 单公司、单租户边界 |
| `scopes` | 是 | 最小资源权限集合 |
| `owner_principal_id` | SERVICE 必需 | 有效人类责任人 Principal UUID |
| `client_id` | SERVICE 必需 | Keycloak confidential client ID，仅审计关联 |

旧 OACT 没有 `principal_id` 时，Semattice 将其 `sub` 作为兼容 human Principal，标记 `legacy_oact`；不接受无 principal claim 的新 service token。

### 验证规则

1. 按 issuer/audience/RS256/exp/iat/JWKS 本地校验，拒绝 Keycloak access token 直连数据平台。
2. `principal_id` 与 `sub` 必须相同；`principal_type` 只能为 HUMAN 或 SERVICE。
3. SERVICE 必须有格式正确的 `owner_principal_id` 与非空 `client_id`；HUMAN 不可携带 service-only claim。
4. Actor 使用 `principal_id`，审计保留 principal type/source；不允许客户端请求体覆盖。
5. 数据平台本地 `principal_projection` 可为角色/组织授权保存投影，但 token 是认证事实；成员撤销、机器暂停需由 AgentCiCi 触发受控投影更新，缺少 active projection 的受保护数据操作 fail closed。

## 实施阶段

1. 扩展 `TrustedPrincipal`、JWT claim/parser 与 API/MCP/CLI 传播，覆盖兼容与负向测试。
2. 扩展 AgentCiCi OACT 签发：由人类 Keycloak session 或 Service Account client credentials 经 AgentCiCi 控制面换取短期 Semattice token；Semattice 不直接验 Keycloak client token。
3. 引入控制面 principal projection upsert/suspend 契约，消费人类成员与机器 owner 生命周期事件。
4. 在真实测试租户执行人类 OACT、机器 OACT、失效 owner、错误 audience 和撤销投影负向验收。

## 已实现与发布状态

- Stage 1 已实现并发布：TrustedPrincipal、JWT parser、API/MCP/CLI 传播均支持 HUMAN 与 SERVICE OACT；旧 human OACT 继续兼容。
- Stage 2 已完成两端代码契约：AgentCiCi 在受控 feature flag 下以 Keycloak client-credentials token 交换 10 分钟 Semattice OACT；Semattice 不直接接受 Keycloak token。
- Semattice 已于 2026-07-27 发布 release 20260727T151437Z-console，匿名边界 smoke 为通过。
- Stage 3/4 的真实 service client 验收仍等待 Keycloak SMTP、OACT 签名配置和受权凭据；所有功能开关关闭时 fail closed。

## 安全与回滚

- 不存储 Client Secret、Keycloak refresh token、密码或 bearer token。
- 新 claim 在 token 版本切换期兼容旧 OACT；未知 principal type/service claim 组合 fail closed。
- 服务发布可回滚至上版二进制；数据库变更只追加正向 migration。
- 生产启用前必须由 AgentCiCi 配置 SMTP 并完成受控人类邀请，机器 client 必须在受管密钥库中保存其单次返回的 secret。

## 验收

- 人类 OACT 以 `principal_id` 驱动 Semattice API、CLI、MCP 并产生一致审计。
- SERVICE OACT 仅由有效 service principal + active human owner 获得；直连 Keycloak Service Account token 被拒绝。
- issuer/audience/tenant/company/scope/owner/sub-principal mismatch 均被拒绝。
- JWKS 仅冷启动、TTL 或未知 `kid` 时抓取；无逐请求 IdP 调用。
- 全量 Go test、race、vet、迁移与受权生产 smoke 通过。
