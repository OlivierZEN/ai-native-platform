---
kind: feature-spec
feature_id: FEAT-030
title: Semattice enterprise administration console
status: approved
owner_role: fullstack-agent
task_ids: TASK-032
related_decisions: ADR-009, ADR-012, ADR-014
related_issues: none
updated_at: 2026-07-25T00:00:00Z
updated_by: root under explicit user authorization
---

# FEAT-030 - Semattice 企业管理中心

## 背景与目标

CloudCC Semattice 当前提供 API、MCP、CLI 与公开能力说明页，缺少供企业系统管理员观察治理事实的独立页面。用户已明确要求在 Semattice 工程内交付一套生产级、可点击、只读的企业管理中心，并使用浅色、高密度的定制企业后台风格。

管理中心的目标是让已有当前管理权限的系统管理员快速核验成员与角色、组织架构、对象与字段、运行审计和系统配置。它不用于浏览业务记录，也不提供任何创建、编辑、删除或发布操作。

## 范围

### In Scope

- 在 `https://semattice.agentcici.com/console/` 提供独立管理中心，保留现有公开说明首页。
- 可点击的六个只读页面：概览、成员与角色、组织架构、对象与字段、运行审计、系统配置。
- 为页面提供明确标识为“演示环境”的模拟治理数据；对象与字段页展示对象数量、对象目录、字段类型、必填、唯一、索引、分类、状态和对象检查器。
- 新增浏览器会话交换端点：AgentCiCi 从既有统一认证取得的 OACT 仅经 URL fragment 到达控制台，前端立即 POST 到 Semattice 交换为短时 HttpOnly、Secure、SameSite=Lax Cookie，随后移除 fragment。
- 会话交换与每个控制台数据入口均本地验证 OACT/会话的 tenant、company、subject、过期时间和最小管理 scope；不调用 Keycloak、不在 localStorage/sessionStorage 存储 Token、不把 Token 写入日志。
- 本地构建、浏览器桌面验收、演示数据验证、不可变 release 发布和线上 smoke。

### Out Of Scope

- 不改写 AgentCiCi 的运营管理端、租户生命周期、全局公司目录或任何外部项目业务逻辑。
- 不提供控制台内配置写入、审批、对象设计器、业务数据浏览、知识库或文档管理。
- 不把 Bearer Token 放入 query string、Cookie、localStorage、sessionStorage、审计 payload 或日志。
- 不在本次增加移动端专属布局、SSO 登录客户端或 Keycloak 新 browser flow。

## 用户场景

1. 系统管理员从 AgentCiCi 以 `https://semattice.agentcici.com/console/#oact=<short-lived-token>` 跳转。控制台只在 fragment 中读取 Token，立即交换为 HttpOnly 会话并清空地址栏，随后按已验证的管理 scope 显示当前公司演示治理数据。
2. 管理员进入“对象与字段”，在左侧目录选择对象，在中心字段矩阵检查结构定义，在右侧检查器确认版本、来源、关联对象和审计事实。
3. 管理员进入成员、组织、审计、系统配置页面，仅查看模拟数据和状态。无管理 scope、会话失效或 API 错误时，页面不渲染治理内容。
4. 管理员在 Semattice 顶栏的产品菜单中可返回 `https://x.agentcici.com/admin`。该跳转不交换、储存或传递 OACT；AgentCiCi 原站已建立的统一登录会话继续按原有边界校验。

## 方案设计

### 视觉与交互

用户已确认首版“浅色目录账页”方向：固定左导航、低高度顶栏、窄摘要带、对象目录/字段矩阵/对象检查器三栏结构。暖白画布、蓝灰文字、钴蓝选中线、轻量状态标签和 1px 分隔线构成视觉基线。顶栏产品菜单明确当前 Semattice 管理端，并提供返回 AgentCiCi 管理端的直接入口。页面交互仅包含导航、目录选择、搜索、筛选、分页和审计详情展开。

### 认证与授权

- Go 服务新增 `/console/session`：仅接收 `Authorization: Bearer <OACT>`，用既有 `identity.Verifier` 本地验签并要求至少一个管理 scope：`authorization.manage`、`organization.manage`、`system.manage` 或 `audit.read`。
- 成功后以独立 `AI_NATIVE_CONSOLE_SESSION_HMAC_KEY` 签发最短化的会话 Cookie。Cookie payload 仅含已校验的 tenant UUID、company ID、subject、scope、`exp` 与随机会话 ID；有效期不超过 OACT 剩余时间且上限 15 分钟。Cookie 不含原始 Token。
- `/console/session` 的 GET 用于前端恢复会话；DELETE 用于退出。所有响应使用 `Cache-Control: no-store`。
- 控制台 API 与静态 UI 分离：静态文件可加载但不会包含治理数据；治理演示数据由经会话校验的 `/console/api/*` 返回。未认证返回 401，无管理 scope 返回 403。
- AgentCiCi 当前已具备 OACT 事实源；本任务只定义 Semattice 接收契约，不改动 AgentCiCi 跳转入口。演示验证可使用同一身份服务签发的短期 OACT。

### 演示数据

演示数据只存在于 Semattice 控制台 API 的内存只读 fixture 中，以当前会话的公司为显示归属，不写入生产业务记录或控制库。数据包括 12 个语义对象、约 80 个字段、8 位成员、两层组织、24 条运行审计和 10 项系统配置。所有页面和标签明确标示“演示环境 / 模拟治理数据”。

## 接口与数据影响

| Endpoint | 认证 | 行为 |
| --- | --- | --- |
| `POST /console/session` | OACT Bearer | 交换为短时 HttpOnly 会话；禁止缓存。 |
| `GET /console/session` | 控制台 Cookie | 返回脱敏当前会话信息。 |
| `DELETE /console/session` | 控制台 Cookie | 清除会话 Cookie。 |
| `GET /console/api/overview` | 控制台 Cookie | 返回当前公司演示概览。 |
| `GET /console/api/members` | 控制台 Cookie | 返回成员与角色 fixture。 |
| `GET /console/api/organizations` | 控制台 Cookie | 返回组织树 fixture。 |
| `GET /console/api/objects` | 控制台 Cookie | 返回对象和字段 fixture。 |
| `GET /console/api/audit-events` | 控制台 Cookie | 返回运行审计 fixture。 |
| `GET /console/api/system-settings` | 控制台 Cookie | 返回配置 fixture。 |

不新增数据库 migration，不写入演示数据。生产环境仅需设置独立、长度不少于 32 的 `AI_NATIVE_CONSOLE_SESSION_HMAC_KEY`，不得复用 HMAC 开户密钥、身份验签密钥或任何其他服务密钥。

## 验收标准

- `/console/` 在无会话时显示安全入口，不显示任何治理 fixture。
- 携带有效 OACT fragment 的跳转会在地址栏清空 fragment 后创建 HttpOnly 会话，且 UI 只对具备管理 scope 的主体加载。
- 失效 OACT、错误签名、无 scope、错 Cookie 和到期 Cookie 分别以 401/403 fail closed。
- 六页导航、对象筛选和对象字段检查器可点击、可用键盘访问；无 console error/warning、无横向溢出。
- 页面展示的所有数据均有“演示环境/模拟治理数据”标识，且不写入 PostgreSQL。
- `go test -race ./...`、`go vet ./...`、`go mod verify`、前端静态检查与真实桌面浏览器检查通过。
- 发布前保留线上静态站备份与上一不可变应用 release；Nginx 检查通过后原子切换。线上 `/console/`、登录前边界、会话交换正反例和现有 `/healthz`、首页、API/MCP smoke 均通过。

## 风险与回滚

- 浏览器接入增加会话密钥配置。若缺失或不合法，服务必须启动失败而非匿名开放管理 API。
- AgentCiCi 尚未提供跳转入口时，控制台保持安全入口状态；不以 query Token、默认管理员或公开 fixture 作为临时替代。
- 静态页面发布失败时恢复 `/var/www/semattice-backups/<release>`；应用发布失败时将 `/opt/semattice/current` 原子指回上一 release。不会修改数据库数据。

## 实现进展

- 2026-07-25：用户确认浅色高密度三栏对象目录方向，并授权实现、模拟数据与线上发布。
- 已完成本地：短时 HttpOnly 会话交换、管理 scope 检查、只读 fixture API、六页静态管理端、对象/字段三栏检查器、单元测试、全量 race/vet/module/state 验证、Linux amd64 构建与桌面浏览器验收。
- 已于授权 ECS 发布：`/opt/semattice/releases/20260724T230626Z-console`。脚本在主机本地生成或验证独立会话密钥，校验 Nginx 后原子切换；线上控制台、匿名会话状态、匿名/伪造 OACT 负例、健康检查和浏览器无登录入口均已验证。此前 release、Nginx 和静态站备份可用于回滚。有效管理 OACT 的实际跳转由 AgentCiCi 已登录用户发起，不在发布验收中读取或伪造用户 Token。
