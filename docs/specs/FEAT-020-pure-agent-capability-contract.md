---
kind: feature-spec
feature_id: FEAT-020
title: Pure-Agent Capability Contract for API, MCP, and CLI
status: approved
owner_role: shared
task_ids: TASK-020
related_decisions: ADR-004, ADR-007 (accepted)
related_issues: none
updated_at: 2026-07-17T16:46:51Z
updated_by: ai + independent checker
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

以下清单把验收标准转为可重复运行的证据要求。当前 L1 只完成了清单设计；所有条目必须在用户批准 L2 allowlist 后，使用 Go 1.26.5 的真实代码、测试和命令输出验证。

| ID | 需证明的契约不变量 | 最小 L2 证据 | 当前状态 |
|---|---|---|---|
| CC-01 | 单一 registry 是已发布能力、版本、input/output schema、风险、权限、幂等、错误码和审计事件的唯一事实源 | `internal/capability/registry_test.go` 覆盖发布 metadata、schema 与高风险门禁、草稿隐藏、幂等/审计 | `independently_verified_for_poc` |
| CC-02 | API、MCP、CLI 都只调用共享 invocation 层，不复制领域逻辑 | 三个 adapter 均规范化为 `capability.Request` 并调用 `Invoker.Invoke`；适配器 parity 测试覆盖其可观察结果 | `independently_verified_for_poc` |
| CC-03 | 同一成功输入的业务结果、能力 ID、请求 ID、审计 ID 和结果 schema 相等 | `TestAPICLIAndMCPProduceEquivalentResult` 进行语义 JSON parity 比较 | `independently_verified_for_poc` |
| CC-04 | 同一失败输入产生同一稳定错误码和可预测 transport 映射 | `TestAPICLIAndMCPUseSameStableFailureCodes` 覆盖输入验证、未授权、幂等冲突；API/CLI 还覆盖多 JSON 文档。未发布 MCP tool 按协议发现缺失，不伪造为已发布 capability 的业务错误。 | `independently_verified_for_poc` |
| CC-05 | CLI 面向 Agent、没有 TTY 依赖或交互菜单 | `cmd/ai-native-platform/main_test.go` 子进程经 stdin JSON 验证单一 JSON stdout、无 stderr；错误为结构化 JSON 和非零退出码 | `independently_verified_for_poc` |
| CC-06 | STDIO MCP 不污染 JSON-RPC stdout，Tool schema 从 registry 投影 | `internal/mcp/server_test.go` 校验 published registry 投影；子进程 stdio 测试只解析 JSON-RPC stdout | `independently_verified_for_poc` |
| CC-07 | 幂等、审计、身份和租户上下文在三入口保持一致 | `TestAPICLIAndMCPKeepReplayAuditContext` 与并发幂等测试覆盖重放 request/audit identity、actor、tenant 和 result | `independently_verified_for_poc` |
| CC-08 | 缺少任一入口投影或契约测试的能力不能发布 | 已发布定义必须具备 object input/output schema；草稿从 registry/API invocation/CLI discovery/MCP tools 排除；高风险同步定义拒绝注册 | `independently_verified_for_poc` |
| CC-09 | 高风险或异步操作不以 CLI 提示取代治理状态 | 该低风险、同步 PoC 不实现高风险 operation；registry 已拒绝发布缺少 async approval 的高风险定义，且 CLI 没有确认提示。完整 `operation_id` 生命周期属于后续 Changeset/operation capability。 | `bounded_poc_guard_only` |

完整 FEAT-020 的完成判定仍要求 CC-01 至 CC-09 都有可复现命令输出，并由独立于实现者的验证者审阅。2026-07-18 的受限 L2 只完成一个低风险同步能力：CC-01 至 CC-08 已独立验证为该 PoC 适用，CC-09 仅验证发布门禁；高风险异步 `operation_id` 生命周期不在本次完成声明内。任何“仅 API 成功”或“仅 MCP 能启动”的结果都不足以证明 Capability Contract 已完成。

## 风险与缓解

- MCP Tool 数量增长：按命名空间和能力目录组织，并保留按需发现机制。
- 契约变更破坏调用方：使用语义版本、兼容性校验和弃用窗口。
- CLI 包装绕过策略：CLI 只调用共享 invocation 层，禁止直连数据库或内部服务。
- 三入口测试成本：将测试向量写为契约资产，针对三种 transport 复用同一套成功与失败案例。

## 交接说明

在技术栈 ADR 完成后，优先实现 capability registry、一个共享 invocation 层，以及记录读取、元数据读取、Changeset 校验三类样例能力。实现前必须将该 spec、`FEAT-009` 和上表的 CC-01 至 CC-09 一起阅读。
