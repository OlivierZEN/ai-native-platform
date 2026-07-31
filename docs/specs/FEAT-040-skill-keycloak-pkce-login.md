---
kind: feature-spec
feature_id: FEAT-040
title: Semattice skill Keycloak PKCE login and OACT credential helper
status: implemented
owner_role: integration-agent
task_ids: TASK-040
related_decisions: ADR-014
related_issues: none
updated_at: 2026-07-30T13:20:27Z
updated_by: root after local implementation and independent forward test
---

# FEAT-040 - Semattice skill Keycloak PKCE 登录与 OACT 凭据助手

> 本规格记录 `1.2.1` 的历史实现。其 AgentCiCi公司目录和换票设计已由
> [FEAT-041](FEAT-041-semattice-owned-keycloak-access-context.md) 替代；当前 Skill只向
> Semattice `/v1/auth/token` 换取 OACT。

## 背景与目标

`cloudcc-semattice` 当前只能从 `SEMATTICE_TOKEN` 读取外部注入的短期 OACT，缺少人工 CLI/MCP 场景的登录、续期和当前公司换票能力。用户要求按 Keycloak 统一身份流程补齐：CLI 打开浏览器完成 Authorization Code + PKCE，使用 Keycloak 会话向 AgentCiCi 获取当前公司对应的短期 OACT，再用 OACT 调用 Semattice；CLI 不接收用户名或密码。

本次交付让技能提供可复用的本地凭据助手，同时保留现有无交互 `SEMATTICE_TOKEN` 调用方式。

## 范围

### In Scope

- 为 Keycloak Realm 注册 `semattice-cli` public client，启用 Authorization Code + PKCE，只注册 Keycloak native-app 专用 `http://127.0.0.1` loopback URI（动态端口）。
- 为技能脚本增加 `login`、`status`、`logout` 和 `call` 命令；旧 `semattice_api.py --capability ...` 形式保持兼容。
- 通过 OIDC discovery 获取授权和 Token endpoint，校验随机 `state`，使用 S256 PKCE，不接触用户密码。
- 用 Keycloak access token读取 AgentCiCi 可用公司并请求面向 `semattice-api` 的短期 OACT。
- 只将 refresh token 保存到操作系统凭据库；本地权限受限文件只保存非敏感会话元数据和短期 OACT。
- OACT 到期前使用 Keycloak refresh token续期并重新换发；Semattice 返回 401 时刷新后最多重试一次相同请求。
- 更新技能认证契约、使用说明、版本和自动化测试。

### Out Of Scope

- 修改或部署 AgentCiCi 服务端 ACS；技能按 FEAT-028 的公开逻辑契约调用，端点可显式覆盖。
- 生产 Keycloak/AgentCiCi/Semattice 发布、远程 client 创建或真实用户登录验收。
- 把 Keycloak access token或 service-account token直接交给 Semattice。
- 在普通明文配置、权限不受限的文件、仓库、日志、命令行参数或浏览器 URL 中保存 refresh token/OACT；本规格明确允许的 `0600` 短期 OACT 缓存除外。
- 本次实现官方 BFF、无人服务 `client_credentials` 或第三方登录流程。

## 用户场景

1. 用户执行 `semattice login`，浏览器打开 Keycloak；登录成功后 Keycloak 回调本机 loopback。
2. CLI 取得 Keycloak access/refresh token，读取可用公司；单公司自动选择，多公司由用户选择或传入 `--company-id`。
3. CLI 向 AgentCiCi 请求 `audience=semattice-api` 的短期 OACT，并缓存短期 OACT。
4. 用户执行 `semattice call ...`；OACT 有效时直接调用，临近到期时先刷新 Keycloak 会话并重新换票。
5. `status` 只显示 issuer、client、company 与到期状态；`logout` 删除系统凭据库中的 refresh token和本地缓存。

非交互自动化仍可只注入 `SEMATTICE_TOKEN`；该值优先于本地登录缓存，且不会被脚本保存。

## 现状与约束

- Keycloak issuer 为 `https://sso.agentcici.com/realms/agentcici`，Semattice audience 为 `semattice-api`。
- FEAT-028 定义 `GET /v1/access-contexts/available-tenants` 与 `POST /v1/access-contexts/mint`，但本仓库不拥有 AgentCiCi 服务端实现；默认 AgentCiCi 根地址为 `https://x.agentcici.com`，允许通过环境变量或登录参数覆盖。
- OACT 由 AgentCiCi/ACS 签发；Semattice 只接受 OACT，不接受 raw Keycloak 用户或服务 token。
- 脚本只使用 Python 标准库。macOS 使用 Security.framework；Linux 使用支持 stdin 的 Secret Service `secret-tool`。没有安全凭据库时 fail closed，不回退到明文 refresh-token 文件。
- `SEMATTICE_TOKEN` 的既有 dry-run 和调用行为必须向后兼容。

## 方案设计

### 命令与流程

```text
semattice login
  -> OIDC discovery
  -> loopback callback + Authorization Code/S256 PKCE
  -> Keycloak access/refresh token
  -> GET AgentCiCi available-tenants
  -> POST AgentCiCi mint(current company, semattice-api, requested scopes)
  -> OS credential store(refresh token) + mode 0600 cache(short OACT/metadata)

semattice call
  -> SEMATTICE_TOKEN（若存在）
  -> 否则读取短期 OACT
  -> 临近到期时用 OS credential store 中的 refresh token刷新 Keycloak
  -> 再向 AgentCiCi 换短期 OACT
  -> Authorization: Bearer <OACT> 调用 Capability API
```

### AgentCiCi 客户端契约

- `GET /v1/access-contexts/available-tenants` 使用 `Authorization: Bearer <keycloak-access-token>`，返回 `{"tenants":[{"company_id":"...","display_name":"..."}]}`。
- `POST /v1/access-contexts/mint` 使用相同 Bearer，正文为 `{"company_id":"...","audiences":["semattice-api"],"requested_scopes":[...]}`，返回 OAuth 风格 `access_token`、`token_type=Bearer` 和 `expires_in`。
- 调用方不能提交 `sub`、`tenant_id`、`principal_id` 或最终 scopes；AgentCiCi 必须从已验证主体、成员、订阅和授权计算最终 OACT。

### 本地安全边界

- 回调 listener 只绑定 `127.0.0.1` 根路径，使用系统选择的临时端口；随机 `state` 不匹配立即拒绝。
- PKCE verifier 使用加密安全随机源，challenge 为 SHA-256 base64url。
- refresh token仅进入系统凭据库；缓存文件拒绝符号链接、要求当前用户所有且权限不宽于 `0600`，父目录权限不宽于 `0700`。
- 输出和异常不得包含 Authorization header、authorization code、access token、refresh token或 OACT。
- 携带 Bearer 的 Keycloak、AgentCiCi 和 Semattice 请求禁止自动跟随 HTTP redirect，避免 Authorization header跨来源转发；认证错误只输出 HTTP 状态和受限稳定错误码。

## 交付计划

- `TASK-040`：实现技能凭据助手、Keycloak client、文档、版本和测试。
- 主要范围：`skill/cloudcc-semattice/`、`deploy/keycloak/configure-agentcici-realm.sh`、`docs/specs/FEAT-029-*`、项目状态和 Python 测试。
- 不修改 Go Resource Server、数据库 schema 或外部 AgentCiCi 仓库。

## 接口与数据影响

- 新增本地 CLI 命令，不改变 Capability API 的 URL、body 或响应格式。
- 新增配置：`SEMATTICE_KEYCLOAK_ISSUER`、`SEMATTICE_OIDC_CLIENT_ID`、`SEMATTICE_AGENTCICI_BASE_URL`、`SEMATTICE_CREDENTIALS_FILE`；均可由相同名称的登录参数覆盖。
- `SEMATTICE_BASE_URL` 与 `SEMATTICE_TOKEN` 保持兼容。
- Keycloak 新 public client不包含 secret，禁用 implicit、direct access grant和 service account。

## 验收标准

- Authorization URL 含随机 state、S256 code challenge和 loopback redirect；错误 state/code/error均 fail closed。
- Token endpoint只接收 authorization code或 refresh token，不接收用户名/密码。
- refresh token不写入缓存；缓存权限、符号链接和宽权限文件校验有测试。
- 多公司选择、OACT mint、到期刷新、401 单次刷新重试、logout清理和 `SEMATTICE_TOKEN` 优先级有测试。
- 登录默认请求最小 `system.capability.read`，Bearer 请求拒绝 redirect，恶意错误描述不能进入终端输出。
- 旧 `python3 scripts/semattice_api.py --capability ... --dry-run` 与新 `scripts/semattice call ... --dry-run` 均通过。
- `quick_validate.py`、Python 编译/测试、YAML、shell syntax、版本一致性、`git diff --check` 通过。

## 风险与回滚

- AgentCiCi 尚未发布或契约漂移时，登录在换票步骤明确失败，不回退为 raw Keycloak token直连。
- 操作系统凭据库不可用时要求使用既有短期 `SEMATTICE_TOKEN`，不落盘 refresh token。
- 可回滚技能脚本和 `semattice-cli` 注册；删除 public client不会影响现有 BFF、OACT Resource Server 或显式 token调用。

## 实现进展

- 当前状态：`implemented_locally_not_deployed`。
- 已完成：Keycloak public PKCE client声明、loopback/state/S256 登录、系统凭据库 refresh token、`0600` 短期 OACT 缓存、当前公司换票、自动续期/401 重试、新旧 CLI、文档和 13 项 Python 回归测试。
- 独立前向检查发现并推动修复：Bearer redirect 转发、认证错误描述泄密和缺少默认发现 scope。
- 未完成：AgentCiCi 人类 access-context 端点发布、生产 `semattice-cli` 注册、真实账号/多公司/OACT 端到端验收、技能发布仓库同步与 `v1.2.1` 发布。

## 交接说明

- 先阅读本规格、FEAT-028、FEAT-029 和 `skill/cloudcc-semattice/references/api-contract.md`。
- 不得用 Keycloak client secret、密码登录或明文 refresh-token 文件作为临时替代。
- 真实生产登录仍依赖 AgentCiCi 发布上述人类 ACS 端点并注册 `semattice-cli` callback，本任务不声称已完成远程部署。
