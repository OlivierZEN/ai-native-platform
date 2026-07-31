---
kind: feature-spec
feature_id: FEAT-042
title: Production rollout of Semattice-owned Keycloak access context
status: implemented
owner_role: release-agent
task_ids: TASK-043
related_decisions: ADR-014
related_issues: none
updated_at: 2026-07-31T05:02:19Z
updated_by: root after authorized production rollout and live PKCE verification
---

# FEAT-042 - Semattice 自有 Keycloak 换票生产发布

## 目标

将 FEAT-041 已验证的本地实现发布到现有 Semattice 单机生产环境，使
`semattice-cli` 完成 Keycloak Authorization Code + PKCE 后，只调用
Semattice `POST /v1/auth/token` 获取短期 OACT。

## 变更范围

- 从当前工作树构建 `linux/amd64` 静态二进制并部署到新的不可变 release 目录。
- 备份 `/etc/semattice/semattice.env`，移除已废弃的 AgentCiCi 开户变量并加入
  Keycloak access-context 配置；保留既有数据库、identity 和 console 密钥。
- 只为 Keycloak `semattice-cli` 增加或核对 `semattice-api` audience mapper，
  不修改其他 client、Organization、用户或会话。
- 原子切换 `/opt/semattice/current`，重启 Semattice，并验证公网健康、匿名负向
  换票、现有受保护 API 和日志。

## 明确不做

- 不创建、删除或修改业务租户数据。
- 不自动创建 `tenant_registry` 记录或 Principal/RBAC 映射。
- 不运行会批量协调其他 AgentCiCi client 的 Realm 全量配置脚本。
- 不提交、推送、打标签或发布 Skill 仓库。

## 回滚

- 保留发布前 release 路径和环境文件备份。
- 新服务启动或 smoke 失败时，恢复环境文件、原子切回旧 release 并重启。
- Keycloak mapper 新增前导出 `semattice-cli` 配置；如需回滚，只删除本次新增
  的 mapper，不影响原有 client scope。

## 验收标准

- `semattice`、`nginx`、`postgresql-16`、`keycloak` 均为 active。
- 公网 `GET /healthz` 返回 200。
- 匿名 `POST /v1/auth/token` 返回 401 JSON `invalid_token` 且包含 `no-store`。
- `semattice-cli` access token audience包含 `semattice-api`，Organization scope仍
  保持分配状态。
- 服务日志无 Token、密钥、panic 或持续重启。
- 记录新旧 release、二进制校验和、环境备份和验证证据。

## 发布结果

- 当前 release：`/opt/semattice/releases/20260731T045751Z-standalone-auth`
- 上一 release：`/opt/semattice/releases/20260731T012059Z-console`
- 二进制 SHA-256：`73c552daffcf3ee2dcc203a009f08acc7b8effe3754e9a1d69267a690b3074f0`
- 环境备份：
  `/etc/semattice/semattice.env.backup.20260731T045751Z-standalone-auth`
- Keycloak client 备份：
  `/opt/keycloak/backups/20260731T045751Z-standalone-auth-before-sematttice-auth`
- `semattice-cli` audience mapper唯一且目标为 `semattice-api`；
  `organization` client scope仍为 optional。
- 真实 PKCE登录把 Organization alias `orgx2x8awt02djpp5xdp` 映射到 active tenant
  `ce85dabd-68be-503d-9d1b-9b63c536fa78`，OACT 调用
  `system.capability.list` 成功并发现 51 项能力。
- TASK-043发布时生产allowlist只开放`system.capability.read`，用于安全完成登录和能力
  发现；后续已由[FEAT-043](FEAT-043-skill-all-capability-scopes.md)按用户要求扩展为
  全部公开能力scope，Principal/RBAC、RLS、审批和审计边界保持不变。
