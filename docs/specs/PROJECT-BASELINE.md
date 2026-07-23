---
kind: project-baseline
title: CloudCC Semattice project baseline
status: active_reference
owner_role: shared
updated_at: 2026-07-23T07:57:32Z
updated_by: project-manager
---

# PROJECT-BASELINE

## 项目概览

- 产品：**CloudCC Semattice（语义格）**，产品类别为 Agentic Business Data Runtime，定位为面向智能体的业务数据与语义运行底座。
- 代码仓库仍使用 `ai-native-platform`、`native_platform`、`AI_NATIVE_*` 及既有 Go module path 作为兼容标识；正式命名及迁移边界以 `.claw/decisions.md` 的 ADR-012 为准。
- 当前为 Go Phase 0/授权数据运行时 PoC：以统一 Capability Contract 将同一原子能力投影为 HTTP API、MCP 和无交互 JSON CLI。
- 本基线建立于 2026-07-23 的项目总管首轮盘点。它只记录可复用的当前事实和未知项，不替代任务、测试、决策或运维文件。
- 用户已确认项目已发布在线环境，当前处于小范围内测。内测反馈应以可复现问题和运行证据入库；它不改变单节点验证部署、无 HA/自动续证/备份恢复演练的既有边界。

## Verified Facts

- 技术栈为 Go 1.26.5、`slog`、环境变量配置、`pgxpool` 和显式 checksum 迁移；主入口是 `cmd/ai-native-platform`。
- 核心模块边界为 `internal/capability`（注册/调用契约）、`api`、`mcp`、`cli`、`identity`、`tenant`、`database`、`metadata`、`record`、`authorization`、`governance` 与 `operations`。
- 数据层是 PostgreSQL 16 PoC：128 tenant bucket、强制 RLS、事务级 `TenantContext`、control/runtime 双信任域连接，以及同租户关系约束。
- 元数据、记录和授权实现已经覆盖：版本化 metadata、Changeset、候选写入/回填/coverage gate、typed index、记录 CRUD/query、关系边、软删除/幂等、角色→Permission Set→原子权限、组织数据范围与受限 `record × group` 投影。
- 当前注册表发布 49 个 atomic capabilities；部署前的 API、MCP stdio 与 CLI 均由同一注册表/调用器投影。工作树还实现了认证的 Streamable HTTP MCP `/mcp`，其远端部署状态须以 TASK-024/运维记录为准。
- `.claw/` 是唯一状态目录；根目录 `README.md` 和 `AGENTS.md` 已声明 `agentic-project-guidelines`。该状态目录在本次盘点前通过该技能的校验器。
- 可重复本地验证入口：`./scripts/test-postgres.sh run`、`GOTOOLCHAIN=go1.26.5 go test -race ./...`、`GOTOOLCHAIN=go1.26.5 go vet ./...`。真实执行结果的唯一来源为 `.claw/test-report.md`。
- 单节点验证部署已存在：Nginx、PostgreSQL 16.13 和 systemd Semattice 服务；部署约束、健康检查、回滚与证书责任以 `.claw/devops.md` 和 `FEAT-022` 为准。

## Inferred Facts

- 该仓库从架构设计驱动的绿地项目，已演进为“已部署 PoC + 在制授权/集成增强”阶段；不应把已部署单节点验证环境误称为具备 HA、备份恢复或生产 SLA 的完整生产平台。
- `FEAT-009` 是跨阶段架构主规格，而非单一可直接完成的功能；后续工作应以独立任务和 feature spec 缩小交付范围。
- 当前工作区存在大量未提交的源码、规格、部署与 `.claw` 修改。这些改动属于在制工作，应在新的实现任务开始前先按任务边界审阅和验证，不能被项目治理文档覆盖或回退。

## Pending Verification

- TASK-019 尚未完成 8/16 GiB、50 并发、200 活跃用户、热点公平性与恢复相关的完整 Phase 0 验收；已有本机百万记录模拟不是生产 SLA。
- TASK-017 的通用 transactional outbox/worker 与告警尚未实现；这也是共享规则投影可靠性进一步运营化的前置能力。
- TASK-018 的 Agent Tool Gateway PoC 尚未开始。
- Streamable HTTP MCP 的工作树实现尚未在当前 ECS 制品上得到部署验证；当前 JWT/HS256 方案也不等同于完整 MCP OAuth resource server。
- 单节点部署未实现 HA、自动续证、备份与恢复演练；证书续期、发布与回滚须按 `FEAT-022` 和 `.claw/devops.md` 的明确边界执行。

## Legacy Hotspots

- `internal/database/migrate/`：迁移顺序与 checksum 是数据兼容和安全边界，禁止以重写历史迁移方式绕过失败。
- `internal/database/` 与 `internal/authorization/`：RLS、TenantContext、control/runtime 连接域、角色与共享投影耦合度高，改动必须走真实 PostgreSQL 测试。
- `internal/metadata/` 与 `internal/record/`：Changeset 生命周期、coverage、typed index 与记录可见性相互依赖，不能绕过 active index 或 publish gate 做基准。
- `.claw/current-status.md`：已有在制状态编辑，后续仅以其为当前任务/下一步的来源，不从本基线反向推导任务状态。

## Key Entry Points

- 架构与产品：`README.md`、`docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`、`.claw/goals.md`、`.claw/decisions.md`。
- 当前交付：`.claw/current-status.md`、`.claw/task-board.md`，以及 TASK-017/018/019 各自引用的规格。
- 代码：`cmd/ai-native-platform/main.go`、`internal/capability/registry.go`、`internal/api/handler.go`、`internal/mcp/server.go`、`internal/cli/run.go`。
- 数据与测试：`internal/database/migrate/`、`scripts/test-postgres.sh`、各 `internal/**/*_test.go`。
- 部署与运维：`deploy/semattice/`、`docs/specs/FEAT-022-semattice-single-node-https-deployment.md`、`.claw/devops.md`。

## Active Delivery Surface

- `TASK-017`：transactional outbox 与 worker 验证，优先级 high，owner 为 integration-agent。
- `TASK-018`：Agent Tool Gateway PoC，优先级 high，owner 为 backend-agent。
- `TASK-019`：隔离、容量与恢复验证，优先级 critical，owner 为 qa-agent。
- 上述任务的状态、依赖、负责人和下一步以 `.claw/task-board.md` 为唯一来源；本节只提供接手索引。

## Adoption Plan

- 继续只维护 `.claw/`，不创建 `.ai-dev/` 双写目录。
- 先用本基线让新接手者建立项目边界；仅在实际变更某一领域时更新对应 `FEAT-xxx`，不补写整段历史。
- 对在制工作区先进行 `git status`、任务卡和专项测试核对；治理任务不得混入实现改动。
- 当前三个后续任务保持独立规格/任务卡，避免把 outbox、Tool Gateway 和容量验收混为一个交付项。

## Handoff Notes

- 接手顺序：`.claw/current-status.md` → `.claw/task-board.md` → 本文件 → 任务所指的 feature spec → `.claw/test-report.md` / `.claw/devops.md`（按需）。
- 任何数据库改动先在隔离的 `127.0.0.1:55432` PostgreSQL 16 环境验证；不得把 control 与 runtime URL 混用，也不得记录密钥。
- 不把任务板中的历史完成事实复制到此文件；运行与部署结论必须以实际测试/运维证据文件为准。
