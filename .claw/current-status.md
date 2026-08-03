---
kind: current-status
version: 3
updated_at: 2026-08-03T05:13:45Z
updated_by: root
phase: merged-production-line-validation
active_task: "TASK-052"
next_action: "统一主线全量验证通过；提交合并并构建发布最终不可变 release。"
read_next:
  goals: true
  decisions: true
  issue_list: true
  task_board: true
  test_report: true
  devops: false
---

# 项目当前状态

## 快照

- TASK-052 / ISSUE-003：members 聚合排序修复已在临时 release 验证；CodeUp `df308b1` 的手动元数据发布、零字段对象和 migration 17 校验和兼容已合并。PostgreSQL 16 全仓、全量 race、vet、module、16 项 Skill 测试、JS/Shell 和 Linux 构建通过，等待提交并发布最终统一 release。

- TASK-050 / ISSUE-002 修复提交`36e1c0a32b2ed0e00755a6b2fd857969868e586c`已发布为`/opt/semattice/releases/20260731T101946Z-web-oidc-36e1c0a32b2e`，二进制SHA-256为`f26f18994494365efe030bbe29739967b9c60c704dbfea189e28d8fee5e11528`。四项服务active、Nginx和健康/匿名负例通过、错误日志为0，线上静态契约包含新归一逻辑。真实Chrome旧Session已过期，Keycloak未保留前台SSO，页面已交回用户登录；登录后仍需最终点击验收。

- TASK-051 / FEAT-035：已完成目标租户研发身份与强制授权治理。产品总监 HUMAN 绑定 AgentCiCi 全局用户 `18611892001`；产品经理和开发者 SERVICE 均投影其 owner，三者状态 active 且分别绑定“产品总监 / 产品经理 / 开发者”角色。活动元数据版本 `019fbde4-76cf-73d9-b36a-324692b10d05` 为 5 个对象、42 个字段，5 个对象策略全部 enforced，三类主体均有研发交付部 primary membership。独立审批 `f1591286-71bb-49ed-b874-80a7c7640fa9` 下的开发者投影暂停会阻断 CLI，自恢复被禁止；人类管理员恢复后 CLI 成功，审计记录两次状态写入。JWKS 私网/公网路径与固定 issuer 配置已上线，生产服务 active。

- 本地 `main` 已合并手动元数据发布、零字段控制台修复与研发身份治理三条提交线，并通过全量 race、vet、module、Linux 构建和状态校验；当前生产 release `20fe64e` 不包含提交 `34023d0` 和 `36e1c0a`，后续需从合并提交构建新 release 后再执行联合生产验收。

- CodeUp `origin/main` 已刷新至 `874f3fd`，其研发身份治理提交线已纳入本地合并结果；本次不推送远端，也不改写历史。

- TASK-049 / FEAT-046 已完成：提交`34023d0a55981761ed0642809b82a5f5b2f7db9f`发布为`/opt/semattice/releases/20260731T092534Z-web-oidc-34023d0a5598`，二进制SHA-256为`6a24e4434b4eb97157d30e4284c0537147d3f8032ad2f1d10cd4e8a920a721f3`。`metadata.version.publish`现在接受用户明确提供的非空手动`approval_id`并在发布事务中持久审计，其他高风险能力仍校验可信OACT审批声明。四项服务active、Nginx和健康检查通过、错误日志为0；Skill开发/安装副本均为`1.4.0`。

- 用户已明确确认并发布生产元数据首版本：当前OACT绑定企业`orgx2x8awt02djpp5xdp`与租户`ce85dabd-68be-503d-9d1b-9b63c536fa78`。版本`019fb736-8c34-7f0c-a0e8-82f385ffd9b0`现在为`published`，包含`contact / 联系人`对象`019fb736-c3cb-7e1b-8f98-b93614102672`，以及`large_backpack / 大书包`对象`019fb75e-29db-7726-8ff0-8c5033ae08d8`；后者包含active索引字段`name / 书包名称`和`color / 颜色`。快照摘要为`9d56197d04962a3ab6ad60b4610fc4035dd51b2cb6b9f049d719eb88c9953f4f`，发布审计标识为`audit:req-f7b19407-5c65-48b3-8eaa-fdd6369c063b`。未创建业务记录或修改授权。

- TASK-048 已完成：提交`ffdbec4fada7aa0169d75dd785bac8607cf927b8`发布为`/opt/semattice/releases/20260731T080337Z-web-oidc-ffdbec4fada7`，二进制SHA-256为`0e31e75dc59487c6bdf02a9eed169f826fa918d2427e3373a304c2584d8f57f0`。四个服务active、健康和匿名负例通过，真实Chrome顶栏精确显示“应用开发组织”且控制台零错误；上一release、Nginx和静态站备份均保留。

- TASK-047 已上线：控制台右上角不再展示Keycloak内部用户UUID，改为展示当前`/console/api/overview`返回的`tenant_name`，缺失名称时回退为“当前租户”；未修改登录Session、Organization映射或后端接口。

- TASK-046 / FEAT-045 已完成：提交`dcf2b811b7ec88d0685938f6d6564c818ba24314`已发布为`/opt/semattice/releases/20260731T074549Z-web-oidc-dcf2b811b7ec`，二进制SHA-256为`d000e922e0231d39cca9040821bc42cdfa7b96411ad782d5b679bd083db93b87`。现有`semattice-web` Secret只在服务器内写入受保护文件；服务、Nginx、健康与匿名负例、OIDC 303/S256 PKCE均通过。真实Chrome登录成功回到企业管理中心，显示当前租户和退出按钮且浏览器控制台零错误。上一release与环境、Nginx、静态站备份均保留。

- TASK-045 / FEAT-044 已随TASK-046上线：保留`semattice-cli`，新增`semattice-web` confidential OIDC网站登录。`/auth/oidc/login`使用state/nonce/S256 PKCE，回调以client_secret_basic换码并验证access/ID token、subject、唯一Organization和active tenant，成功后只创建最长15分钟且不含Keycloak Token的签名安全Cookie。

- TASK-044 / FEAT-043 已完成：`cloudcc-semattice` `1.3.0` 默认请求当前51项公开Capability所需的全部26个唯一scope，旧v2登录缓存fail closed并要求重新登录；开发副本与本机安装副本一致。生产 `AI_NATIVE_OACT_ALLOWED_SCOPES` 已扩展为同一集合，备份为 `/etc/semattice/semattice.env.backup.20260731T052514Z-all-capability-scopes`。真实PKCE登录返回26个scope；线上能力发现为51项/26个scope且零差异，`tenant.get-status`只读调用成功。scope不替代Principal/RBAC、RLS、审批、幂等或审计，验收未执行业务写操作。

- ISSUE-001 已关闭：当前生产release从同一提交构建，已同时包含真实治理控制台、Semattice CLI自有登录和网站OIDC登录，并完成真实浏览器验收。

- TASK-043 / FEAT-042 已完成：历史独立认证release为`/opt/semattice/releases/20260731T045751Z-standalone-auth`，现已由TASK-046组合release取代并保留回滚。旧AgentCiCi开户环境变量已移除，`semattice-api` audience mapper唯一，Organization scope仍为optional。真实PKCE登录已将`orgx2x8awt02djpp5xdp`映射到active tenant `ce85dabd-68be-503d-9d1b-9b63c536fa78`，短期OACT调用`system.capability.list`返回succeeded / 51项能力。TASK-043完成时allowlist仅开放`system.capability.read`，后续已由TASK-044扩展为全部26个公开能力scope；未创建租户/Principal/RBAC、未改其他Keycloak数据、未发布独立Skill release。

- TASK-042 / FEAT-041 本地实现已完成：Semattice新增 `POST /v1/auth/token`，固定验证 Keycloak RS256 issuer/audience/JWKS、`azp=semattice-cli` 和唯一 Organization alias，再映射 active `tenant_registry.company_id` 并使用 Semattice自有 HS256 identity key签发短期 OACT。Skill/CLI删除 AgentCiCi base URL、公司目录和外部 mint调用，缓存升至 v2；`serve`删除 AgentCiCi开户启动门禁、HMAC路由和 reservation/complete代码。Skill升至 `1.2.2` 并同步本机安装目录。14 项 Python测试、全量 Go race、vet、module/build、技能/YAML/bash/diff/secret检查通过。现有 Keycloak Realm/域名作为基础设施标识保留。

- TASK-041 / FEAT-040 本地实现已完成：`cloudcc-semattice` `1.2.1` 开发副本新增 `semattice login/status/logout/call`，以 Keycloak Authorization Code + S256 PKCE、本机 `127.0.0.1` 动态端口、Organization Scope、系统凭据库 refresh token和 `0600` 短期 OACT 缓存实现人类登录；API 临期或 401 时安全刷新一次，显式 `SEMATTICE_TOKEN` 继续优先。Bearer 请求禁止 redirect，认证错误不输出服务端描述，默认只请求 `system.capability.read`。后续 FEAT-041 已将外部换票替换为 Semattice 自有 `/v1/auth/token`。

- TASK-040 / FEAT-034：已发布 `/opt/semattice/releases/20260731T012059Z-console`。管理中心不再返回内存演示 fixture，而是由已验证 OACT 会话的 tenant context 经 runtime RLS 查询真实数据。目标公司 `org5nszpgj99jaysxv6y` 为 metadata v1 published，真实控制台 API 返回 5 个研发交付对象、37 个有效字段；其 Semattice 本地成员、角色和组织投影均为 0，页面以明确空态展示，绝不再显示 `example.demo` 或“演示环境”。

- TASK-039 已完成：项目技能与发布证据以提交 `a55d71d773446902598b28fb525c7562003f351b` 快进推送至阿里云 CodeUp `main`，远端分支回读一致。独立技能仓库将 `main + v1.1.0` 原子推送至 `https://github.com/CloudCCAI/cloudcc-semattice`，release commit 为 `3ac29afc34366d66a2e9320975dc3be498d55181`；本地 HEAD、`origin/main` 和 `v1.1.0^{}` 一致，远程 HEAD 指向 main，仓库页、标签页及 raw VERSION/README 均验证通过。未使用 force push，未移动历史标签。

- TASK-038 已完成：`cloudcc-semattice` 本地开发版本升至 `1.1.0`，新增产品定位与业务模块指南，并把技能入口拆分为“理解与设计 / 实施与调用 / 设计后实施”三种模式。对象、字段和关系现在具有明确的创建、读取、修改、删除或退役边界；已发布元数据演进明确要求完整复制定义到空候选草稿并保持稳定 ID。官方技能校验、YAML、链接、无 Token dry-run、本地安装目录一致性均通过。TASK-038 完成时仅同步本机安装目录；后续 GitHub 正式发布由 TASK-039 记录。

- TASK-037 已完成：技能 ID、项目内目录、标题、UI 显示名和调用名已统一为 `cloudcc-semattice` / `CloudCC Semattice（语义格）` / `$cloudcc-semattice`。不兼容调用名变更以 `1.0.0` 发布到 `https://github.com/CloudCCAI/cloudcc-semattice`；远端 `main` 与 annotated tag `v1.0.0` 均指向提交 `5b156c057af7517c81f5892d1f8123ec74f00ea6`，远端 VERSION、README、标签页面和安装示例均已验证。

- TASK-036 已按用户纠正：项目根 `AGENTS.md` 是“项目内开发副本 → 独立发布仓库”内部流程的唯一维护入口；技能 README 只保留面向安装者的介绍、安装、使用和版本升级说明。项目内技能目录仍不初始化 `.git`；独立仓库的本地 README 已同步但未提交或推送，远程已发布版本仍为 `v0.1.1`。

- TASK-035 已完成：`semattice-customization-expert-universal` 采用根目录 `VERSION` + SemVer + `v<version>` Git 标签管理升级，首发版本为 `0.1.0`，并包含用户要求的 README。独立仓库已发布到 `https://github.com/CloudCCAI/semattice-customization-expert-universal`；远程 `main` 与 annotated tag `v0.1.0` 均指向提交 `93c2701`，公开页面、远程 VERSION 和 README 已验证。

- TASK-034 已完成：仓库内在制技能已从 `semattice-operator` 重命名为 `semattice-customization-expert-universal`，目录、SKILL frontmatter、文档标题、UI 显示名和默认 `$skill` 调用名保持一致；API 约束、参考资料和脚本功能未改动。

- TASK-033 / FEAT-033 已启动：AgentCiCi `2.8.20` 已发布统一 Principal 基座和机器责任模型，但 Semattice 当前仅把官方 OACT `sub` 直接作为 actor，尚未承载 `principal_id` / `principal_type`。本任务将保持 OACT/JWKS 本地验签，拒绝 Keycloak Service Account token 直连，并将新官方 human/service OACT 映射为数据平台本地 Principal。Keycloak Realm 尚未配置 SMTP，自动人类邀请不提前启用。

- 术语边界已确认并写入项目规范：后续“数据平台”始终且仅指本仓库的 CloudCC Semattice（语义格，`/Volumes/AISpace/codehouse/AI-Native-Platform`）；Agent CC / AgentCiCi 与 CloudCC CRM 均是外部应用或集成方，不得混称。规范来源为根目录 `README.md` 与 `AGENTS.md`。
- `TASK-032` Semattice 企业管理中心第一版已由 TASK-040 真实数据投影替代：当前 release 为 `/opt/semattice/releases/20260731T012059Z-console`。顶栏产品菜单仍可直接回到 `https://x.agentcici.com/admin`，不传递或储存 OACT；控制台 HTML、CSS 与 JS 使用 no-store 响应。匿名治理 API 为 401，受管理 scope 保护的 API 才可读取当前租户真实治理事实。
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
- TASK-024/TASK-031 已完成：`serve` 进程提供 Streamable HTTP MCP `/mcp` endpoint。`initialize`、`notifications/initialized`、`tools/list` 可匿名发现；每个 `tools/call` 均走 Bearer JWT 校验，并从该请求绑定 `tenant_id + subject + scopes`，不信任 discovery session 或客户端身份参数。2026-07-24 已在授权 ECS 发布 release `20260724T155000Z-mcp-public-discovery`；Nginx 的无缓冲 `/mcp` 代理固定 loopback Host，公网匿名 initialize/tools/list 均成功、匿名工具调用返回 401，edge health 为 200。
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

- 2026-07-29 TASK-037 发布验证：项目内目录 basename、`SKILL.md` frontmatter、README 标题/安装路径、`agents/openai.yaml` 显示名与默认 `$cloudcc-semattice` 调用名一致；当前文件旧技能身份零残留。官方技能校验、YAML、Python 语法、CLI help、无 Token dry-run、版本/README 一致性、两目录一致性和 diff 检查均通过。`main + v1.0.0` 已原子推送到 `CloudCCAI/cloudcc-semattice`，三方提交均为 `5b156c057af7517c81f5892d1f8123ec74f00ea6`，远端页面与发布内容已回读验证。项目状态 validator 仅保留既有 `FEAT-033` 错误。
- 2026-07-29 TASK-036 文档边界纠正：项目内与独立发布副本的技能 README 均已删除内部维护/同步/推送步骤，完整流程只存在于项目根 `AGENTS.md`。官方技能校验、YAML、Python 语法、CLI help、无 Token dry-run、两目录一致性和 diff 检查均通过；本次未提交、打标签或推送。
- 2026-07-29 TASK-036 发布流程固化与 `v0.1.1` 发布完成：同步 dry-run 只包含 README/VERSION，官方技能校验、YAML、Python 语法、CLI help、无 Token dry-run、SemVer 一致性、缓存/私钥扫描和 diff 检查均通过。原子 push 后，本地 HEAD、远程 main 和 `v0.1.1^{}` 均为 `228f6f737b53ce41cc3f51126ca58498d33a3f47`；仓库页面 HTTP 200，远程 VERSION 为 `0.1.1`。
- 2026-07-29 TASK-036 项目状态 validator 另行复验；仅被既有 `FEAT-033` 缺少 `feature_id` / `updated_at` / `updated_by` 和非标准 status 阻断，TASK-036 新增状态记录未产生新错误。
- 2026-07-29 TASK-035 发布完成：官方 `quick_validate.py`、`agents/openai.yaml` YAML、SemVer/目录结构、Python 语法、CLI help 和无 Token dry-run 均通过；独立仓库以原子 push 发布 `main + v0.1.0`。本地 HEAD、远程 main 和 tag peeled commit 均为 `93c270124c7992612100380676cecf4affc31b5d`，默认分支为 main，公开页面 HTTP 200，远程 VERSION 为 `0.1.0`，README 标题和版本引用验证通过。
- 2026-07-29 TASK-034：`skill-creator` 的 `quick_validate.py` 返回 `Skill is valid!`；技能目录名与 frontmatter 名称一致（40 字符），技能目录内旧名称零残留，Python 辅助脚本语法与技能目录空白检查通过。项目状态 validator 另被既有 `FEAT-033` frontmatter 缺字段和非标准状态值阻断，本次未修改该无关规格。
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
