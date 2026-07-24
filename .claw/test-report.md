---
kind: test-report
version: 3
updated_at: 2026-07-24T08:48:00Z
updated_by: integration-agent after TASK-029 Keycloak infrastructure verification
last_run_at: 2026-07-24T08:48:00Z
last_run_status: passed
---

# 测试报告

## 当前运行摘要

- 状态：`passed`。覆盖 `TASK-010`、`TASK-011`、`TASK-012`、`TASK-013`、`TASK-015`、`TASK-016`、`TASK-024`、`TASK-025` 的当前本地实现，以及 `TASK-022` 的授权 ECS 部署验收。
- 数据库：Docker PostgreSQL `16.13 (Debian 16.13-1.pgdg13+1)`；镜像 digest `postgres@sha256:5d143123fdf80462d1778cd4f24b9f7ca13c87174bca19141fb194c5a1ebca59`。仅绑定 `127.0.0.1:55432` 的专用临时容器，结束后清理。
- 本轮 maker 从 fresh schema 执行 migrations 1–13；`schema_migration` checksum 可重复，128 个 object partitions 与五类共 640 个 typed-index partitions 完整。TASK-010–013 的第三轮独立 checker 历史结论继续有效；TASK-015 尚未声明新的独立 checker 结论。

## 2026-07-24 TASK-029 Keycloak 基础设施验证

## 2026-07-24 TASK-029 应用认证链路发布验证

- AgentCiCi 后端 V96 已上线；24 个有效全局账户以 `issuer=https://sso.agentcici.com/realms/agentcici + sub` 映射到 `account_external_identity`，密码哈希迁移不读取或输出明文。`agentcici-bff` 使用 Authorization Code + PKCE；入口 `/auth/oidc/login` 返回 302 到 Keycloak 且设置 `CICI_OIDC_STATE` 安全 Cookie。
- AgentCiCi `2.8.11` 已启用 10 分钟 RS256 OACT 签名，公开 JWKS 返回 200、一枚 `RSA/sig/RS256` 公钥；签名私钥和 Keycloak client secret 均只在 root 受限配置中保存。
- Semattice release `20260724T094721Z-keycloak-jwks` 已运行。全量 `GOTOOLCHAIN=go1.26.5 go test ./...`、定向配置/身份/main 测试和 Linux amd64 CGO-free 构建通过。其固定 JWKS verifier 对无 Token 和非受信 issuer 均返回 HTTP 401；使用一次性、无业务数据技术烟测 OACT 调用 `system.capability.list` 返回 HTTP 200 / `succeeded`。验证仅发生本地 JWKS 验签，不调用 Keycloak。
- 初始发布阶段没有 `PROVISIONED` binding，因此技术烟测不作为真实业务租户验收；随后按本报告下一条完成了用户选择公司的受控开户恢复与两端绑定确认。
- 后续真实公司验收：用户选择 `org2sva14i4udjmi2t4s`。定位到 Semattice 的 AgentCiCi 回调域名 `onechat.agentcici.com` 无法在 ECS 解析，导致 reservation 返回 `FAILED_PRECONDITION`；改为 `https://x.agentcici.com` 后，以既有幂等键安全重试，签名 `POST /internal/v1/company-provisionings` 返回 HTTP 200 / `active`。AgentCiCi binding 与 Semattice `tenant_registry` 均确认 tenant `93ff0c87-a626-529e-b8cf-195825df2488`、公司 `org2sva14i4udjmi2t4s`、状态 `PROVISIONED/active`。
- 最终真实主体 OACT 验收：该公司有两名有效成员已映射 Keycloak `sub`。在不输出成员身份、密码、Token 或私钥的前提下，以其中一名真实 `sub`、该 tenant UUID、公司 ID、`aud=semattice-api` 和 `scope=system.capability.read` 构造 120 秒 RS256 OACT。公网 Semattice API 返回 HTTP 200，`system.capability.list` 状态为 `succeeded`；无 Token/非受信 issuer 的对照请求仍返回 401。该验证证明真实公司、真实已映射主体、AgentCiCi JWKS 和 Semattice 本地验签绑定一致，且没有逐请求 IdP 回调。

- ECS `115.29.222.70` 已安装 Keycloak `26.7.0`（Amazon Corretto 21、PostgreSQL 16 独立 `keycloak` 数据库/role、非特权 `keycloak` systemd、loopback `8180/9000`）并通过 `systemctl` 重启后启动。
- `https://sso.agentcici.com/realms/agentcici/.well-known/openid-configuration` 与 JWKS cert endpoint 均 HTTPS 200，证书校验成功；管理控制台重定向正常。
- 本机 `/health/live` 与 `/health/ready` 均为 `UP`；公网 `/health`、`/metrics` 均为 404；Nginx `nginx -t` 通过。
- `agentcici` Realm 与 `agentcici-bff`、`semattice-api`、`official-access-context`、`followup-worker` client 注册已创建；未读取/输出 client secret。
- 随机、自动删除的账号和直接授权测试客户端验证：AgentCiCi `PBKDF2WithHmacSHA256` / 120,000 轮 / 256-bit hash，可作为 Keycloak `pbkdf2-sha256` credential 导入并成功换取 Token。导入时 Keycloak salt 使用 AgentCiCi 存储 salt 字符串 UTF-8 字节的 base64，不是二次解码该字符串。无真实账号、密码或 credential 被读取。
- 回归保护：`https://semattice.agentcici.com/` 仍 HTTPS 200。此节只证明 IdP 基础设施，不证明 AgentCiCi/Semattice 运行时代码已完成 OACT/JWKS 切流。

## 2026-07-23 TASK-025 company_id 全局身份改名验证

- fresh Docker PostgreSQL 16 按 checksum 执行 migrations 1–13；migration 13 将 `tenant_registry.org_id` 和关联约束无损改名为 `company_id`。集成断言确认新列存在、旧列不存在，并保留既有编号值与唯一性。
- JWT 只接受 `company_id`；旧 `org_id` claim、旧 tenant capability input 均被拒绝。六项 tenant capability descriptor 升至 v2，输入/输出 schema 仅暴露 `company_id`。
- `organization_id` 授权组织树、组织合并和数据范围 PostgreSQL 回归随全量测试执行，未发生字段改名。
- `GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`./scripts/test-postgres.sh run`、`GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、本机临时路径 `go build -trimpath`、状态 validator 与 `git diff --check`：passed。
- 本轮仅验证当前工作树；未更新 ECS、远程 PostgreSQL、Nginx、外部运营控制面或 JWT 发行方。

## 2026-07-23 TASK-024 Streamable HTTP MCP 验证

- `mcp.StreamableClientTransport` 经 HTTP Bearer transport 连接新 `/mcp` handler，完成初始化并成功调用 `system_capability_list`；调用审计的 tenant 与 actor 均来自已验证 JWT principal。
- 未携带 Bearer token 的规范 MCP POST 返回 HTTP 401。
- `GOTOOLCHAIN=go1.26.5 go test ./... -count=1`、`GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、`GOTOOLCHAIN=go1.26.5 go build -trimpath ./cmd/ai-native-platform` 和 `git diff --check`：passed。
- 本轮仅验证当前工作树；未更新授权 ECS 制品、Nginx 配置或公网 endpoint。

## 2026-07-23 TASK-022 部署验收

- 发布前：`GOTOOLCHAIN=go1.26.5 go test ./... -count=1`、`go vet ./...`、HTML parse、状态 validator 与 `git diff --check` 均通过。
- Linux amd64 CGO-free 二进制 SHA-256 为 `24fa672c9399e2f60cce4412ed754654141594e76e82b2d253bf309c93b3db59`；公网下载与 `.sha256` 声明完全一致。
- ECS 运行 PostgreSQL 16.13、Nginx 1.30.2；12 个内置 migration 一次事务完成，809 张父表/分区表启用并强制 RLS。migrator 临时 CREATEROLE 已撤销，control/runtime/migrator 均为非 superuser、非 CREATEROLE、非 BYPASSRLS。
- 监听边界：PostgreSQL `127.0.0.1:5432`，Semattice `127.0.0.1:8080`，Nginx 公网 80/443。
- 公网验证：HTTPS 首页 200；HTTP 301；edge health 200；无令牌 Capability API 401；证书 SAN 覆盖 `*.agentcici.com`，有效期至 2026-11-24。
- 五分钟短期 smoke JWT：公网 API `system.capability.list` 返回 `succeeded` 和 49 项能力；服务器 CLI 返回同样 49 项；MCP stdio 以协议 `2025-11-25` 初始化、枚举 49 个工具并真实调用 `system_capability_list` 成功。
- `semattice.service` 重启后 PostgreSQL、Semattice、Nginx 均保持 active/enabled，首页与 health 再次通过；env mode `0640`、TLS 私钥 mode `0600`。
- 浏览器结构检查：标题、首屏主标题、API/CLI/MCP/安全导航均正确，控制台没有 error/warn。浏览器截图接口超时，因此不把截图声明为验收证据。
- 本轮为 maker 部署验收；未使用真实租户数据，未声明 HA、备份恢复、自动续证、容量或生产 SLA。

## 2026-07-23 TASK-023 能力矩阵验收

- 从已部署、经短期 smoke JWT 鉴权的 `system.capability.list` 取得 49 项 Capability Descriptor，页面按七域分组为 `6 + 6 + 10 + 5 + 12 + 9 + 1 = 49`。
- HTML parser 对本地页面和公网页面均验证 7 个 domain、49 个唯一 capability item；页面 ID 集合与 Runtime Registry 集合完全相等。
- 进一步逐项验证 49 个 `required_scope` 与 49 个 `risk_level`，全部和运行时 descriptor 一致。
- `node --check deploy/semattice/www/app.js`、CSS brace、HTML parse、`git diff --check` 均通过；Nginx 配置复验通过。
- 公网首页、CSS、JS 均返回 200；能力介绍、矩阵标题和四个能力支柱的可见文本断言通过，edge health 保持正常。
- 浏览器桌面 1440×900：四个支柱等宽四列，矩阵 7 域/49 项，默认展开 1 域；“展开全部”后 7 域全部打开且 `aria-expanded=true`。
- 浏览器移动 390×844：支柱单列、矩阵摘要和条目按移动布局收敛；body 禁止横向滚动。桌面和移动检查均无 console error/warn。
- 服务器发布前保留上一版静态文件于 `/var/www/semattice-backups/20260723T0658Z`；本次只更新静态 HTML/CSS/JS，未重启或修改应用、数据库和 TLS。

## 通过的门禁

- `TEST_DATABASE_URL=... go test ./... -count=1`
- `TEST_DATABASE_URL=... go test -race ./... -count=1`
- `go vet ./...`
- `go mod verify`：`all modules verified`
- `python3 /Users/owenmacbook/.agents/skills/y-agentic-project-guidelines/scripts/validate-state.py .claw`
- `git diff --check`
- `CGO_ENABLED=0`、`-trimpath`：`linux/amd64`、`linux/arm64`、`darwin/arm64`、`windows/amd64` 四目标构建

## 数据库与安全证据

- `ai_native_runtime` 和 `ai_native_control` 均非 owner、非 superuser、非 BYPASSRLS。
- control 仅对 `tenant_registry`、`tenant_operation`、`audit_event` 三表拥有按操作精确授权及三条 role policy；对 `shard_registry`、业务记录和元数据表均无权限。
- runtime 缺失 TenantContext 时对 registry、records 和 metadata 均读取 0；错误 tenant/bucket、跨租户写入与跨租户关系全部拒绝。
- `MaxConns=1` 的 commit/error/panic 路径均不残留 tenant/bucket/actor；分区裁剪只命中目标 bucket。
- 实际 `main` 双角色接线以 control 完成 `tenant.provision`，再以 runtime TenantContext + control router 创建元数据版本。
- typed-index parent/child tables 与 `record_operation` 均 FORCE RLS；control 无记录面权限，runtime 无 TenantContext 读取 0。typed rows 和 relation source 以复合 FK 绑定 tenant、metadata version、object 和 record。

## 能力与模型证据

- 一个 system、六个租户、十三个元数据/Changeset 和五个记录能力均从同一 Capability Registry/Invoker 投影到 authenticated API、authenticated MCP 和无交互 agent CLI；记录五能力和 Changeset get-status 的成功/错误 parity 通过。
- JWT issuer/audience/algorithm/key/expiry、tenant/org/subject/scope 负向用例 fail closed。
- 元数据 UUIDv7、同 tenant/version 复合约束、draft-only 变更、独立批准发布、发布后不可变及确定性 snapshot/SHA-256 digest 测试通过。
- 记录 create/get/update/delete/query、published-only 对象、兼容 metadata version 读取/查询与 update 惰性迁移、未知字段、类型/default/required、UUIDv7、revision 冲突、软删除、游标、未索引 filter 拒绝、关系 target/restrict、跨租户和连接池复用测试通过。

## TASK-015 百万记录物理基准

- 数据集：1,000,000 条 active records，每条一个 indexed text 和一个 indexed number；2,000,000 typed rows，写放大精确为 2 typed rows/record。
- 写入：object set insert 6.493 s；typed-index set insert 61.228 s。
- 尺寸：object 580,608,000 bytes；text tree 416,006,144 bytes；number tree 321,191,936 bytes；合计 1,317,806,080 bytes，平均 1,317.81 bytes/record。
- 查询：真实 `buildQuery` 的 200 次等值/范围样本 p50 0.503 ms、p95 1.400 ms；计划命中 `record_index_number_b007_value_idx`，无 `object_record_b007` full scan。
- 此结果只验证当前本机 PostgreSQL 物理路径，不是生产 SLA；8/16 GiB、50 并发、200 活跃用户与热点公平性仍属于 TASK-019。

## 依赖与范围

- 完整 module graph 为项目本身加 26 个外部模块；15 个进入生产二进制闭包，11 个仅属于完整 graph/test/tool graph。精确版本和许可记录在 `FEAT-010`；`FEAT-020` 的旧依赖快照已标记 superseded。
- 第一轮 checker 的 control/runtime 单 pool、metadata 预路由和许可图问题，以及第二轮 checker 的 control 多余 `shard_registry SELECT` 均已修复并有回归测试。
- 未执行生产/共享数据库访问、远端 Git 写入、CI、制品发布、部署、HA、备份或恢复。

## 2026-07-19 动态字段设计治理增量

- 新增 FEAT-014，更新 FEAT-009、FEAT-015、ADR-010、task board 和 current status；本次只有文档与项目状态变更，没有修改或重新验证 Go/数据库源码。
- `python3 /Users/owenmacbook/.agents/skills/y-agentic-project-guidelines/scripts/validate-state.py .claw`：passed。
- `git diff --check`：passed。
- PostgreSQL 官方 JSONB/TOAST/limits 文档与 TASK-015 百万记录尺寸重新核对；`737,198,080 / 2,000,000 = 368.60` bytes/record/indexed-field，20/40/50 字段线性估算为 7.37/14.74/18.43 GB。该估算不替代 TASK-019 的真实并发容量测试。

L1 历史审计结果归档在 `docs/archive/loop-engineering/2026-07-16-l1-test-report.md`，不作为本轮 Go/PostgreSQL 实现证据。

## 2026-07-24 TASK-026 本机真实 AgentCiCi 受控开户验收

- Semattice 使用项目内置的独立 `postgres:16` 容器（`127.0.0.1:55432`）完成 schema migration，并以分离的 `ai_native_control` / `ai_native_runtime` 登录角色启动；AgentCiCi 使用本机既有 `agentcici` PostgreSQL、Redis、RabbitMQ 和 Qdrant。
- 真实 AgentCiCi 运营平台认证成功后，创建组织 `orgc9h2xs5puanlbykmc`，随后调用 AgentCiCi 的 `POST /platform/tenants/{orgId}/semattice-provisionings`。这是 AgentCiCi 对 Semattice 的真实 HMAC 请求；Semattice 再以独立 HMAC 调用 AgentCiCi 的 reservation 与 completion 回调。
- 成功响应的 `company_id` 与新建 `org_id` 完全一致；Semattice tenant UUID 为 `22369429-94c0-5dc2-ad04-600673f62829`，产品状态为 `active`。AgentCiCi binding 为 `PROVISIONED`，其回写 tenant UUID 与 Semattice `tenant_registry` 一致。
- 使用同一 idempotency key 重放 AgentCiCi 开通请求后，返回相同 tenant UUID；Semattice `tenant_operation` 对该 tenant 只有 1 条 `succeeded` 记录。未创建重复开户。
- 浏览器端到端补充：在 AgentCiCi 运营端以真实平台账号登录，打开新组织 `orgnuctqa4lpdn9zz1qx` 的应用卡片并点击“开通 Semattice”。页面收到真实成功响应后，卡片变为“运行中”、应用数从 1 到 2、按钮变为禁用的“已开通”。
- 本轮只访问本机服务和本机专用数据库；临时 HMAC、数据库角色口令和平台 token 均未写入仓库或本报告。

## 2026-07-20 TASK-014 核心实现增量

- fresh `postgres:16` 专用容器执行 migrations 1–5；`metadata_changeset`、`metadata_changeset_object`、字段 lifecycle/index/default columns、128 bucket 和既有 640 typed-index partitions 一并验证。
- `./scripts/test-postgres.sh run`：全量通过。
- `TEST_DATABASE_URL=... GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`：全量通过。
- Changeset 非破坏性 `validate -> simulate/get-status -> approve -> publish -> rollback` 通过；API、MCP、CLI get-status 结果等价，直接后续 version publish 被拒绝，rollback 需要受信审批。
- 有存量记录时新增 indexed + `backfill_required` 字段被判定 high risk，候选索引固定为 `building`，coverage 未 ready 前 publish 返回稳定 `FAILED_PRECONDITION`，tenant metadata pointer 保持旧版本。
- 记录层新增 lifecycle 写入/可见性门禁、只允许 active typed index 过滤、256 KiB 总 JSON、64 KiB 单 JSON 字段、8 层和 1,000 数组元素硬限制；record write route lock 与 Changeset activation lock 消除旧 metadata route 写回窗口。
- 本节记录第一阶段 maker 自验；当时未完成的 backfill/coverage、purge/tombstone 和版本化配额已由下方最终实现验证补齐。`TASK-019` 容量认证仍未完成。

## 2026-07-20 TASK-014 最终实现验证

- migration 6 新增 `governance_policy`、`record_unique_value`、`field_tombstone`、字段 unique contract 和 Changeset checkpoint/progress；fresh PostgreSQL 16 可按 checksum 顺序执行 migrations 1–6，唯一值与墓碑表均启用并强制 RLS。
- 有界 backfill 真实覆盖按对象暂停/恢复、savepoint 错误隔离和 checkpoint。测试使用一个未提交的并发 record update 制造 PostgreSQL 行锁竞争；worker 读取旧 revision 后条件更新影响 0 行，当前批返回 `conflict_records=1` 且不覆盖用户值，下一批读取最新 revision 后成功。
- predecessor 测试证明原 `field_id` 改类型被拒绝，而新字段通过 `predecessor_field_id` 将文本 `42.50` 转换为 number、重建 typed index 并在 coverage ready 后激活。
- 两条重复 `code` 记录启用 unique 时，一条成功、一条因唯一冲突留在旧版本，coverage/publish 继续关闭；修复冲突后重跑完成并进入 ready。
- candidate 写入测试覆盖回填期间的新 customer/contact 记录投影、候选 required/default typed index 和 relation edge 双写；历史 contact 回填后 relation coverage 精确为 2。
- optional `on_create` 默认值测试确认旧记录的物理 metadata version 与 JSON key 均未重写，新记录才获得默认值。
- purge 测试覆盖 `deprecated_read_write -> deprecated_read_only -> hidden -> purging -> tombstone`、普通 backfill 对破坏性变更拒绝、独立 purge approval、JSON/typed/unique/relation 清理、tombstone 名称保留和破坏性回滚拒绝。
- 版本化 standard policy 在 Changeset validate 时冻结为 `policy_version=1`、500 字段/20 index；dedicated-16g 为 500/40。超过 256 KiB 的 record create 在 API/MCP/CLI 返回同一 `VALIDATION_FAILED`。
- `TEST_DATABASE_URL=postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable GOTOOLCHAIN=go1.26.5 go test ./... -count=1`：passed。
- `TEST_DATABASE_URL=postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`：passed。
- `GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、darwin/linux × arm64/amd64 四目标 `CGO_ENABLED=0 go build`：passed。
- 本轮为 maker 最终自验，未声明独立 checker；`TASK-019` 的 8/16 GiB、50 并发与热点公平性容量认证仍保持独立 `todo`。

## 2026-07-22 TASK-016 授权设计治理验证

- 新增用户确认的 `FEAT-016` 与 ADR-011；更新 TASK-016 为 `ready`，本次未修改 Go、数据库 migration 或运行时行为。
- `git diff --check`：passed。
- 交叉引用已核对：TASK-016 指向 FEAT-016；FEAT-016 指向 ADR-011；`current-status.md` 的 active task 和下一步与任务看板一致。
- 项目根目录缺少约定的 `scripts/validate-state.py`；改用已安装技能提供的 `/Users/owenmacbook/.agents/skills/y-agentic-project-guidelines/scripts/validate-state.py .claw`，结果：passed。

## 2026-07-23 TASK-016 首个实现增量

- fresh Docker PostgreSQL 16 成功执行 migrations 1–7；migration 7 新增角色中心授权、组织树/闭包、主体成员关系、数据范围、对象策略、团队/例外共享/规则定义/`record × group` 投影和 permission snapshot 表。新增租户表均启用 FORCE RLS，仅授予 runtime role。
- `TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope` 在真实数据库覆盖：无角色用户对象访问拒绝；字段写入默认拒绝；未授予字段不会回显；`organization_descendants` 可读取范围内记录；负责人调岗只变更 membership，不改写记录 `data_organization_id`，且 Owner 持续访问。
- `./scripts/test-postgres.sh run`：全量通过（migrations 1–7、全部 Go packages）。本轮未执行 race/vet/发布构建；未访问生产/共享数据库，也未进行远端 Git/CI/部署操作。
- TASK-016 尚未完成：平台管理授权 Capability、共享规则计算/投影刷新与失效、组织合并编排、三入口与跨租户负向覆盖仍待实现。

## 2026-07-23 TASK-016 第二实现增量

- migration 8 新增 `organization_merge_operation` 与 `record_organization_history`，组织不物理删除；merge 以 `running -> completed` 操作保存进度，记录数据归属按最多 1,000 条批次迁移，完成时迁移成员和固定组织范围、重建 closure 并失效 snapshot。
- 新增 9 个 Capability：角色/Permission Set 创建、授权、绑定和分配，直接共享授予/撤销，组织合并启动/执行。管理调用同时需 Capability Scope、角色中的 platform 原子权限；高风险动作还需受信 principal 携带的 approval。
- `TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope` 扩展验证角色/Permission Set 写路径、approval、直接共享即时赋予记录读取、组织合并完成。`./scripts/test-postgres.sh run`：migrations 1–8 与全量 Go packages 通过。
- 尚未实现：组/团队管理、共享规则定义/刷新、`record × group` 投影生成、access explain、角色冲突管理 Capability，以及三入口和跨租户的专门负向测试。

## 2026-07-23 CloudCC Semattice 命名治理验证

- 用户正式确认产品名称 **CloudCC Semattice（语义格）**，产品类别为 Agentic Business Data Runtime；ADR-012 是品牌、CLI/服务前缀和兼容标识边界的唯一事实源。
- `python3 /Users/owenmacbook/.agents/skills/y-agentic-project-guidelines/scripts/validate-state.py .claw`：passed。
- `git diff --check`：passed。
- README 正式标题、ADR-012、goals 与 FEAT-009/011/020 品牌引用存在性检查通过；未发现仍把 `AI Native Platform` 声明为正式标题或正式产品名称的冲突文本。
- 本次只修改命名与项目治理文档；未改动、运行或重新验证 Go/PostgreSQL 运行时代码。现有 `native_platform`、`ai-native-platform`、`AI_NATIVE_*` 和 Go module path 继续作为兼容标识。

## 2026-07-23 TASK-016 第四实现与发布门禁

- fresh PostgreSQL 16 按 checksum 执行 migrations 1–9；migration 9 记录共享规则的投影状态、游标、修订和错误字段。`record × group` 投影按最大 1,000 条批次刷新，只有 `active + ready` 规则可授权，构建中不暴露记录。
- 集成数据库用例覆盖对象策略受审批发布、组成员、团队、规则投影、直接共享、授权 explain 审计、职责分离、调岗 Owner 连续性及组织合并取消/完成。权限包新增未由操作者持有的原子权限返回 `UNAUTHORIZED`。
- 授权表完整纳入 FORCE RLS、非 owner 和 control 无权断言；runtime 缺失 TenantContext 读取授权角色为 0，错误 tenant/bucket 写入授权角色被拒绝。
- `TEST_DATABASE_URL=postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`：passed。
- `GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、`git diff --check`、`CGO_ENABLED=0 -trimpath` 的 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 构建：passed。
- 本轮是 maker 自验，未执行生产/共享数据库、远端 Git 写入、CI、制品发布或部署；百万记录、50 并发和热点组织规则投影容量结论仍属于 TASK-019。

## 2026-07-23 TASK-016 生命周期与投影可靠性复验

- migrations 10/11 分别加入无组织锚点数据范围的幂等约束，以及新建/组织迁移记录的 ready-rule `record × group` 投影触发器；测试验证新记录即时获得规则投影。
- 角色撤销、snapshot 失效、受审批数据范围配置及失败规则重试均有 Capability 与数据库集成覆盖；失败规则重试清理旧投影并回到 `building`，未 refresh 到 `ready` 前不产生授权。
- `TEST_DATABASE_URL=postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、状态校验、`git diff --check` 与 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 `CGO_ENABLED=0 -trimpath` 构建：passed。

## 2026-07-23 百万记录基准复跑尝试

- opt-in `TestRecordPhysicalBenchmark` 两次运行均在查询门禁处返回 `FAILED_PRECONDITION: filter field has no active typed index: name`。当前历史基准直接发布 metadata，未按现行 Changeset 流程完成 indexed field 的 active 生命周期；这证明门禁仍生效，不构成性能结果。
- 未保留任何绕过索引生命周期的测试改动。TASK-019 必须先将基准模型改为完整 Changeset 激活路径，再记录百万记录与热点组织的真实性能结果。

## 2026-07-23 TASK-016 本地百万记录授权模拟

- 专用本机 PostgreSQL 16 容器执行：`TEST_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:55432/postgres?sslmode=disable AI_NATIVE_RUN_AUTHORIZATION_SIMULATION=1 AI_NATIVE_AUTHORIZATION_SIMULATION_RECORDS=1000000 AI_NATIVE_AUTHORIZATION_SIMULATION_CONCURRENCY=50 GOTOOLCHAIN=go1.26.5 go test ./internal/record -run TestAuthorizationLocalSimulation -count=1 -v`。
- 当前谓词复验结果：1,000,000 条记录，规则投影只生成 100,000 条 `record × group` 边；组织闭包数据范围与仅组共享的两个边界断言均通过，未生成 `record × user` ACL。批量写入耗时 19,849.874 ms；50 个运行时租户连接并发读取的 p50 为 297.759 ms，p95 为 355.822 ms。
- 同轮 `GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、状态校验与 `git diff --check` 均通过。仅使用本机专用数据库，未访问生产/共享数据库、未执行远端写入或部署。
- 该结果是 FEAT-016 的本地回归与容量证据，不替代 TASK-019 对 200 活跃用户、热点组织公平性及 8/16 GiB 档位的完整验收，也不构成生产 SLA。

## 2026-07-23 TASK-016 投影一致性与组生命周期收口

- `TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope` 新增真实数据库回归：先以一条为批次建立投影，再让已越过游标的记录离开并回到规则组织；末批刷新必须返回 `building` 并重置游标，下一轮完整 catch-up 后才可转为 `ready`。因此 building 期间的并发变更不会造成“ready 但缺 share edge”。
- 同一用例验证 group rule 对一个无组织范围的主体实际授权、`access_group.lifecycle_state=disabled` 立即拒绝该路径、恢复 active 后重新可读、结束 membership 后再次拒绝。记录 PDP 与 access explanation 的组共享查询均要求 active group。
- `TEST_DATABASE_URL=... GOTOOLCHAIN=go1.26.5 go test ./internal/record -run TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope -count=1 -v`：passed；100,000 记录/10,000 投影/50 并发的本地授权模拟复验亦 passed（p50 51.396 ms、p95 65.718 ms）。
- 当前代码完整门禁：`TEST_DATABASE_URL=... GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 `CGO_ENABLED=0 -trimpath go build`、状态校验与 `git diff --check`：全部 passed。

## 2026-07-23 TASK-016 条件数据范围

- `authorization.role.set-data-scope` 的 `conditional` 只接受 `condition.equals`：1–5 个合法数据字段，各值仅可为 string/number/boolean。服务将其规范化为 JSONB containment 对象，记录 get/query/update/delete 共用参数化 `data @> condition` 谓词；嵌套对象、数组、null、未知 envelope 和任意 SQL 文本均被稳定拒绝。
- 真实 PostgreSQL 集成覆盖：只有 `name=Scoped customer` 的记录被条件角色读取，另一条记录拒绝；`access.explain` 返回 `conditional_scope`，API/MCP/CLI 成功与非法条件错误保持等价。
- 当前实现复跑：1,000,000 records、100,000 `record × group` projections、50 concurrent runtime readers；insert 20,220.986 ms，query p50 285.095 ms、p95 353.419 ms，passed。该本机实测不构成生产 SLA 或 TASK-019 的完整容量验收。

## 2026-07-23 TASK-016 组织重组多批可靠性

- 真实数据库集成用例先取消一个无记录 merge（验证只在 0 已迁移记录时可取消），随后在源组织创建两条受保护记录并重启 merge。源组织改为 `migrating` 后，owner 的 `runtime.record.create` 返回 `FAILED_PRECONDITION`，不会为正在退休的组织创建新的 data anchor。
- `organization.merge.execute` 以 `batch_size=1` 连续执行两次：第一批保持 `running` 且 `records_migrated=1`，第二批才变为 `completed` 且 `records_migrated=2`；两条记录的 `data_organization_id` 和 owner 的主成员关系均迁移至目标组织。
- `TEST_DATABASE_URL=... GOTOOLCHAIN=go1.26.5 go test ./internal/record -run TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope -count=1 -v`：passed。

## 2026-07-23 TASK-016 三入口与最终本机门禁

- 受保护对象的真实集成用例现通过 API、MCP、CLI 分别验证：无对象 read 权限返回 `UNAUTHORIZED`；未经字段 write 授权的 create 返回 `UNAUTHORIZED`；条件数据范围对匹配记录允许、对不匹配记录返回 `RESOURCE_NOT_FOUND`。适配器的 HTTP 状态同时按稳定错误码验证为 403、403、404，CLI 和 MCP 错误消息保持一致。
- 当前最终门禁：`TEST_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:55432/postgres?sslmode=disable GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 `CGO_ENABLED=0 -trimpath go build`、状态校验与 `git diff --check`：全部 passed。

## 2026-07-24 TASK-026 发布前质量门

- AgentCiCi 与 Semattice 的真实本机 HTTP 联调通过：AgentCiCi 创建组织后，以内部 HMAC 调用 Semattice 受控开户入口；reserve、projection、complete 全部成功，同一幂等键重试没有新增 tenant 或 operation。
- `./scripts/test-postgres.sh run`、`go test -race ./...`、`go vet ./...`、`go mod verify` 和 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=go1.26.5 go build -trimpath -ldflags='-s -w' -o /tmp/semattice-release-check ./cmd/ai-native-platform`：全部通过。
- 修复并覆盖无数据库 MCP/CLI 模式下空计量服务的接口 nil 陷阱；公开 `tenant.provision` 能力保持未发布，数据库角色接线测试以已验证的 tenant fixture 覆盖 metadata 运行时路径。

## 2026-07-24 TASK-026 生产发布验收

- 专用 `semattice_migrator` 成功执行生产正向 migration，`schema_migration` 达到 version 16；临时 `CREATEROLE` 已在迁移完成后撤销。运行时 control/runtime 身份未用于 migration。
- 新二进制 SHA-256 `c1617398e9ddf3b83a942fa8b5852e54f7caf943900771703e8b1bacbf712962` 经远端校验后，`/opt/semattice/current` 原子指向 `20260724T0045Z-controlled-provisioning`；`systemctl` active、edge health 通过，近 20 分钟日志无 panic/fatal。
- 跨生产主机 HMAC smoke：匿名请求被 403 拒绝；来自 AgentCiCi 的有效签名请求经过 Semattice 和 AgentCiCi 双向校验后，因专门构造的不存在组织返回 `FAILED_PRECONDITION` / HTTP 412。该负向探针未创建 tenant、reservation 或 operation，证明 fail-closed 与无副作用失败路径。
