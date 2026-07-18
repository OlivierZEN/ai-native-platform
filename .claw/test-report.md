---
kind: test-report
version: 3
updated_at: 2026-07-18T15:51:53Z
updated_by: ai pre-publish verification + prior independent checker
last_run_at: 2026-07-18T15:51:53Z
last_run_status: passed
---

# 测试报告

## 当前运行摘要

- 状态：`passed`（此结果仅覆盖本次单一低风险 Capability Contract PoC）。
- 2026-07-18 发布前复测：`GOTOOLCHAIN=go1.26.5 go test ./... -count=1`、`GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`GOTOOLCHAIN=go1.26.5 go vet ./...`、`GOTOOLCHAIN=go1.26.5 go mod verify`（`all modules verified`）、`validate-state.py .claw` 和 `git diff --check` 均通过。
- 同次发布前复测：`CGO_ENABLED=0 -trimpath` 的 `linux/amd64`、`linux/arm64`、`darwin/arm64`、`windows/amd64` 构建全部通过，临时产物已清理；敏感凭据模式扫描和 5 MB 以上文件检查无发现。
- 独立 checker 复跑：上述 Go 命令全部通过；`CGO_ENABLED=0 -trimpath` 的 `linux/amd64`、`linux/arm64`、`darwin/arm64`、`windows/amd64` 构建全部通过，临时产物已清理；其核对确认 API/CLI/MCP 均进入同一 `Invoker`、已发布 Registry 投影、草稿拒绝、无 TTY CLI、MCP stdout JSON-RPC 以及 denylist 路径。
- 已修复并覆盖：MCP 子进程测试的 `stderr` data race、并发幂等双执行、重放审计身份不一致、硬编码 MCP Tool、API/CLI/MCP 成功与三类稳定错误 parity、CLI `describe`、多 JSON 文档输入。
- 依赖：`github.com/modelcontextprotocol/go-sdk v1.6.1` 及其传递图已记录在 `docs/specs/FEAT-020-go-dependency-gate.md`；本地 PoC 许可门禁已接受。生产内部 proxy、SBOM/notice/签名、持久审计、高风险异步 `operation_id` 和通用输出 Schema 运行时校验仍未实现，也不在本测试结果中。

L1 历史审计工具及其结果已完成使命并归档在 `docs/archive/loop-engineering/2026-07-16-l1-test-report.md`；它们不属于当前 Go 运行时的测试基线。
