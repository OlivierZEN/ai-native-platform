---
kind: current-status
version: 3
updated_at: 2026-07-24T13:30:08Z
updated_by: release-agent after initial CodeUp publication
phase: cross-product-identity-live
active_task: "TASK-027"
next_action: "Keycloak identity and its AgentCiCi login theme are live; continue the independently authorized usage-metering task without widening production scope."
read_next:
  goals: true
  decisions: true
  issue_list: false
  task_board: true
  test_report: true
  devops: false
---

# 项目当前状态

## 快照

- 术语边界已确认并写入项目规范：后续“数据平台”始终且仅指本仓库的 CloudCC Semattice（语义格，`/Volumes/AISpace/codehouse/AI-Native-Platform`）；Agent CC / AgentCiCi 与 CloudCC CRM 均是外部应用或集成方，不得混称。规范来源为根目录 `README.md` 与 `AGENTS.md`。
- 用户指定的阿里云 CodeUp 仓库已作为独立 `codeup` remote 接入，当前项目快照首次发布到其 `main` 分支；GitHub `origin` 保持不变，本地工作分支与 upstream 未被改写。
- 用户已确认并于 2026-07-24 明确授权官方应用身份互通实施（ADR-014、FEAT-028、FEAT-029）：Keycloak 是唯一 IdP；官方应用使用短期、单租户的 OACT 在 API/MCP/CLI 间互通，登录/切换公司/续期时换发而非逐请求交换；第三方只能使用独立 Keycloak Service Account 的数据平台最小权限 Token。当前执行 TASK-029，允许在 ECS `115.29.222.70` 部署 Keycloak、配置 `sso.agentcici.com`，并在此授权范围内实施跨仓库运行时改造和生产发布。
- TASK-029 已完成基础链路发布：Keycloak 26.7.0 运行于授权 ECS 的 loopback `:8180`，Nginx 经现有 `*.agentcici.com` TLS 证书公开 `https://sso.agentcici.com`；独立 PostgreSQL `keycloak` 数据库/role、非特权 systemd、业务 Realm 和首批 client 已创建。AgentCiCi `2.8.11` 已切换至 Keycloak OIDC，24 个全局账户存在一对一外部身份映射，OACT JWKS 公开为 RS256。Semattice release `20260724T094721Z-keycloak-jwks` 已上线，固定信任 AgentCiCi OACT issuer/audience/JWKS 并保留旧 HS256 兼容；真实 RS256 技术烟测调用成功，未逐请求回调 Keycloak。
- Keycloak `agentcici` Realm 已部署并启用 `agentcici` 原生 login theme：直接复用 AgentCiCi `login_mode2` 的星空参数、6 张真实立方体面图、3D 旋转与指针倾斜结构，不再以 CSS 近似重画。账号/密码/MFA/错误处理仍完全由 Keycloak 负责。`/opt/keycloak/backups/20260724T120956Z-before-agentcici-login-theme` 保留上一版本；真实授权入口的默认表单通过桌面浏览器验收。
- 用户于 2026-07-24 选择公司 `org2sva14i4udjmi2t4s`（智能体平台演示环境）作为真实验收租户。原失败原因是 Semattice 服务器无法解析过期回调域名 `onechat.agentcici.com`；已将运行配置改为 `https://x.agentcici.com` 并重启服务。以同一幂等键安全重放受控开户后，AgentCiCi binding 与 Semattice `tenant_registry` 均为 `PROVISIONED/active`，共享 tenant UUID `93ff0c87-a626-529e-b8cf-195825df2488`。
- 最终 OACT 验收：目标公司存在两名已映射、有效成员。服务器内以其中一名成员的 Keycloak `sub` 和真实 `tenant_id + company_id` 签发一次性 120 秒 RS256 OACT；Semattice 固定 JWKS verifier 返回 HTTP 200 / `succeeded`，调用 `system.capability.list` 成功。未输出 Token、私钥、成员身份或密码，且该调用未回调 Keycloak。
- 用户于 2026-07-23 授权优先实现 `TASK-027` 的 Phase 1 用量计量：API/MCP/CLI 请求、RU、逻辑业务数据大小和有效业务记录数。计量必须低开销、租户隔离、幂等且可审计；不在本阶段启用收费、自动停服、AI/连接器计量、外部 TSDB 或 Web UI。该实现替代当前“先执行 TASK-026 远程发布动作”的近期优先级，但不扩大既有远程生产授权。
- 用户已授权跨项目生产变更：Semattice 的 `company_id` 必须是已存在的 AgentCiCi `org_id`。任意已登记服务可请求开户，但 Semattice 必须向 AgentCiCi reserve/complete，完成 HMAC、幂等、并发与失败恢复验证后才分别发布。
- 用户于 2026-07-23 确认：CloudCC Semattice 已发布在线环境，当前处于小范围内测。内测阶段不等同于 HA、备份恢复、生产 SLA 或将本地在制改动自动发布；线上制品与本地工作树的差异必须按任务和发布证据单独核对。
- 正式产品名称：**CloudCC Semattice（语义格）**；产品类别为 Agentic Business Data Runtime，中文定位为“面向智能体的业务数据与语义运行底座”（ADR-012）。现有 `native_platform`、`ai-native-platform`、`AI_NATIVE_*` 和 Go module path 暂按兼容标识保留。
- 当前技术栈：Go 1.26.5 单运行时与签名二进制交付（ADR-007）。API、MCP 与无交互 CLI 从同一 Capability Contract 投影。
- 当前数据 PoC：Docker PostgreSQL 16.13，单可用区、单 writer，8 GiB/16 GiB 内存档位；50 并发、200 活跃用户、100 万记录；不做 HA、备份或恢复演练（ADR-008）。
- 当前租户设计：既有 Agent CC 运营管理端扩展为产品无关的统一租户控制面；一个全局租户分别拥有 Agent CC `0..1` 与 Native `0..1` 产品订阅，任一产品都可单独或按任意顺序开通。只有需要绑定时，两者才必须归属同一全局租户并共享 UUIDv4 `tenant_id + 20 位 company_id`（ADR-009、FEAT-011）。
- 当前隔离原则：UUID 提供不可枚举和误命中缓冲，但不是授权边界；受信 TenantContext、强制 RLS、同租户关系约束、事务级数据库上下文和连接池残留测试必须共同生效。
- 架构评审状态：`FEAT-009` 已于 2026-07-18 由用户正式批准，`TASK-009` 已关闭，ADR-003 已接受。
- 已有基线：`TASK-020` 的 Go Capability Contract PoC 已独立验证，`system.capability.list` 从同一 Registry/Invoker 投影 API、MCP 与无交互 CLI。
- 当前实现：`TASK-010` 至 `TASK-015` 的工程基线、租户控制面、隔离、元数据、Changeset 和记录运行时任务均为 `done`。数据库配置存在时，共 28 项能力从同一 Registry 投影三入口，其中 10 项为 `metadata.changeset.*`。
- 记录运行时：published metadata 校验、兼容版本读取/惰性迁移、UUIDv7、revision 乐观锁、软删除、durable write idempotency、关系边和五类 typed indexes 已实现；100 万记录真实 bounded query 基准 p50 0.503 ms、p95 1.400 ms，物理合计 1,317,806,080 bytes。
- 动态字段安全演进：migrations 5/6、十项 Changeset 能力、字段/index 生命周期、版本化 500/20/40 配额、256 KiB 与 JSON 形状门禁、候选新写投影、有界 backfill、revision 冲突恢复、required/index/unique/reference coverage、predecessor 改名/类型转换、purge/tombstone 均已实现。只有 `active` 索引可查询，coverage 未达 100% 时发布 fail closed；`TASK-014` 已关闭。
- 授权设计：用户已批准 `FEAT-016`。业务授权采用“角色 → Permission Set → 原子权限”，不采用业务 Profile；组织树仅计算数据范围和层级记录共享，不向下继承功能权限；Owner 与稳定 `data_organization_id` 分离，调岗不自动改写历史记录；共享采用团队、例外 `share_grant`、规则定义与可选 `record × group` 投影，禁止 `record × user` ACL 物化。
- TASK-016 已开始实现：migration 7 创建主体/组织闭包/组、角色/Permission Set/原子权限/数据范围、对象策略、团队/共享/规则/投影与快照表，均为 runtime 最小授权 + FORCE RLS。记录服务对显式 `enforced` 对象执行对象、字段和记录 PDP；记录查询/单取/更新/删除统一使用 Owner、组织范围、团队、直接/组共享和 `record × group` 规则投影谓词。策略未开启对象保持旧行为，支持渐进迁移。
- TASK-016 第二增量：新增 9 个授权/共享/组织合并 Capability（总能力数 37），管理动作同时经过 JWT Scope、platform 原子权限和高风险 verified approval。手工 `share_grant` 可立即影响记录谓词；migration 8 新增无物理删除的组织合并 operation 与记录归属历史，按最多 1,000 条记录批量迁移、完成时迁移成员/固定数据范围、重建组织闭包并失效 permission snapshot。
- TASK-016 第三增量：新增 access group 创建/成员变更和记录团队成员 Capability（总能力数 40）；组成员、团队路径均被真实受保护记录用例验证，不扩展为 `record × user` ACL。
- TASK-016 第四/五/六增量：migrations 9–11 为共享规则加入 `building/ready/failed` 投影状态、游标和修订，并在记录创建/数据组织变更时维护 ready rule 的 `record × group` 投影。新增规则定义/有界投影刷新/失败重试、角色互斥、角色撤销与数据范围配置、最小授权解释及持久审计、组织合并取消 Capability；规则投影未 ready 时 fail closed。投影结束前重新检查缺边，构建期间越过游标的组织变更会重置游标而不会误标 ready；禁用 access group 会同步撤销规则/组共享和 explain 依据。组织 merge 期间源组织不再可为受保护记录提供 primary data anchor；两条源记录的多批迁移、成员转移和目标归属已有实测。对象授权策略现有受审批 Capability，累计 49 项能力。权限包写入、绑定与角色分配均验证发起人已持有目标原子权限，阻断“授权管理权限”向任意业务权限提升。
- TASK-016 已完成（本地授权范围）：授权表被纳入 FORCE RLS/非 owner/control 无权断言，缺失 TenantContext 与错误 tenant/bucket 的授权表访问均有负向测试。条件数据范围限于最多五项 scalar JSONB 等值条件，且 `access.explain` 输出 `conditional_scope`；100 万记录、10 万条 record×group 投影、50 并发的本地真实授权查询模拟已在当前谓词复验通过（p50 285.095 ms / p95 353.419 ms）。TASK-017 的通用 outbox/告警和 TASK-019 的 200 活跃用户、热点公平性或生产 SLA 均为独立后续任务。
- TASK-022 已完成：当前授权工作树已部署到 ECS `115.29.222.70`，`https://semattice.agentcici.com` 提供使用说明、CLI 下载和经鉴权的 Capability API。PostgreSQL 16.13、Nginx 1.30.2 与 Semattice systemd 服务均已启用；API、CLI、MCP stdio 实际枚举 49 项能力并完成工具调用，重启后验证通过。

- TASK-026：受控开户代码、HMAC/Nginx 路由和 Linux 制品已完成本地全量、race、vet、module 与构建验证。ECS 试发布确认 AgentCiCi reservation/completion 双向 HMAC 均可达，但生产 schema 仍停在 migration 1–12，缺少 migration 13 的 `company_id` 列导致本地 projection 返回 500；新制品、Nginx 与环境已原子回滚至上一健康 release。运行配置不保存 migrator URL，必须由专用 `semattice_migrator` 显式执行 migration 后再发布。
- TASK-023 已完成：首页新增平台能力介绍、7 域能力矩阵和全部 49 项原子 Capability 的 ID、用途、scope 与风险等级；矩阵逐项来自已部署 Runtime Registry，桌面/移动布局和展开交互已通过公网浏览器验证。
- TASK-024 已完成（仅当前工作树、本地验证）：`serve` 进程新增认证的 Streamable HTTP MCP `/mcp` endpoint。每个请求走既有 Bearer JWT 校验，session 绑定 `tenant_id + subject` 并有 Origin/DNS-rebinding 防护；MCP SDK 客户端真实调用和无 token 拒绝均已测试。授权 ECS 上当前制品尚未包含该端点，不能据此宣称已上线。
- TASK-025 已完成（仅当前工作树、本地验证）：统一运营开户的全局身份改为 `company_id`；migration 13 无损重命名 `tenant_registry` 列和约束，旧 `org_id` 请求/JWT claim fail closed。租户内 RBAC/共享 `organization_id` 组织树未改动，既有 20 位编号值也未重键。授权 ECS 仍运行 migration 1–12 的旧制品，尚未升级。
- Checker 轮次：第一轮发现 control/runtime 单 pool 与许可清单问题；第二轮发现 control 多余读取 `shard_registry`；两轮问题均已修复。第三轮从 fresh PostgreSQL 16.13 空库执行 migrations 1/2/3 后明确 `PASS`。

## 当前 L2 授权边界

- 允许：本仓库 Go 源码、规格、迁移、测试和本地工具；本机专用 Docker PostgreSQL 16.13 PoC 的 schema/migration、128 bucket、RLS、TenantContext、连接池、路由和容量验证。
- 允许：Native 租户控制面、产品生命周期、配额/路由投影、Capability API/MCP/CLI、幂等与审计；统一运营控制面使用版本化 adapter/port 接入。
- 允许：共享身份或企业 IdP 的当前仓库集成边界，包括令牌验证、audience/issuer/scope 校验、全局主体与最小成员投影、TenantContext 解析及负向测试；不复制密码或长期凭据。
- 2026-07-23 扩展授权：允许仅在 ECS `115.29.222.70` 安装 PostgreSQL/Nginx、创建本任务专用账号与目录、部署当前工作树制品、配置 `semattice.agentcici.com` 与用户提供的 TLS 证书，并执行无真实租户数据的 smoke 验证。
- 仍禁止：访问其他生产/共享远程数据库、生产身份系统或真实租户数据；HA/备份恢复演练、远端 Git push/PR/merge，以及向 Agent CC、运营端或其他服务器写入代码/配置。扩大这些边界需要再次明确授权。

## 下一步

1. 维持 TASK-022 的单节点验证部署，证书到期前安排续证；HA、自动备份恢复和生产 SLA 尚未获准或实现。
2. 用户选择后，由 `TASK-017` 实现通用 transactional outbox/worker，或由 `TASK-018` 实现 Agent Tool Gateway PoC。
3. `TASK-019` 再执行 8/16 GiB、50 并发、200 活跃用户和热点公平性的完整容量验收；FEAT-016 的单租户百万记录结果不替代它。

## 证据与归档

- 已完成 PoC 计划：`docs/superpowers/plans/2026-07-18-five-hour-go-l2-loop.md`；它不覆盖本次扩大授权，新阶段计划将在 `TASK-010` 启动时创建。
- 统一租户设计：`docs/specs/FEAT-011-unified-tenant-operations-control-plane.md`。
- 已完成 L1 的状态、日志、预算、约束、计划和测试证据：`docs/archive/loop-engineering/` 与 `docs/archive/plans/`。
- 已替代的运行时 ADR：`docs/archive/decisions/ADR-005-superseded-runtime-proposal.md`。

## 已验证事实

- 2026-07-24 CodeUp 首次发布：空仓库认证和 `HEAD -> main` 推送成功；发布前全量 test、vet、module verify、状态 validator、差异、常见密钥、受跟踪敏感扩展名和 10 MiB 大文件门禁均通过。
- 2026-07-23 产品命名治理：用户正式确认 CloudCC Semattice（语义格）；ADR-012、README、goals、FEAT-009/011/020 和兼容命名边界已更新。状态 validator、`git diff --check`、品牌存在性与旧标题冲突检索均通过；本次未修改或重新验证运行时代码。
- 2026-07-23 TASK-022：授权 ECS 的 PostgreSQL 16.13、12 个 migration、三数据库身份、Nginx TLS、Semattice systemd、首页与下载已部署。公网 HTTPS 200、HTTP 301、未授权 API 401、短期 JWT API 200、CLI/MCP 三入口 49 能力、MCP 工具调用、制品 checksum、服务重启和 secret mode 均通过。
- 2026-07-23 TASK-023：首页 7 个能力域按 `6 + 6 + 10 + 5 + 12 + 9 + 1 = 49` 覆盖运行时契约；49 个 ID、required scope 和风险等级逐项比对通过，公网桌面/移动浏览器、全部展开交互和零控制台告警验证通过。
- 2026-07-23 TASK-025：本机 fresh PostgreSQL 16 按 checksum 执行 migrations 1–13；`tenant_registry.company_id` 存在且旧列不存在。全量单元/集成、race、vet、模块校验、状态 validator 和差异检查通过；旧 `org_id` JWT/input 被拒绝，tenant Capability v2 schema 仅暴露 `company_id`。未修改远程系统或部署。
- 2026-07-19 第三轮独立 checker：`passed`。Fresh PostgreSQL 16.13 migration 1/2/3、128 分区、control 精确三表授权、runtime TenantContext fail-closed、实际 main 双角色接线、租户开户、元数据、API/MCP/CLI parity、全量 race、vet、module verify、状态 validator、差异检查和四目标 CGO-free 构建均通过。
- 2026-07-19 TASK-015 maker 验证：fresh migration 1/2/3/4、记录 CRUD/query、draft metadata 拒绝、兼容版本读取与 update 惰性迁移、复合版本 FK、640 typed partitions、FORCE RLS、durable idempotency、关系与跨租户隔离、五能力三入口成功/错误 parity、全量/race/vet/module verify 和四目标 CGO-free 构建通过；百万记录物理结果见 FEAT-015 与 `test-report.md`。本次未声明新的独立 checker 结论。
- 2026-07-20 TASK-014 核心 maker 验证：fresh PostgreSQL 16 执行 migrations 1–5；全量 `go test` 与 `go test -race` 通过。非破坏性 Changeset 完整状态流、API/MCP/CLI parity、独立审批、基线漂移/直接发布阻断、受限回滚、FORCE RLS、record route publication fence、字段可见/可写状态和 JSONB 硬边界均有测试；有存量记录的新索引保持 `building` 且 publish fail closed。本轮未声明独立 checker 结论。
- 2026-07-20 TASK-014 最终 maker 验证：PostgreSQL 16 migrations 1–6、全量 test/race、vet、module verify 和四目标 CGO-free build 通过；真实锁竞争验证 revision conflict 跳过与续跑，另覆盖 unique 冲突修复、candidate 双写、relation coverage、predecessor 类型转换、optional `on_create` 不重写历史、purge/tombstone、破坏性回滚拒绝和新增 Changeset 能力三入口 parity。本轮未声明独立 checker 结论。
- 当前依赖事实：`go list -m all` 为项目本身加 26 个外部模块；其中 15 个进入生产二进制闭包、11 个只在完整 graph/test/tool graph，许可已记录于 FEAT-010。生产发布仍需 SBOM、notices、provenance 和签名。
- 2026-07-23 TASK-016 首个实现增量：fresh PostgreSQL 16 成功执行 migrations 1–7；`TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope` 覆盖无角色拒绝、字段最小权限、组织下级数据范围、以及调岗后不改写 `data_organization_id` 且 Owner 保留访问。`./scripts/test-postgres.sh run` 全量通过。该轮是 maker 自验，TASK-016 仍在进行，未声明独立 checker 结论。
- 2026-07-23 TASK-016 第二实现增量：fresh PostgreSQL 16 成功执行 migrations 1–8；同一真实数据库用例覆盖角色/权限集管理、verified approval、直接共享即时生效、组织合并启动/执行/完成。全量 `./scripts/test-postgres.sh run` 通过；TASK-016 仍在进行，未声明独立 checker 结论。
- 2026-07-23 TASK-016 第四实现增量：fresh PostgreSQL 16 成功执行 migrations 1–9；共享规则以 `record × group` 有界投影刷新，只有 `ready` 规则参与记录谓词。集成用例覆盖规则、组、团队、解释审计、职责分离、组织调岗和可取消/完成的合并；对象策略发布走 Scope + platform 权限 + verified approval。授权表 RLS 的无上下文、错租户写入与 control 无权断言加入数据库测试。全量 `go test -race ./...`、`go vet ./...`、`go mod verify`、`git diff --check` 和四目标无 CGO 构建通过；TASK-016 仍在进行，未声明独立 checker 或容量结论。
- 本次统一租户及独立产品订阅文档治理校验：`passed`（2026-07-18T15:29:56Z）；状态 validator、已跟踪文件 `git diff --check` 与现行文档矛盾措辞检索均通过。本次只修改文档和项目状态，不修改或重新验证应用源码。此前独立验证仍已通过 Go 全量测试、race、vet、模块校验、四目标 `CGO_ENABLED=0` 构建和 denylist 审查。
- 本次 `FEAT-009` 批准与 `TASK-009` 关闭治理校验：`passed`（2026-07-18T15:43:29Z）；状态 validator、`git diff --check` 与状态冲突检索均通过。本次不修改或重新验证应用源码。
- 本次 L2 授权扩展治理校验：`passed`（2026-07-18T15:46:48Z）；状态 validator、`git diff --check` 与现行授权冲突检索均通过。此次只记录授权边界，尚未开始新阶段源码或数据库操作。
- 本次远端发布前校验：`passed`（2026-07-18T15:51:53Z）；Go 1.26.5 全量测试、race、vet、模块校验、四目标纯 Go 构建、状态 validator、差异检查、敏感凭据模式和大文件检查均通过。
- 历史验证的完整输出与运行记录在归档中；它们不是当前 Go 工具链的实现证据。
