---
kind: decisions
version: 3
updated_at: 2026-07-16T16:12:40Z
updated_by: ai
---

# 技术决策记录

`decisions.md` 是架构和技术选型的唯一事实源。

## 决策索引

| 编号 | 标题 | 状态 | 日期 | 替代/被替代 |
|------|------|------|------|-------------|
| ADR-003 | AI 原生 CRM + PaaS 目标架构基线 | proposed | 2026-07-13 | - |
| ADR-004 | 纯 Agent 能力契约与三入口要求 | accepted | 2026-07-16 | - |
| ADR-005 | Phase 0 Capability Contract PoC 运行时 | proposed | 2026-07-17 | - |

推荐状态值：`proposed` / `accepted` / `rejected` / `superseded`

## 新增决策时记录

每条 ADR 至少记录以下信息：

- 编号，如 `ADR-001`
- 状态
- 日期
- 背景
- 备选方案
- 最终结论
- 为什么这个方案胜出
- 后续影响
- 验证方式
- 参考资料

## ADR-003 - AI 原生 CRM + PaaS 目标架构基线

- 状态：`proposed`
- 日期：2026-07-13
- 背景：需要从零构建可横向扩展、可安全配置并可由 Agent 受控操作的企业 CRM + PaaS 平台。
- 结论：以逻辑 OneDatabase、物理多 PostgreSQL 分片为租户数据底座；使用显式租户路由与 RLS；以版本化元数据和 Changeset 统一人工与 Agent 配置；以 JSONB 权威记录与按需类型化索引支持自定义对象；将搜索、分析和异步集成从 OLTP 事务面分离。
- 原因：该方案在租户隔离、无 DDL 开户、动态扩展、事务一致性和后续横向扩容间取得可验证的平衡。
- 后续影响：Phase 0 必须验证分区/RLS、记录与索引、Changeset 编译发布，以及沙箱和 outbox 等高风险假设；具体技术栈与组件选型仍待独立 ADR。
- 验证方式：见 `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md` 的 Phase 0、测试与验收章节。

## ADR-004 - 纯 Agent 能力契约与三入口要求

- 状态：`accepted`
- 日期：2026-07-16
- 背景：平台由智能体而非人类页面操作；若 API、MCP 与 CLI 各自实现业务语义，会产生权限、审计、幂等和错误处理漂移。
- 结论：每个已发布的原子能力必须从统一 Capability Contract 派生 REST/功能 API、MCP Tool 与非交互式 CLI。三入口使用同一输入/输出 schema、权限与风险检查、幂等规则、错误码和审计语义。Web、移动端、BFF 和人类交互式 CLI 不在范围内。
- 原因：统一契约将 Agent 能力发现、远程编排、本地自动化和测试收敛至同一业务行为，而不复制实现逻辑。
- 后续影响：新增能力必须补齐三入口及其契约测试；`Agent Tool Gateway` 扩展为 MCP 服务，CLI 只接受结构化输入并输出 JSON/JSON Lines。
- 验证方式：见 `docs/specs/FEAT-020-pure-agent-capability-contract.md`。

## ADR-005 - Phase 0 Capability Contract PoC 运行时

- 状态：`proposed`
- 日期：2026-07-17
- 背景：`ADR-004` 要求一个原子能力从同一 Capability Contract 投影为功能 API、MCP Tool 与非交互式 CLI。项目需要一个小而可验证的 Phase 0 运行时，以证明三入口共享调用、校验、权限、幂等、错误和审计语义，而不是各自复制业务实现。
- 证据：Node 官方发布页在 2026-07-17 显示 v24（Krypton）为 LTS、v26 为 Current，并要求生产应用使用 Active 或 Maintenance LTS。MCP TypeScript SDK 文档说明 `McpServer` 可结合 `StdioServerTransport` 建立本地进程型 MCP Server，SDK 使用 Zod schema；STDIO 传输的 stdin/stdout 用于 JSON-RPC。
- 提案：对**仅限 Phase 0 的 Capability Contract PoC**，使用 Node.js 24 LTS、TypeScript 和 ESM；使用 `@modelcontextprotocol/sdk` 与 Zod 定义 MCP Tool schema；以一个共享 invocation 层承载契约语义。HTTP/功能 API、MCP Tool 和 CLI 仅作为 transport adapter。MCP 的本地 PoC 使用 STDIO，诊断信息只能写入 stderr，stdout 保留给协议消息。CLI 接收 flags 或 stdin JSON，并只输出 JSON/JSON Lines，不提供交互菜单或确认提示。
- 备选方案：
  - Node 26 Current：本机当前版本可用于只读工具检查，但不作为此 PoC 的 CI 或生产基线，因为其仍是 Current 而不是 LTS。
  - 为 API、MCP、CLI 分别实现 handler：拒绝；会违反 `ADR-004` 的同源契约和等价性要求。
  - 直接指定完整 CRM 的多服务生产栈：暂缓；数据库、部署、消息、可观测性和容量选择仍需要 `TASK-010` 的独立证据。
- 影响：L2 获批后，首个垂直切片只允许创建 capability registry、共享 invocation 层、一个低风险样例能力、三入口适配器及契约测试。依赖版本、构建脚本、CI 和部署不在本 ADR 的批准范围内，且不得在当前 L1 窗口创建。
- 接受条件：`TASK-009` 的架构基线已批准；用户明确批准该 PoC 的 L2 allowlist；独立验证者可检查 API/MCP/CLI 结果一致性；Node 24 的实际构建与测试命令已被 L2 运行证据验证。
- 验证方式：在获得 L2 授权后，使用 Node 24 运行同一 capability input 经 API、MCP 和 CLI 的契约测试，断言输出 schema、错误码、审计字段与幂等行为一致；STDIO 集成测试同时断言 stdout 没有协议外日志。
- 参考资料：
  - https://nodejs.org/en/about/previous-releases
  - https://ts.sdk.modelcontextprotocol.io/server
  - https://modelcontextprotocol.io/docs/develop/build-server
  - `docs/specs/FEAT-020-pure-agent-capability-contract.md`

## 维护规则

- 只记录非平凡技术决策。
- 决策变更时，不删除历史，新增或更新状态。
- 必须写清楚为什么选它，而不只是选了什么。
