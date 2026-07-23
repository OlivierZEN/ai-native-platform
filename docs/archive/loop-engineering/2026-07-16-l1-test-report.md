---
kind: test-report
version: 3
updated_at: 2026-07-16T17:03:46Z
updated_by: ai
last_run_at: 2026-07-16T17:03:46Z
last_run_status: passed
---

# 测试报告

`test-report.md` 只记录真实执行过的测试或验证结果，不记录猜测。

推荐状态值：`passed` / `failed` / `partial` / `not_run`

## 最新运行摘要

- 状态：`passed`
- 范围：`agentic-project-guidelines` 状态文件与 Loop Engineering L1 控制面及运行日志完整性
- 命令：`python3 /Users/owenmacbook/.agents/skills/y-agentic-project-guidelines/scripts/validate-state.py .claw`
- 环境：`local workspace`
- 附加证据：`npx @cobusgreyling/loop-audit . --suggest`，结果为 `89/100`、`L1`；安全策略、停滞升级、MCP 范围与工作树策略已被审计识别
- 运行日志校验：Node 解析 8 条 JSONL 记录；run ID 单调递增、`source_actions` 总数为 0、时长校正记录存在
- 最新 Loop 审计：`npx @cobusgreyling/loop-audit .` 返回 `89/100`、`L1`；Node 解析 9 条 JSONL 记录且 `source_actions` 总数为 0
- 运行时可用性检查：本机为 Node `v26.0.0`，未发现 Node 24 或版本管理器；这是一项 L2 前置条件缺口，不是已验证的构建环境
- 远端只读核对：GitHub `main` 与 `origin/main` 均为 `3c8c961`；本地证据提交保持未推送

## 结果汇总

| 类型 | 总数 | 通过 | 失败 | 跳过 | 覆盖率 |
|------|------|------|------|------|--------|
| 单元测试 | 0 | 0 | 0 | 0 | 0% |
| 集成测试 | 0 | 0 | 0 | 0 | 0% |
| E2E 测试 | 0 | 0 | 0 | 0 | - |
| 治理状态校验 | 1 | 1 | 0 | 0 | - |
| Loop 运行日志完整性 | 1 | 1 | 0 | 0 | - |
| Loop Readiness 审计 | 1 | 1 | 0 | 0 | - |
| 总计 | 3 | 3 | 0 | 0 | - |

## 失败项

- 暂无失败项。

## 覆盖率趋势

| 日期 | 行覆盖率 | 分支覆盖率 | 函数覆盖率 |
|------|----------|------------|------------|
| 2026-07-16 | - | - | - |

## 常用测试命令

- 在项目命令明确后，记录真实可运行的测试命令。
- 不要在这里保留通用占位命令或猜测性的命令。

## 维护规则

- 只有在实际运行命令后才更新这里。
- `current-status.md` 只应摘录一行测试摘要。
- 如果没有运行测试，明确写 `not_run`，不要留空也不要虚构结果。
