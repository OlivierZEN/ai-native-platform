# Loop Engineering 状态

## 活跃 L2 循环

- Pattern: `phase0-go-capability-contract-l2`
- Level: `L2 — assisted implementation`
- Window: 2026-07-18 00:03:53–05:03:53 Asia/Shanghai
- Goal: 一个低风险能力从同一 Go Capability Contract 投影为 API、MCP 与无交互 CLI，并具备可复现的 parity 证据。
- Authorization: 用户于 2026-07-18 明确授权 L2 原码工作。该授权仅覆盖本 PoC，不批准生产部署、数据库迁移、HA/备份、远端发布或 FEAT-009 的其余未决组件。
- State: `STATE.md`、`.claw/current-status.md`、`.claw/task-board.md`、`loop-budget.md`、`loop-constraints.md` 与 `loop-run-log.md`。
- Outcome: 2026-07-18 00:46:51 Asia/Shanghai 独立验证完成单个低风险 `system.capability.list` PoC；循环已停止源码修改，后续范围需要新的 L2 检查点。

## 五小时运行计划

| 窗口 | 单一可验证目标 | 停止/升级条件 |
|---|---|---|
| 00:00–00:35 | L2 约束、预算、日志、Go 1.26.5 工具链与依赖许可门禁 | 工具链或依赖不可审计即只实施标准库核心，并记录缺口 |
| 00:35–01:30 | Capability Registry 与 Invocation 测试优先垂直切片 | 同一项失败三次即暂停并升级 |
| 01:30–02:20 | 非交互 CLI 与功能 API parity | 禁止 TTY 提示或重复业务逻辑 |
| 02:20–03:20 | 官方 Go MCP SDK 的 stdio Tool 投影与 stdout 纯净测试 | 依赖/协议问题不得以自造不兼容协议绕过 |
| 03:20–04:15 | 错误、幂等、审计、受限并发与交叉构建证据 | 不触碰数据库迁移、基础设施或发布 |
| 04:15–05:00 | 独立验证、状态/测试报告、停机与交接 | 验证者不能是实现者；未验证项如实保留 |

## L2 路径 Allowlist

允许：`go.mod`、`go.sum`、`cmd/ai-native-platform/`、`internal/capability/`、`internal/api/`、`internal/cli/`、`internal/mcp/`、`schemas/`、`docs/`、`.claw/` 与本 Loop 控制文件。

拒绝：密钥、环境文件、身份/授权实现、支付/计费、CI、生产基础设施、Docker 配置、数据库迁移、生产数据库写入、自动合并、推送、PR、发布和部署。完整规则见 `loop-constraints.md`。

## Maker / Checker 与停机规则

- 当前实现者只能报告证据，不能宣布完成。
- 最终 API/MCP/CLI parity、stdout 纯净、无 TTY 和交叉构建由独立验证者执行。
- 每个工作项最多三次失败尝试；任一 denylist 路径、依赖许可不明、凭据、生产连接或范围歧义都会立即暂停并升级给用户。
- 达到窗口结束、预算 80%、或没有可验证工作项时，停止源码修改并记录交接。

已完成的 L1 原始证据保留在 `docs/archive/loop-engineering/`，不构成本次 L2 的技术前提。
