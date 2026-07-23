---
kind: decisions
version: 3
updated_at: 2026-07-23T01:22:19Z
updated_by: ai after accepted CloudCC Semattice naming ADR-012
---

# 技术决策记录

`decisions.md` 是架构和技术选型的唯一事实源。

## 决策索引

| 编号 | 标题 | 状态 | 日期 | 替代/被替代 |
|------|------|------|------|-------------|
| ADR-003 | AI 原生 CRM + PaaS 目标架构基线 | accepted | 2026-07-18 | 由 FEAT-009 正式批准 |
| ADR-004 | 纯 Agent 能力契约与三入口要求 | accepted | 2026-07-16 | - |
| ADR-005 | Phase 0 Capability Contract PoC 运行时提案（已替代） | superseded | 2026-07-17 | ADR-007 |
| ADR-006 | 共享身份联邦与数据平台独立授权边界 | accepted | 2026-07-17 | 2026-07-18 明确身份服务与产品订阅解耦 |
| ADR-007 | Go Agent Runtime 与二进制交付 | accepted | 2026-07-17 | 取代 ADR-005 |
| ADR-008 | Phase 0 PostgreSQL Docker 容量基线 | accepted | 2026-07-17 | - |
| ADR-009 | 既有运营端统一租户控制面与安全优先 UUID 租户标识 | accepted | 2026-07-18 | 修订 ADR-006 的租户映射部分 |
| ADR-010 | 动态字段、按需索引与 JSONB 安全演进边界 | accepted | 2026-07-20 | Phase 0 安全边界已接受；数值待 TASK-019 校准 |
| ADR-011 | 角色中心 RBAC、组织数据范围与混合记录共享 | accepted | 2026-07-22 | 用户确认；由 FEAT-016 实施 |
| ADR-012 | CloudCC Semattice 产品品牌与兼容命名边界 | accepted | 2026-07-23 | 取代对外使用泛称 AI Native Platform / Native Platform |

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

- 状态：`accepted`
- 日期：2026-07-13
- 接受日期：2026-07-18；用户正式批准 `FEAT-009` 为 Phase 0/1 架构与实施基线。
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

- 状态：`superseded`
- 日期：2026-07-17
- 被替代：`ADR-007`
- 当前结论：不适用。ADR-007 已接受 Go 1.26.5 单运行时与二进制交付，取代此 Node/TypeScript PoC 提案及其未执行的环境、依赖和验证前置条件。
- 完整历史与当时证据：`docs/archive/decisions/ADR-005-superseded-runtime-proposal.md`。

## ADR-007 - Go Agent Runtime 与二进制交付

- 状态：`accepted`
- 日期：2026-07-17
- 背景：目标 Agent 环境可能位于国内、私有化或受限网络中。要求每个已发布原子能力同时通过 API、MCP 和无交互 CLI 访问，而使用方不应安装 Node.js、npm 或 `node_modules`。
- 备选方案：
  - Node.js/TypeScript 生产交付：拒绝。它会把运行时、npm 和传递依赖安装带入每个 Agent 目标环境。
  - Java 生产交付：暂不采用。其企业生态和虚拟线程能力可作为未来选项，但 JRE/JDK 分发和许可生命周期不是当前 Phase 0 的最小路径。
  - Go 单运行时：采用。
- 结论：Phase 0 的 CI 与发布基线使用 Go `1.26.5`。一个 `ai-native-platform` 二进制以非交互命令运行 REST/功能 API、MCP（stdio 或 HTTP）和 JSON/JSON Lines CLI；三种入口只适配同一 Capability Registry 与 shared invocation 层。MCP stdio 的 stdout 仅保留 JSON-RPC，诊断写入 stderr。Agent 主机只下载、验签并运行已批准制品，禁止运行时 `go get`、包管理器安装或自更新。
- 供应链与许可：CI 通过内部 Go module proxy 获取锁定的 `go.mod`/`go.sum` 输入；每个发布制品必须包含 SPDX 或 CycloneDX SBOM、许可证清单、`THIRD_PARTY_NOTICES`、构建来源、SHA-256 和签名。初始允许 MIT、BSD-2-Clause、BSD-3-Clause、Apache-2.0、ISC；其他许可证必须先经人工合规审批。
- 后续影响：Go 1.26.4 仅是本机现有开发环境，不能充当 Go 1.26.5 的 L2 构建/测试证据。JSON Schema validator 与 MCP Go SDK 的具体版本必须在依赖许可门禁后锁定，但 Capability Contract 的权威格式固定为 JSON Schema Draft 2020-12。
- 验证方式：在获得 L2 allowlist 后，以 Go 1.26.5 对 `system.capability.list` 运行 API/MCP/CLI parity tests、无 TTY CLI 测试、MCP stdout 纯净测试、交叉平台构建、SBOM/许可证/签名检查和受限并发测试。
- 参考资料：
  - `docs/superpowers/specs/2026-07-17-go-agent-platform-runtime-design.md`
  - https://go.dev/doc/devel/release
  - https://go.dev/LICENSE
  - https://modelcontextprotocol.io/docs/sdk

## ADR-008 - Phase 0 PostgreSQL Docker 容量基线

- 状态：`accepted`
- 日期：2026-07-17
- 背景：Phase 0 需要可重复的 PostgreSQL 分片、RLS、连接池与 `object_record` 基准，但不应把生产 HA 与容灾范围混入首个架构 PoC。
- 结论：Phase 0 使用本机 Docker 中已验证的 PostgreSQL `16.13`（`postgres:16`，镜像 ID `sha256:5d143123fdf80462d1778cd4f24b9f7ca13c87174bca19141fb194c5a1ebca59`），以单可用区、单 writer PoC 运行。Phase 0 不实施 HA、WAL 持续归档、备份或恢复演练。容量基准覆盖 8 GiB 与 16 GiB 两个 Docker 内存档位、50 个并发请求、200 名活跃用户和 1,000,000 条业务记录；现有普通记录读/写 P95 目标仍分别为 300 ms / 500 ms。
- 说明：用户给出的 8G/16G 在本 ADR 中按 Docker 内存上限解释；L2 基准 manifest 必须额外冻结 CPU 核数、卷类型/IOPS、PostgreSQL 参数、记录宽度、读写比例和热租户分布。若 8G/16G 原意为存储容量而非内存，必须在 L2 前修订本 ADR。
- 后续影响：Phase 0 不得把单可用区 PoC 的可用性或恢复结果宣传为生产 HA/DR 能力。连接代理、生产托管形态、备份、恢复和多可用区故障切换留待独立生产 ADR。
- 验证方式：基准必须分别报告两个内存档位下的 p50/p95/p99、错误率、CPU、内存、磁盘、连接池饱和度、队列深度和租户公平性；仅在 1,000,000 条记录和规定用户/并发模型下满足普通读写 SLO，才能宣称通过本 PoC 容量基线。

## ADR-006 - 共享身份联邦与数据平台独立授权边界

- 状态：`accepted`
- 日期：2026-07-17
- 背景：AI Native Platform 将作为可独立部署、可承载客户业务数据的应用运行；Agent CC 与 Native 均可单独开通，但同一企业在需要时应能复用一个账号、组织和登录体验。若两个系统各自维护账号，会造成重复身份、停用遗漏和审计断裂；若数据平台直接复用 Agent CC 的用户表、角色表或数据库权限，则会让 Native-only 租户依赖未开通的产品，并把应用角色错误地放大为对象、字段和记录权限。
- 备选方案：
  - 独立账号体系：拒绝。无法保证同一用户、组织和离职状态在两个系统的一致性。
  - 直接共享 Agent CC 数据库与角色表：拒绝。会产生数据库耦合、跨应用发布风险及授权边界混淆。
  - 统一身份认证 + 数据平台独立授权：采用。
- 结论：共享身份服务或企业 IdP 是账号、SSO、MFA、全局 `subject_id` 和组织成员状态的认证事实源；现有 Agent CC 身份中心可以演进为该共享能力，但必须与 Agent CC 产品订阅解耦并支持 Native-only 租户。数据平台只接受签发给自身 audience 的短期联邦令牌并维护最小本地成员投影。对象、字段、记录、共享、元数据发布与 Capability scope 由数据平台独立计算；两个产品不直接读写对方用户数据库。Agent 与服务账号使用独立短期身份；代表用户运行时记录委托链、资源范围、预算与策略版本。
- 2026-07-18 修订：原“数据平台自行维护 `global_org_id -> tenant_id` 映射”不再适用。租户身份、开户和生命周期改由 ADR-009 的统一运营控制面负责；令牌和本地投影直接使用其统一分配的 UUIDv4 `tenant_id + 20 位 company_id`。本 ADR 的身份认证与数据授权分离边界继续有效。
- 原因：该边界同时保留单点登录和统一组织体验、独立应用部署能力，以及面向业务数据的最小权限与可追溯授权。
- 后续影响：TenantContext、Capability Contract、MCP、API 和 CLI 都必须携带可验证的 actor、租户、受众、会话、请求和委托信息。数据平台需要验证令牌签发者、受众、有效期和成员状态，再执行本地数据权限；用户/组织停用、角色变化及紧急吊销必须能在本地授权投影中生效。
- 验证方式：在 Phase 0/1 的身份测试中证明：Agent CC-only、Native-only 和双产品租户均可使用共享主体访问已授权产品，Native-only 不要求 Agent CC 产品 active；错误 audience、过期、撤销、跨组织或跨租户令牌均被拒绝；Agent CC 应用角色不能绕过数据平台字段/记录策略；Agent 代用户调用的审计链可还原委托主体、能力、资源和请求。
- 参考资料：
  - `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md` 第 4.3、8、17 节
  - `docs/specs/FEAT-020-pure-agent-capability-contract.md`

## ADR-009 - 既有运营端统一租户控制面与安全优先 UUID 租户标识

- 状态：`accepted`
- 日期：2026-07-18
- 背景：Agent CC 已有独立运营管理端并由其管理 Agent CC 多租户开户。Agent CC 与 Native Platform 是两个可独立、可按任意顺序开通的产品，不存在主从关系；当同一企业需要绑定两个产品时，两项订阅必须属于同一个全局租户。OneDatabase 多租户的首要风险是错误租户上下文、关系约束或迁移操作造成数据串租。用户最终确认安全和治理可恢复性优先于 8 字节标识的存储优势，并要求避免连续序列在跨环境迁移、回填和恢复中的协调风险。
- 备选方案：
  - Native Platform 自行生成 UUID 并维护 `company_id` 映射：拒绝。会形成第二套租户身份、重复开户和映射漂移。
  - 两个平台直接以 20 位 `company_id` 作为所有表的物理租户键：不采用。统一且可读，但字符串索引和比较成本高于 PostgreSQL 原生 UUID，且自定义 ID 规则需要独立治理。
  - 统一运营控制面分配连续 `BIGINT tenant_id`：不采用。8 字节性能最好，但序列跨环境合并、存量回填、灾备恢复和错误数字命中有效租户的治理风险不符合安全优先要求。
  - 统一运营控制面生成 UUIDv7：不采用为租户主键。UUIDv7 适合高频记录写入，但租户创建低频，不需要时间有序索引收益；其时间前缀和可推断创建时间也没有租户安全收益。
  - 统一运营控制面生成随机 UUIDv4 `tenant_id`，并保留一对一 20 位 `company_id`：采用。
- 结论：既有 Agent CC 运营管理端升级为产品无关的统一租户运营控制面，是全局租户目录、Agent CC/Native 独立产品订阅和生命周期的唯一事实源。一个全局租户分别拥有 Agent CC `0..1` 与 Native Platform `0..1` 订阅；开通任一首个产品时，运营端使用密码学安全随机源生成终身稳定、不可复用且不编码时间、shard、region 或 bucket 的 UUIDv4 `tenant_id`，同时维护一对一 20 位 `company_id`。任一产品可长期不开放；以后绑定第二个产品时必须复用同一个 `tenant_id + company_id`。
- 数据所有权：统一运营端拥有 `tenant_id`、`company_id`、全局生命周期和每个产品的独立开通状态；共享身份服务或企业 IdP 拥有账号、组织成员与全局主体，且不依赖 Agent CC 产品订阅；Agent CC 拥有智能体域授权；Native Platform 仅在开通后拥有 shard/bucket 路由、配额投影、元数据与业务数据授权。各方通过受众受限 API/事件和独立审计协作，不共享数据库表、数据库角色或长期凭据。
- 开户语义：请求可指定 `agent_cc` 或 `native_platform` 以及可选的已有全局租户引用。若没有引用，运营端先创建全局租户并分配 `tenant_id + company_id`，然后只开通所选产品；若要与已有产品绑定，必须选择其全局租户并复用同一标识。每个产品使用独立 `operation_id` 和产品修订号；任一产品开通失败只更新自身状态并可幂等恢复，不回滚另一产品、不改变全局标识、不生成第二组 ID。若两个已运行产品属于不同全局租户，禁止静默改绑，必须另立租户合并/数据迁移决策与流程。
- Native 数据模型：`tenant_registry.tenant_id` 和所有业务、关系、索引、事件及审计表的 `tenant_id` 使用 PostgreSQL 原生 `uuid`，禁止以 `varchar(36)` 保存；`tenant_registry.company_id` 使用唯一的 `varchar(20)` 业务编号。Native 不生成全局租户 ID，只维护产品侧生命周期、配额和 `tenant_id -> shard_id + tenant_bucket + route_revision`。
- 隔离边界：UUID 不是授权或 RLS 的替代品。外部请求不得从 body/header 任意指定受信租户；网关必须从已验证令牌和本地 active 投影构造 TenantContext。每张租户表的 `tenant_id` 必须 `NOT NULL` 并受 RLS 约束；查找、主从和多对多关系必须携带 `tenant_id`，使用同租户复合唯一键/外键或等价确定性校验；数据库账号不得 `BYPASSRLS`，连接使用事务级 `SET LOCAL` 并在池化复用前验证无残留。
- 记录 ID 边界：本决策只改变租户标识。`record_id`、`object_id`、`field_id` 等高频非租户实体继续使用 UUIDv7，以获得大致时间有序的索引写入特性；若以后改用其他类型，必须另立 ADR 和基准证据。
- 迁移：存量 Agent CC `company_id` 必须先做唯一性、格式和状态盘点；运营端为每个存量组织回填 `tenant_id`，Agent CC 添加唯一映射但首步不替换现有字符串主键。回填后 Native 默认 `not_provisioned`；只为明确选择 Native 的租户执行幂等按需开通。异常 ID 不截断、不覆盖、不重新生成。
- 原因：该方案同时满足两个产品独立开通、按需绑定时共享一个企业租户身份、复用现有运营资产、全局唯一与不可枚举、跨环境无需协调序列、错误 ID 更倾向于无匹配而不是静默命中另一租户、跨分片迁移时 ID 不变，以及两个产品独立数据和权限边界。相对 `BIGINT` 增加的 8 字节/值必须通过容量测试量化，但在当前 100 万记录 PoC 目标下接受该成本以换取更安全的治理特性。
- 后续影响：`FEAT-009` 的本地组织映射改为直接消费统一 UUIDv4；TenantContext、`tenant_registry`、对象记录/动态索引表和 RLS 使用原生 UUID `tenant_id`；`TASK-011` 改为既有运营控制面集成，而不是在 Native 中新建全局租户服务。开户、暂停、恢复、套餐变更和注销能力仍须提供等价 API/MCP/CLI。
- 验证方式：完成存量 `company_id` 审计；验证 UUIDv4 生成与 `tenant_id/company_id` 一对一唯一约束、Agent CC-only/Native-only/双产品三种合法状态、任一首个产品开户、第二产品绑定复用标识、任一产品失败不影响另一产品、每个全局租户每种产品最多一个投影、不同全局租户禁止静默改绑、乱序产品修订拒绝、错误 audience/伪造租户拒绝、RLS fail-closed、连接池上下文残留、跨租户查找/主从关系拒绝，以及 UUID `tenant_id` 在 100 万记录 PoC 下的索引、缓存、分区裁剪和路由性能。
- 参考资料：
  - `docs/specs/FEAT-011-unified-tenant-operations-control-plane.md`
  - `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md` 第 4、5、9、21 节

## ADR-010 - 动态字段、按需索引与 JSONB 安全演进边界

- 状态：`accepted`
- 日期：2026-07-19
- 接受日期：2026-07-20；用户批准按该方案开始 `TASK-014` 核心落地，容量数值仍允许由 `TASK-019` 通过新决策修订。
- 背景：动态字段保存在权威 JSONB 中，不需要为每个字段执行 DDL；但字段数量、记录尺寸和类型化索引数量会分别放大元数据编译、整行更新、WAL/TOAST、权限计算、索引存储与写入成本。字段定义一旦作用于存量记录，直接设 required、直接激活索引、原地改名/改类型或立即删除还会导致更新失败、查询漏数和数据失去解释。
- 备选方案：
  - 仅使用 PostgreSQL 约 1 GiB 单值理论上限：拒绝。物理可表示不等于共享 OLTP 可安全运行，无法约束锁竞争、WAL、网络和内存成本。
  - 沿用固定 500 物理字段槽位并全部预建索引：拒绝。重新引入宽表/无差别索引问题，且百万记录下存储和写放大不可接受。
  - JSONB 稀疏权威值 + 字段定义配额 + 按容量分档的 active typed-index 配额 + 分阶段安全演进：建议采用。
- 结论：每对象在线动态字段默认 300、硬上限 500；8 GiB/16 GiB 档 active typed-index 默认 10/20、硬上限 20/40，平台绝对上限 50 且只允许专属分片专项批准；单记录规范化业务 JSON 平均/P95 目标 4/16 KiB，64 KiB 告警、256 KiB 硬拒绝，单个 JSON 字段不超过 64 KiB、嵌套不超过 8 层、单数组不超过 1,000 元素。required、indexed/unique/reference、约束收紧、改名、改类型和删除必须走 prepare/backfill/validate/activate/retire 协议。
- 原因：字段定义与实际记录可以保持稀疏，500 定义不必产生 500 个空值；但当前百万记录实测平均每个“记录 × typed-index 字段”约 `368.60` bytes，20/40/50 个满覆盖索引字段约为 `7.37/14.74/18.43 GB`，必须单独限制。256 KiB 产品上限为异常记录保留空间，同时防止大对象成为常态；正文、文件和无界子集合使用专用存储或关系对象。
- 后续影响：`TASK-014` 已实现字段/index 生命周期、Changeset 模拟、可恢复回填、coverage gate 和 API/MCP/CLI 配额错误等价投影；`TASK-019` 用真实 8/16 GiB 负载确认或修订数值，修订通过新增 `governance_policy.policy_version` 和本 ADR 状态更新完成。
- 验证方式：验证 300/500 字段的元数据编译和 Agent schema 成本；在 1,000,000 记录下分别测量 4/16/64/256 KiB 记录形状与 10/20 个 active typed-index 字段的吞吐、P95/P99、WAL、磁盘、vacuum、buffer 命中和热点租户公平性；故障注入验证 required/index backfill、并发 revision、暂停恢复和覆盖率不足时 fail closed。
- 参考资料：
  - `docs/specs/FEAT-014-changeset-and-dynamic-field-evolution.md`
  - `docs/specs/FEAT-015-record-runtime-and-typed-indexes.md`
  - https://www.postgresql.org/docs/16/datatype-json.html#JSON-DOC-DESIGN
  - https://www.postgresql.org/docs/16/storage-toast.html
  - https://www.postgresql.org/docs/16/limits.html

## ADR-011 - 角色中心 RBAC、组织数据范围与混合记录共享

- 状态：`accepted`
- 日期：2026-07-22
- 背景：当前平台只有 JWT Capability Scope 和租户 RLS，无法表达租户内对象、字段、记录、组织层级、团队与例外共享。需求要求角色中心的最小权限和职责分离；同时讨论确认组织层级更适合作为数据范围，而不是父组织功能权限向下继承。Salesforce 的 Profile/Permission Set 与 Share 模型可提供分层参考，但 Profile 及大规模逐用户 share 行不适合本平台的纯 Agent、多租户和组织调整场景。
- 备选方案：
  - 直接采用 Salesforce Profile + Role Hierarchy + 每对象 Share 表：拒绝。Profile 承担的 UI/登录配置不适用于纯 API/MCP/CLI；角色层级语义与功能权限下放不同；逐记录共享重算会形成高写放大和热点。
  - 父组织功能权限向下自动继承：拒绝。部门调整或新建下级组织会隐式扩大管理权限，违反最小权限。
  - 纯 `record × user` ACL：拒绝。组织层级、用户组和规则变化将导致极大物化与重算量。
  - 角色中心 + Permission Set + 组织数据范围谓词 + 按组例外共享：采用。
- 结论：业务角色是管理员配置中心，角色关联可复用 Permission Set，Permission Set 承载对象、字段、Capability、平台/IAM/组织/审计原子权限；身份与会话安全策略独立，业务 Profile 不引入。对象动作和字段权限是第一道业务门。记录默认 private，Owner、稳定 `data_organization_id` 的组织闭包范围、团队、`share_grant`、规则共享共同给出候选记录集合，再与密级、委托和限制策略相交。组织树只用于数据范围；用户调岗不改写历史记录，组织合并使用显式、可审计的迁移并保留 tombstone。共享规则优先谓词计算，热点时最多投影 `record × group`，禁止 `record × user` ACL。
- 原因：该模型保留国内企业以角色配置权限的可理解性，避免角色/权限组合爆炸；将功能权限、数据范围和例外协作分离，既能满足上级查看下级数据，也不会因组织层级自动分发平台管理权限。稳定的记录数据归属避免人员调岗造成历史数据突然扩权；按组共享与版本化快照把规模控制在组织闭包、少量团队/例外授权和受限投影内。
- 后续影响：`TASK-016` 必须按 FEAT-016 先建本地授权投影和统一 PDP，再扩展记录运行时。当前 JWT Scope、TenantContext、FORCE RLS 和 control/runtime PostgreSQL 角色保持不变，且不能被业务角色取代。角色互斥、自审禁止、管理作用域上限、短期高危授权和授权依据审计成为强制验证项。
- 验证方式：验证对象/字段/记录三层 allow/deny、三入口 parity、Owner/组织范围/团队/例外共享、调岗与组织合并、角色互斥、自审禁止、跨租户失败、无 `record × user` ACL 物化以及 100 万记录热点组织下的真实 PostgreSQL 查询/重算成本。完整容量与 worker 证据分别由 TASK-019/TASK-017 补足。
- 参考资料：
  - `docs/specs/FEAT-016-role-centered-rbac-organization-data-sharing.md`
  - `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md` 第 4、9、17 节
  - `docs/specs/FEAT-012-postgresql-shard-isolation-poc.md`

## ADR-012 - CloudCC Semattice 产品品牌与兼容命名边界

- 状态：`accepted`
- 日期：2026-07-23
- 背景：`AI Native Platform` 与 `Native Platform` 只能描述技术方向，缺少可识别的专有产品名称；`Data Grid` 容易被理解为内存数据网格或表格组件，单独使用 `Lattice` 又与多个现有软件和 AI 品牌重名。产品需要一个能够覆盖业务数据、语义元数据、关系、租户边界、权限治理和原子能力投影的稳定名称。
- 备选方案：
  - `Data Grid`：拒绝作为产品品牌。行业既有含义过强，不能准确表达语义元数据与治理运行时。
  - `Lattice`：保留为核心设计意象，不单独作为品牌。含义准确，但名称占用和识别冲突较多。
  - `CloudCC Semattice`：采用。`Semattice` 是本项目提出的合成品牌词，由 `Sema(ntic) + (La)ttice` 构成，中文释义为“语义格”。
- 结论：产品正式英文名为 **CloudCC Semattice**，中文释义为 **CloudCC 语义格**；产品类别为 **Agentic Business Data Runtime**，中文定位为“面向智能体的业务数据与语义运行底座”。`Lattice` 表达租户、对象、字段、关系、记录、权限、策略和原子能力共同构成的受治理业务格网。
- 命名边界：新对外文档和产品叙述使用 `CloudCC Semattice` 或简称 `Semattice`。历史文档中的 `AI Native Platform` / `Native Platform` 均指同一产品；协议枚举 `native_platform` 继续作为稳定产品代码，不因品牌变化而修改语义。
- CLI 与服务规范：目标品牌 CLI 命令为 `semattice`，新服务和制品使用 `semattice-*` 前缀。当前 `ai-native-platform` 二进制、`AI_NATIVE_*` 环境变量、数据库角色/表名以及 `github.com/OlivierZEN/ai-native-platform` Go module path 保持兼容，不在本 ADR 中直接重命名。任何移除或替换都必须另立版本化迁移方案，提供别名、弃用周期、契约测试和回滚路径。
- 法律边界：该名称是项目命名创作，不等同于已经取得全球商标专用权。生产发布、域名购买或市场宣传前必须完备中英文商标、企业名称、域名和主要软件包/应用市场检索，并由人工确认。
- 原因：该名称保留 `Lattice` 的结构化关系网络含义，同时由 `Semantic` 明确区别于缓存网格、UI 表格和通用 AI 工具；CloudCC 主品牌提供产品族归属。兼容边界避免品牌决策意外破坏已实现的 API、CLI、部署配置和自动化脚本。
- 后续影响：README、目标和主规格使用正式品牌；后续新能力、文档和制品采用 Semattice 命名。现有技术标识的迁移不与 TASK-016 混合实施，需单独排期和验证。
- 验证方式：执行状态文档 validator、命名一致性检索和 `git diff --check`；未来标识迁移必须补充 API/MCP/CLI、配置、制品和升级兼容测试。
- 参考资料：
  - `README.md`
  - `.claw/goals.md`
  - `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md`

## ADR-013 - 全局公司身份与租户内权限组织的术语边界

- 状态：`accepted`
- 日期：2026-07-23
- 背景：统一运营控制面开户的全局企业业务编号曾使用 `org_id`，而授权域已经使用 `organization_id` 和组织树计算数据范围。两者不是同一实体，名称相近会导致协议集成和运维排障误用。
- 决策：全局租户/公司身份在 Capability、operations port、JWT、可信 principal、路由和 `tenant_registry` 中统一命名为 `company_id` / `CompanyID`；`organization_id`、`organization_node` 及其 RBAC/共享语义保持不变。
- 兼容与迁移：这是全局开户协议的 v2 破坏性字段变更，旧请求和 JWT `org_id` claim 一律拒绝。migration 13 仅重命名 PostgreSQL 列及约束，保留既有 20 位编号值（其遗留 `org...` 字面格式不在本决策中重键）。
- 原因：用公司身份表达跨产品统一租户目录，用组织身份表达单租户授权范围，使两层边界可读、可验证并避免跨域混淆。
- 后续影响：外部运营控制面、令牌发行方与已部署运行时必须在独立升级任务中切换到 v2；当前本地代码和 schema 已验证，但不得将其声称为已部署 ECS 能力。
- 验证方式：验证旧 claim/input fail closed、fresh 数据库依次执行 migrations 1–13、三入口 schema 只暴露 `company_id`，并回归组织树授权测试。
- 参考资料：
  - `docs/specs/FEAT-025-company-identity-rename.md`
  - `docs/specs/FEAT-011-unified-tenant-operations-control-plane.md`

## 维护规则

- 只记录非平凡技术决策。
- 决策变更时，不删除历史，新增或更新状态。
- 必须写清楚为什么选它，而不只是选了什么。
