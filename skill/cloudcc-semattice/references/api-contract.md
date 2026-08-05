# API 调用契约

## 目录

- [统一入口](#统一入口)
- [认证和租户边界](#认证和租户边界)
- [响应信封](#响应信封)
- [风险和审批](#风险和审批)
- [线上发现](#线上发现)
- [不属于本技能的接口](#不属于本技能的接口)

## 统一入口

所有公开原子能力使用同一个 HTTP 入口：

```text
POST /v1/capabilities/{capability-id}/invoke
```

请求头：

```http
Authorization: Bearer <short-lived-oact>
Content-Type: application/json
```

请求正文最大 1 MiB，只允许一个 JSON 对象，并拒绝未知顶层字段。建议正文只包含：

```json
{
  "request_id": "req-唯一请求标识",
  "idempotency_key": "idem-同一逻辑写操作的稳定标识",
  "input": {}
}
```

- `request_id`：必填，用于响应和审计关联；每次尝试使用可追踪的唯一值。
- `idempotency_key`：选填。重试完全相同的逻辑写操作时保持不变；不同输入禁止复用。当前三项 `usage.*` 能力没有启用幂等键，其余公开能力均已启用。
- `input`：必填，必须符合目标能力的 JSON Schema。
- `capability_id`：可省略；如果提供，必须与 URL 中的能力 ID 完全一致。
- `tenant_id`、`actor`：远程认证 API 中不要提交。服务会用已验证令牌绑定身份，冲突值会被拒绝。

## 认证和租户边界

使用短期 OACT 或配置中其他受信签发方签发的 Bearer JWT。服务端本地验证 issuer、audience、算法、签名、有效期和 scope，并从令牌绑定：

- `tenant_id`：唯一租户边界
- `company_id`：企业边界
- `sub` / `principal_id`：调用主体
- `scopes`：允许调用的能力范围
- `approvals`：已验证的审批标识
- 服务主体适用时的 `owner_principal_id` 和 `client_id`

禁止索要或保存用户密码、Keycloak 管理凭据、客户端密钥、数据库连接串或签名密钥。人工 CLI 续期所需的 Keycloak refresh token只能由 [登录凭据助手](authentication.md) 保存到操作系统凭据库，不得进入普通文件、仓库、日志或命令行；其他长期令牌不得保存。不要使用未经官方主体投影的 Keycloak 用户或服务账号令牌直连平台。

## 响应信封

成功响应：

```json
{
  "capability_id": "runtime.record.get",
  "request_id": "req-...",
  "audit_id": "audit:req-...",
  "status": "succeeded",
  "result": {}
}
```

失败响应：

```json
{
  "capability_id": "runtime.record.get",
  "request_id": "req-...",
  "audit_id": "audit:req-...",
  "status": "failed",
  "error": {
    "code": "RESOURCE_NOT_FOUND",
    "message": "..."
  }
}
```

HTTP 与稳定错误码映射：

| HTTP | 错误码 | 处理方式 |
|---|---|---|
| 400 | `VALIDATION_FAILED` | 对照线上 `input_schema` 修正输入，不添加 Schema 未声明字段 |
| 401 | `UNAUTHENTICATED` | 停止调用并刷新短期令牌 |
| 403 | `UNAUTHORIZED` | 报告 `required_scope`，禁止自行提权 |
| 404 | `CAPABILITY_NOT_FOUND` / `RESOURCE_NOT_FOUND` | 核对能力或资源标识，不探测其他租户 |
| 409 | `CONFLICT` / `IDEMPOTENCY_CONFLICT` | 读取当前状态，核对原请求和幂等键 |
| 412 | `FAILED_PRECONDITION` | 检查修订版本、生命周期、引用、变更集状态或审批 |
| 503 | `OVERLOADED` | 使用有界指数退避，并保持同一逻辑写操作的幂等键 |
| 500 | `INTERNAL` | 停止重复写入，保存 `request_id` 和 `audit_id` 供排查 |

## 风险和审批

- `low`：只读或低影响操作，通常同步执行。
- `medium`：创建或更新操作，必须由用户授权且环境明确。
- `high`：契约标记为异步并要求审批。开发阶段的 `metadata.version.publish` 首版本直接发布和 `metadata.changeset.approve` 采用手动确认，要求用户明确提供非空 `approval_id`，并由服务端持久审计 `approval_id` 与 `approval_mode=manual`；这两个能力不要求该值存在于可信令牌声明中。其他输入包含 `approval_id` 的高风险能力仍要求该值存在于可信令牌的 `approvals` 声明中。

不要根据“HTTP 返回成功”推断长任务已经完成。检查结果中的业务状态，例如 `pending_approval`、变更集 `state`、共享规则 `projection_state` 或操作 `completed`。

## 线上发现

调用 `system.capability.list` 获取当前发布的能力描述、版本、scope、风险、输入/输出 Schema、幂等和执行策略：

```bash
python3 scripts/semattice_api.py \
  --capability system.capability.list \
  --input '{}'
```

技能内 [API 能力目录](api-catalog.md) 用于快速规划；实际调用前以该接口返回的线上 Schema 为最终依据。

## 不属于 Capability 调用的接口

- `POST /v1/auth/token`：仅由登录助手使用 Keycloak access token换取短期 Semattice OACT；它不是业务 Capability，不接受租户或主体身份正文。
- `/console/session`、`/console/api/*`：浏览器管理中心会话和只读模拟治理数据，不是业务事实来源。
- `/mcp`：本技能明确不使用。
