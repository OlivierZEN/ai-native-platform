---
kind: feature-spec
feature_id: FEAT-010
title: Go engineering and PostgreSQL foundation
status: approved
owner_role: backend-agent
task_ids: TASK-010
related_decisions: ADR-007, ADR-008
related_issues: none
updated_at: 2026-07-18T17:19:29Z
updated_by: ai maker after independent checker PASS
---

# FEAT-010 - Go 工程与 PostgreSQL 基线

## 目标

把现有 Capability Contract PoC 固化为可启动、可迁移、可测试的 Go 单运行时工程。运行时使用 Go 1.26.5；配置只来自参数和环境，日志使用 `log/slog`，数据库使用 `pgx/v5 + pgxpool`，迁移使用仓库内置、版本化、带 checksum 的 SQL runner。

## 依赖与许可门禁

| 直接依赖 | 锁定版本 | 许可证 | 用途 |
|---|---:|---|---|
| `github.com/modelcontextprotocol/go-sdk` | `v1.6.1` | MIT | MCP SDK，既有依赖 |
| `github.com/jackc/pgx/v5` | `v5.10.0` | MIT | PostgreSQL protocol、事务和连接池 |
| `github.com/google/uuid` | `v1.6.0` | BSD-3-Clause | typed TenantContext、UUIDv4 解析及 UUIDv7 元数据 ID |
| `github.com/golang-jwt/jwt/v5` | `v5.3.1` | MIT | 可注入的 JWT 验证边界 |

迁移不引入额外框架，避免另一个可执行文件和供应链。`go mod tidy` 后必须记录完整 module graph，执行 `go mod verify` 并复核新增传递依赖许可证；任何非允许许可阻止完成。

## 配置与日志

- 配置包含 HTTP 地址、数据库 URL、pool 上限/空闲/生命周期、日志 level/format、身份 issuer/audience/algorithm/key 来源。
- 无默认生产凭据；数据库与身份密钥缺失时只有不需要它们的纯本地命令可以运行。
- 错误不得回显 DSN、JWT、密钥或密码；日志字段必须支持 request、tenant、actor、operation 关联。
- CLI 保持非交互：只读 flags/stdin，stdout 仅 JSON/JSON Lines 或 MCP JSON-RPC，诊断写 stderr。

## 数据库与迁移

- `internal/database` 统一解析 pgxpool 配置、设置连接上限和可观测 application name。
- 三种连接身份严格分离：`AI_NATIVE_DATABASE_URL` 只供显式 migration；`AI_NATIVE_CONTROL_DATABASE_URL` 使用非 owner/非 BYPASSRLS 的 `ai_native_control`，通过专属 RLS policy 跨租户维护 registry/operation/audit；`AI_NATIVE_RUNTIME_DATABASE_URL` 使用严格 TenantContext 的 `ai_native_runtime`。
- `internal/database/migrate` 嵌入顺序 SQL，使用 advisory lock 防并发迁移，在 `schema_migration(version, name, checksum, applied_at)` 持久化历史。
- 已应用 migration 内容变化必须拒绝；失败 migration 回滚且不登记版本；重复执行无副作用。
- 应用启动只检查 schema；实际迁移由显式 `db migrate` 命令执行。

## 测试结构

- 单元测试与包同目录；需要 PostgreSQL 的集成测试以 `TEST_DATABASE_URL` 门控，无变量时明确 skip。
- 仓库提供专用本地 PostgreSQL 16 测试脚本，只监听 `127.0.0.1` 独立端口，不触碰已有容器。
- 基线门禁：`go test ./...`、`go test -race ./...`、`go vet ./...`、`go mod verify`、`git diff --check`、状态 validator 和纯 Go 交叉构建。

## 验收标准

- [x] 配置合法/非法/敏感值保护测试通过。
- [x] JSON/text slog 和稳定字段测试通过。
- [x] pgxpool 创建、ping、关闭及错误不泄密测试通过。
- [x] migration 首次执行、重入、checksum 漂移、失败回滚和并发锁测试通过。
- [x] 专用 PostgreSQL 16 测试入口可重复启动、验证和清理。
- [x] 直接/传递依赖版本、checksum 和许可证有审计证据。

## 完成证据

- 2026-07-19 在专用 `postgres:16` 临时容器、`127.0.0.1:55432` 运行全量测试；显式 `db migrate` 两次执行、pool ping、迁移事务/checksum/并发锁均通过，容器随后清理。
- 新增直接依赖为 `pgx/v5 v5.10.0`（MIT）和 `google/uuid v1.6.0`（BSD-3-Clause）；pgx 新增运行时传递依赖 `pgpassfile v1.0.0`、`pgservicefile 5a60cdf6a761`、`puddle/v2 v2.2.2` 均为 MIT，`x/sync v0.17.0`、`x/text v0.29.0` 均为 BSD-3-Clause 风格 Go 许可。`go mod verify` 通过。
- Go 1.26.5 全量 tests、race、vet，以及 `CGO_ENABLED=0 -trimpath` 的 linux amd64/arm64、darwin arm64、windows amd64 构建均通过。

## 当前完整 Module Graph 与许可审计

2026-07-19 以 `go list -m all` 得到 27 行（本模块 + 26 个外部模块），逐项读取本地 module cache 的 LICENSE/COPYING；全部落在 ADR-007 的 MIT、BSD-2-Clause、BSD-3-Clause、Apache-2.0 或 ISC allowlist。下表是当前唯一事实源，替代历史 FEAT-020。

随最终 Go 二进制进入依赖闭包的 15 个外部模块：

| Module | Version | License |
|---|---:|---|
| `github.com/golang-jwt/jwt/v5` | `v5.3.1` | MIT |
| `github.com/google/jsonschema-go` | `v0.4.3` | MIT |
| `github.com/google/uuid` | `v1.6.0` | BSD-3-Clause |
| `github.com/jackc/pgpassfile` | `v1.0.0` | MIT |
| `github.com/jackc/pgservicefile` | `v0.0.0-20240606120523-5a60cdf6a761` | MIT |
| `github.com/jackc/pgx/v5` | `v5.10.0` | MIT |
| `github.com/jackc/puddle/v2` | `v2.2.2` | MIT |
| `github.com/modelcontextprotocol/go-sdk` | `v1.6.1` | Apache-2.0 |
| `github.com/segmentio/asm` | `v1.1.3` | MIT |
| `github.com/segmentio/encoding` | `v0.5.4` | MIT |
| `github.com/yosida95/uritemplate/v3` | `v3.0.2` | BSD-3-Clause |
| `golang.org/x/oauth2` | `v0.35.0` | BSD-3-Clause |
| `golang.org/x/sync` | `v0.17.0` | BSD-3-Clause |
| `golang.org/x/sys` | `v0.41.0` | BSD-3-Clause |
| `golang.org/x/text` | `v0.29.0` | BSD-3-Clause |

只存在于完整 module/test/tool graph、不进入 `go list -deps ./cmd/ai-native-platform` 生产二进制闭包的 11 个模块：

| Module | Version | License |
|---|---:|---|
| `cloud.google.com/go/compute/metadata` | `v0.3.0` | Apache-2.0 |
| `github.com/davecgh/go-spew` | `v1.1.1` | ISC |
| `github.com/google/go-cmp` | `v0.7.0` | BSD-3-Clause |
| `github.com/kr/pretty` | `v0.3.0` | MIT |
| `github.com/pmezard/go-difflib` | `v1.0.0` | BSD-3-Clause |
| `github.com/stretchr/objx` | `v0.1.0` | MIT |
| `github.com/stretchr/testify` | `v1.11.1` | MIT |
| `golang.org/x/mod` | `v0.27.0` | BSD-3-Clause |
| `golang.org/x/tools` | `v0.42.0` | BSD-3-Clause |
| `gopkg.in/check.v1` | `v1.0.0-20201130134442-10cb98267c6c` | BSD-2-Clause |
| `gopkg.in/yaml.v3` | `v3.0.1` | dual MIT/Apache-2.0（按文件） |

`go.sum` 和 `go mod verify` 覆盖 checksum；最终发布仍必须生成 SBOM、第三方 notices、hash/provenance/signature，且不得把 graph-only 判定误写成“无需许可审计”。

## 非目标

生产部署、CI、镜像、远端 module proxy、SBOM 签名、HA、备份、恢复和生产数据库迁移不在本任务范围。

第三轮独立 checker 已在 fresh PostgreSQL 16.13 上复跑全量 race、vet、module verify、状态校验及四目标 CGO-free 构建并判定 `PASS`。
