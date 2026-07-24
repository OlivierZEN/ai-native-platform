---
kind: feature-spec
feature_id: FEAT-024
title: Public-discovery, authenticated-invocation Streamable HTTP MCP endpoint
status: implemented
owner_role: backend-agent
task_ids: TASK-024
related_decisions: ADR-004, ADR-007
related_issues: none
updated_at: 2026-07-24T15:49:11Z
updated_by: backend-agent after anonymous-discovery handler implementation
---

# FEAT-024 - 公开发现、认证调用的 Streamable HTTP MCP Endpoint

## 背景与目标

在保留本地 `mcp stdio` 传输的同时，让远程 MCP 客户端可通过标准 Streamable HTTP 协议连接同一 Capability Registry。端点固定为应用 HTTP 服务的 `/mcp`，不把已有 Capability API 伪装为 MCP，也不实现已被替代的旧 HTTP+SSE transport。

## 范围

### In Scope

- `serve` 进程提供 Streamable HTTP `GET` / `POST` / `DELETE /mcp` endpoint。
- `initialize`、`notifications/initialized` 与 `tools/list` 可匿名调用，供外部 MCP 客户端发现公开发布的工具元数据。
- 所有其他 MCP 请求（特别是 `tools/call`）每次都验证 Bearer JWT；Tool 调用只使用 token 绑定的 tenant、主体和 scopes。
- 携带 Bearer JWT 创建的 MCP stateful session 绑定 `tenant_id + subject`，阻止不同主体重放 session ID；匿名 discovery session 不携带身份，且不授予调用权限。
- Origin/Cross-Origin 防护和 SDK localhost DNS-rebinding 防护。
- 复用现有 MCP Tool 投影、Capability Registry、Invoker、审计和稳定错误码。

### Out Of Scope

- MCP OAuth authorization server、动态客户端注册、Protected Resource Metadata 和 token exchange。
- 旧 HTTP+SSE transport、跨进程 session/event persistence、SSE resumability。
- 修改现有 `mcp stdio`、CLI 或 `/v1/capabilities/*` API 协议。

## 方案设计

```text
MCP client --> /mcp
  -> Cross-Origin protection
  -> initialize / notifications/initialized / tools/list: anonymous discovery only
  -> all other methods: JWT verifier (every HTTP request)
  -> authenticated tool call binds tenant-qualified principal
  -> MCP Tool projection -> shared Capability Invoker -> domain handler / audit
```

`identity.Verifier.VerifyWithExpiration` 复用同一 issuer/audience/algorithm/expiry 校验，并将已验证 principal 与过期时间传给 MCP SDK Bearer middleware。每一次实际调用都从该请求的 SDK `RequestExtra` 提取已验证 principal，再以它绑定调用；不会信任 discovery session ID、客户端提交的 tenant/actor/scopes 或前序请求。SDK 以 tenant-qualified subject 作为带 token 会话的 user ID，在后续请求中拒绝主体不匹配的 session。会话空闲五分钟自动关闭；没有 EventStore，因此不承诺断连重放。

## 接口与兼容性

- 新 endpoint：`http(s)://<host>/mcp`。
- 客户端须按 MCP Streamable HTTP 协议发送 JSON-RPC；POST 的 `Accept` 同时包含 `application/json,text/event-stream`。
- 不带 Bearer JWT 时仅可使用 `initialize`、`notifications/initialized`、`tools/list`；`tools/call`、`GET`、`DELETE`、未知方法、批次中含任一非发现方法与格式非法请求均要求 `Authorization: Bearer <JWT>`。
- 认证 MCP Tool 参数仅需要 `request_id` 和 `input`；租户、actor 和 scopes 从 JWT 绑定，客户端提供冲突身份会被拒绝。
- 现有 `/v1/capabilities/<id>/invoke`、CLI 和 `mcp stdio` 保持不变。

## 验收与验证

- [x] MCP SDK `StreamableClientTransport` 成功完成 initialize 与 `system_capability_list` 工具调用。
- [x] 调用审计记录使用 JWT 绑定的 tenant 和 actor。
- [x] 无 Bearer token 的 `initialize` 与 `tools/list` 成功，且不触发 token verifier。
- [x] 无 Bearer token 的规范 `tools/call` 返回 HTTP 401，且不创建审计事件。
- [x] `go test ./...`、`go test -race ./...`、`go vet ./...`、`go mod verify`、本机 build 与差异检查通过。

## 风险与后续

- 2026-07-24 已在授权 ECS 配置 Nginx 精确代理 `/mcp`，关闭 request/response buffering。发布本实现后，公网应允许匿名 MCP discovery，并继续拒绝没有 Bearer JWT 的工具调用；发布验收需要分别记录这两类结果。
- 当前验证器支持固定信任清单中的 JWKS/RS256，以及兼容期 HS256；这不等于完整 MCP OAuth resource-server 实现，后者仍需要独立规格与身份系统授权。
- 多节点或长任务场景需要持久 EventStore、session 策略和容量验证。
