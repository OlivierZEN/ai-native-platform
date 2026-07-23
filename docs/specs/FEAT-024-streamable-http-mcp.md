---
kind: feature-spec
feature_id: FEAT-024
title: Authenticated Streamable HTTP MCP endpoint
status: verified
owner_role: backend-agent
task_ids: TASK-024
related_decisions: ADR-004, ADR-007
related_issues: none
updated_at: 2026-07-23T07:13:45Z
updated_by: ai after local Streamable HTTP MCP verification
---

# FEAT-024 - 认证的 Streamable HTTP MCP Endpoint

## 背景与目标

在保留本地 `mcp stdio` 传输的同时，让远程 MCP 客户端可通过标准 Streamable HTTP 协议连接同一 Capability Registry。端点固定为应用 HTTP 服务的 `/mcp`，不把已有 Capability API 伪装为 MCP，也不实现已被替代的旧 HTTP+SSE transport。

## 范围

### In Scope

- `serve` 进程提供认证的 `GET` / `POST` / `DELETE /mcp` Streamable HTTP MCP endpoint。
- 每个请求验证现有 Bearer JWT；Tool 调用只使用 token 绑定的 tenant、主体和 scopes。
- MCP stateful session 绑定 `tenant_id + subject`，阻止不同主体重放 session ID。
- Origin/Cross-Origin 防护和 SDK localhost DNS-rebinding 防护。
- 复用现有 MCP Tool 投影、Capability Registry、Invoker、审计和稳定错误码。

### Out Of Scope

- MCP OAuth authorization server、动态客户端注册、Protected Resource Metadata 和 token exchange。
- 旧 HTTP+SSE transport、跨进程 session/event persistence、SSE resumability。
- 修改现有 `mcp stdio`、CLI 或 `/v1/capabilities/*` API 协议。
- 将本次未部署的工作树变更声明为已上线到 ECS。

## 方案设计

```text
MCP client -- Bearer JWT --> /mcp
  -> Cross-Origin protection
  -> JWT verifier (every HTTP request)
  -> tenant-qualified authenticated Streamable HTTP session
  -> MCP Tool projection
  -> shared Capability Invoker -> domain handler / audit
```

`identity.Verifier.VerifyWithExpiration` 复用同一 issuer/audience/algorithm/expiry 校验，并将已验证 principal 与过期时间传给 MCP SDK Bearer middleware。SDK 以 tenant-qualified subject 作为 session user ID，在后续请求中拒绝主体不匹配的 session。会话空闲五分钟自动关闭；没有 EventStore，因此不承诺断连重放。

## 接口与兼容性

- 新 endpoint：`http(s)://<host>/mcp`。
- 客户端须按 MCP Streamable HTTP 协议发送 JSON-RPC；POST 的 `Accept` 同时包含 `application/json,text/event-stream`，并携带 `Authorization: Bearer <JWT>`。
- 认证 MCP Tool 参数仅需要 `request_id` 和 `input`；租户、actor 和 scopes 从 JWT 绑定，客户端提供冲突身份会被拒绝。
- 现有 `/v1/capabilities/<id>/invoke`、CLI 和 `mcp stdio` 保持不变。

## 验收与验证

- [x] MCP SDK `StreamableClientTransport` 成功完成 initialize 与 `system_capability_list` 工具调用。
- [x] 调用审计记录使用 JWT 绑定的 tenant 和 actor。
- [x] 无 Bearer token 的请求返回 HTTP 401。
- [x] `go test ./...`、`go test -race ./...`、`go vet ./...`、`go mod verify`、本机 build 与差异检查通过。

## 风险与后续

- 远程 deployment 尚未更新；上线前必须由单独部署任务重建制品并验证 Nginx `/mcp` 反向代理不会缓冲 SSE。
- 当前 JWT 方案是项目既有 HS256 验证，不等于完整 MCP OAuth resource-server 实现；后者需要单独规格与身份系统授权。
- 多节点或长任务场景需要持久 EventStore、session 策略和容量验证。
