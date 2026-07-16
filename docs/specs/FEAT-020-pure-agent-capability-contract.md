---
kind: feature-spec
feature_id: FEAT-020
title: Pure-Agent Capability Contract for API, MCP, and CLI
status: approved
owner_role: shared
task_ids: TASK-020
related_decisions: ADR-004
related_issues: none
updated_at: 2026-07-16T15:59:39Z
updated_by: ai
---

# FEAT-020 - 纯 Agent 能力契约：API、MCP 与 CLI

## 背景与目标

平台是纯 AI Native 系统，不提供 Web、移动端、BFF 或人工交互式控制台。所有业务、配置和运维均由 Agent 操作。为避免同一功能在功能 API、MCP Tool 和 CLI 中出现不同的权限、幂等、审计和错误语义，每个已发布原子能力必须从同一个版本化 Capability Contract 派生三个入口。

## 范围

### In Scope

- Capability Contract：能力 ID、动作、输入/输出 JSON Schema、版本、同步/异步语义、权限、风险、配额、幂等、错误码和审计事件。
- 功能 API、MCP Service 和 `agent-cli` 对同一契约的适配。
- 三入口的能力发现、调用、异步 operation 查询、审计和契约测试。
- 无 TTY、结构化输入输出的 CLI 运行模式。

### Out Of Scope

- Web、移动端、BFF、管理页面、页面渲染协议和人类交互式 CLI。
- 为 API、MCP、CLI 分别维护或复制业务实现。
- 未注册为 Capability Contract 的内部实现细节对 Agent 暴露。

## 方案设计

### 统一调用路径

```text
API request / MCP tool call / CLI JSON invocation
  -> transport adapter
  -> Capability Contract validation and normalization
  -> identity, permission, quota, risk and idempotency checks
  -> shared domain handler
  -> audit event + result / operation_id
```

Transport adapter 只负责协议转换；领域 handler 不识别其调用来自 API、MCP 或 CLI。所有入口都携带执行者身份、租户上下文、请求 ID 与幂等键，且都产出同一结果 schema 和稳定错误码。

### 三入口规范

| 入口 | 发现方式 | 调用与输出 |
|---|---|---|
| API | OpenAPI / capability registry | JSON 请求和响应；版本化路径；异步返回 `operation_id` |
| MCP | 从 registry 投影 Tool 与 JSON Schema | Tool 名与 capability ID 稳定映射；调用结果遵循同一输出 schema |
| CLI | 内置 `capability list/describe` 与子命令帮助的机器可读模式 | JSON 参数或 stdin JSON；标准输出 JSON/JSON Lines；非零退出码与 JSON 错误对象 |

CLI 不显示菜单、确认提示或依赖终端会话状态。高风险操作的审批是契约中的异步状态，而非 CLI 交互；调用者提交请求后使用相同的 operation 查询能力轮询或订阅状态。

### 发布门禁

能力发布器必须验证 API、MCP 与 CLI 投影均可生成，且输入/输出 schema、授权、风险、幂等和审计规则相同。任一投影或契约测试失败时，能力不得标记为 published。

## 验收标准

- 第一批记录、元数据和 Changeset 能力具有一份权威 Capability Contract。
- 每项能力可通过 API、MCP Tool、CLI 成功调用，并产生相同业务结果、错误码与审计字段。
- CLI 在没有 TTY 的 CI/Agent 环境中运行，所有输入输出均为 JSON 或 JSON Lines。
- API/MCP/CLI 不存在独立的业务权限、审计或领域逻辑实现。
- 高风险和异步操作在三入口中都遵循相同的审批与 `operation_id` 状态模型。

## 风险与缓解

- MCP Tool 数量增长：按命名空间和能力目录组织，并保留按需发现机制。
- 契约变更破坏调用方：使用语义版本、兼容性校验和弃用窗口。
- CLI 包装绕过策略：CLI 只调用共享 invocation 层，禁止直连数据库或内部服务。
- 三入口测试成本：将测试向量写为契约资产，针对三种 transport 复用同一套成功与失败案例。

## 交接说明

在技术栈 ADR 完成后，优先实现 capability registry、一个共享 invocation 层，以及记录读取、元数据读取、Changeset 校验三类样例能力。实现前必须将该 spec 与 `FEAT-009` 一起阅读。
