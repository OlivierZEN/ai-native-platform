---
kind: feature-spec
feature_id: FEAT-026
title: AgentCiCi-controlled company provisioning
status: verified
updated_at: 2026-07-24T02:20:00Z
updated_by: integration-agent after AgentCiCi company identity unification
owner_role: integration-agent
---

# FEAT-026 - AgentCiCi 校验的 Semattice 受控公司开户

## 目标

替代“可信 JWT 已含 tenant/company 身份即可开户”的前置假设。Semattice 新增仅供受信服务调用的公司开户入口：调用方提交 AgentCiCi 已创建的 `company_id`；Semattice 必须在写入本地 tenant projection 前向 AgentCiCi 原子校验并保留该公司，完成后回写结果。

## 调用模型

- AgentCiCi、运营工具及其他已登记服务均可发起；请求来源不拥有组织身份的最终解释权。
- Semattice inbound：`POST /internal/v1/company-provisionings`，使用 caller-specific HMAC，不使用用户 Bearer JWT。
- Semattice outbound：调用 AgentCiCi reservation / completion 接口，使用专用 HMAC service identity `semattice`。
- AgentCiCi 可作为已登记 caller 调用 Semattice，因此双方调用均可由 HMAC 验证；任一方向均不传递或记录密钥。

## 行为

1. 验证 caller HMAC、timestamp、nonce、body hash 与幂等键；非法/重放请求 fail closed。
2. 生成 Semattice tenant UUID 和 operation ID；调用 AgentCiCi reserve。
3. reserve 允许后，在本地事务创建唯一 `company_id` tenant projection 与操作审计。
4. 调用 completion；网络失败进入可重试失败状态，重试相同幂等键不得创建第二个 tenant。
5. 已绑定 `company_id`、无效/非活动 AgentCiCi 公司、冲突幂等键和 AgentCiCi 拒绝均返回稳定错误码。

## 安全与运维

- 规范签名字符串：`service_id + "\\n" + method + "\\n" + path + "\\n" + timestamp + "\\n" + nonce + "\\n" + sha256(body)`。
- 只接受显式 allowlist caller；五分钟时钟偏差；恒时比较；nonce 内存缓存作为重复请求快速门禁，业务幂等以数据库为准。
- 生产配置仅通过环境变量或系统密钥文件提供 base URL 与 secret；启动时缺少完整受控开户配置时，内部开户路由不可用且不得回退到无校验开户。
- 生产必须配置 `AI_NATIVE_AGENTCICI_BASE_URL`、`AI_NATIVE_AGENTCICI_HMAC_KEY` 和 `AI_NATIVE_PROVISIONING_CALLER_KEYS`。其中前两项用于 `semattice -> AgentCiCi`；caller keys 至少包含 `agentcici=<key>`，该值与 AgentCiCi 的 outbound key 成对配置。两个方向使用独立且长度不小于 32 的密钥。
- `tenant.provision` 不再作为已发布 Capability 或 HTTP/MCP/CLI 开户通道；所有新开户只能通过受控内部路由。缺少完整 AgentCiCi 对接与 HMAC 配置时，`serve` 必须拒绝启动。

## 验收

- AgentCiCi 校验通过时可开户并获得一致的 `company_id`、tenant ID、reservation/operation 关联。
- 无效签名、篡改 body、过期 timestamp、nonce 重放、未知 caller、无效公司、已绑定公司及并发同公司请求均被拒绝或幂等重放。
- AgentCiCi completion 失败和 Semattice数据库失败可重试、无重复开户、状态可查询。
- 定向单测、真实 PostgreSQL 集成、API 契约、race/vet/module/build、两边发布 dry-run 和部署后 smoke 全部通过。

## 发布阻断记录

2026-07-23 的 ECS 试发布已确认双向 HMAC 预约与完成回调可达，但生产数据库仅执行到 migration 12。该版本的本地 projection 因 migration 13 尚未将 `tenant_registry.org_id` 改为 `company_id` 而失败；新制品已原子回滚。恢复发布前必须通过专用 `semattice_migrator` 显式运行 `semattice db migrate`，验证 `schema_migration` 记录 version 13 后才可切换新制品。不得复用运行时 control/runtime 凭据、修改历史 migration 或执行 schema repair。

## 2026-07-24 发布前验证

- AgentCiCi 与 Semattice 在独立本机 PostgreSQL 16 环境完成真实 HTTP 联调：新建 AgentCiCi 公司后，内部 HMAC 入口成功完成 reserve、Semattice 本地 projection 与 complete；同一幂等键重试未新增 tenant 或 operation。
- Semattice 全量 PostgreSQL 测试、`go test -race ./...`、`go vet ./...`、`go mod verify` 与 linux/amd64 无 CGO 构建全部通过。MCP/CLI 无数据库模式已验证不会触发空计量服务；公开 `tenant.provision` 未发布。
- 生产已由独立 `semattice_migrator` 执行正向迁移并在完成后撤销临时 `CREATEROLE`，随后原子切换不可变制品；schema 已到 version 16。

## 2026-07-24 生产验收

- 新 release `/opt/semattice/releases/20260724T0045Z-controlled-provisioning` 的 SHA-256 为 `c1617398e9ddf3b83a942fa8b5852e54f7caf943900771703e8b1bacbf712962`；systemd 服务 active，edge `/healthz` 返回 `ok`。
- 从 AgentCiCi 生产主机发起的未签名 `POST /internal/v1/company-provisionings` 被 403 拒绝；使用 `agentcici` HMAC 的相同路径请求获认证，并在 AgentCiCi 不存在公司校验处稳定返回 `FAILED_PRECONDITION` / 412。该 smoke 未创建 tenant、reservation 或 operation。

## 2026-07-24 跨项目身份契约确认

- AgentCiCi 的 V94 将顶层租户数据库、API、JWT 和开发态上下文统一为 `company_id`；Semattice 不再需要或允许将入站 `company_id` 映射为 AgentCiCi `org_id`。
- 既有不可变企业编号的字符串值不重键，因此历史 `org...` 前缀仍可能作为 `company_id` 的值出现；字段名、授权边界与契约语义均为 `company_id`。
- `organization_id` 保留给未来公司内部组织架构，不能作为 Semattice 开户或跨产品身份字段。
