---
kind: feature-spec
feature_id: FEAT-020
title: Pure-Agent Capability Contract for API, MCP, and CLI
status: approved
owner_role: shared
task_ids: TASK-020
related_decisions: ADR-004, ADR-005 (proposed)
related_issues: none
updated_at: 2026-07-16T16:14:56Z
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

## L2 实施证据清单

以下清单把验收标准转为可重复运行的证据要求。当前 L1 只完成了清单设计；所有条目必须在用户批准 L2 allowlist 后，使用 Node 24 LTS 的真实代码、测试和命令输出验证。

| ID | 需证明的契约不变量 | 最小 L2 证据 | 当前状态 |
|---|---|---|---|
| CC-01 | 单一 registry 是已发布能力、版本、input/output schema、风险、权限、幂等、错误码和审计事件的唯一事实源 | registry 单元测试：注册、查找、重复 ID、schema 失败、未知 capability | `not_started_l1` |
| CC-02 | API、MCP、CLI 都只调用共享 invocation 层，不复制领域逻辑 | 适配器集成测试与依赖替身证明三入口均到达同一 invocation 函数 | `not_started_l1` |
| CC-03 | 同一成功输入的业务结果、能力 ID、请求 ID、审计 ID 和结果 schema 相等 | 对一个低风险样例能力运行 API/MCP/CLI 三入口 parity test | `not_started_l1` |
| CC-04 | 同一失败输入产生同一稳定错误码和可预测 transport 映射 | malformed input、未知 capability、未授权与幂等冲突的三入口负向测试 | `not_started_l1` |
| CC-05 | CLI 面向 Agent、没有 TTY 依赖或交互菜单 | 无 TTY 子进程测试：stdin JSON/flags 输入、stdout 仅 JSON/JSON Lines、stderr 仅诊断、非零退出码附 JSON 错误对象 | `not_started_l1` |
| CC-06 | STDIO MCP 不污染 JSON-RPC stdout，Tool schema 从 registry 投影 | in-process MCP 测试与子进程 smoke test；断言 `StdioServerTransport` stdout 无协议外日志 | `not_started_l1` |
| CC-07 | 幂等、审计、身份和租户上下文在三入口保持一致 | 重放同一 idempotency key 的契约测试；比较 audit event、actor、tenant、request ID 和 result | `not_started_l1` |
| CC-08 | 缺少任一入口投影或契约测试的能力不能发布 | publisher/registry gate 的失败测试；确认该 capability 不进入发现列表 | `not_started_l1` |
| CC-09 | 高风险或异步操作不以 CLI 提示取代治理状态 | 三入口都返回同一 `operation_id` / approval 状态，且 CLI 无 confirm prompt 的测试 | `not_started_l1` |

L2 完成判定要求 CC-01 至 CC-09 都有可复现命令输出，并由独立于实现者的验证者审阅。任何“仅 API 成功”或“仅 MCP 能启动”的结果都不足以证明 Capability Contract 已完成。

## 风险与缓解

- MCP Tool 数量增长：按命名空间和能力目录组织，并保留按需发现机制。
- 契约变更破坏调用方：使用语义版本、兼容性校验和弃用窗口。
- CLI 包装绕过策略：CLI 只调用共享 invocation 层，禁止直连数据库或内部服务。
- 三入口测试成本：将测试向量写为契约资产，针对三种 transport 复用同一套成功与失败案例。

## 交接说明

在技术栈 ADR 完成后，优先实现 capability registry、一个共享 invocation 层，以及记录读取、元数据读取、Changeset 校验三类样例能力。实现前必须将该 spec、`FEAT-009` 和上表的 CC-01 至 CC-09 一起阅读。
