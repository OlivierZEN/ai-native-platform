---
kind: devops
version: 3
updated_at: 2026-08-04T15:12:00Z
updated_by: root after metering baseline production release
verification_status: passed
---

# 项目部署运维手册

## 2026-08-04 TASK-062 计量基线容错生产发布

- 源提交 `a70142857f7d` 已推送，release `/opt/semattice/releases/20260804T143600Z-metering-a701428` 原子切换；二进制 SHA-256 为 `bb79c61ceae93aa587f7c67a1dc3b27799e7c894f8b90ff5485bc51173c1ff29`，上一 release 保留。
- `semattice` active、`NRestarts=0`，公网 `https://semattice.agentcici.com/healthz` 正常。临时 OSS 制品已删除。
- 目标租户当前用量物化桶已在租户上下文中重算为 41 条 active 记录 / 18,272 逻辑字节；操作先清零现有桶再按对象和 UUID 分桶原子 upsert，active 与 metered 计数一致。

## 2026-08-04 TASK-060 当前元数据发现生产发布

- 生产制品源提交 `0398ebebe3b97e343a49bb342d07d4d7d61e3226` 已普通推送至 CodeUp；当前 release 为 `/opt/semattice/releases/20260804T071457Z-web-oidc-0398ebebe3b9`，二进制 SHA-256 为 `ed58228e1f3f893b387eb5b6a0892a39fe6d5c672423a49134eb57d70fba9c3c`，上一 release 保留为原子回滚点。后续发布证据提交只更新项目状态，不改变该生产二进制。
- 能力注册表为 56 项；`metadata.version.get-current` 复用既有 `metadata.read`，不改变生产 26 项人类 CLI OACT scope allowlist。`identity.principal.sync` 继续是服务身份专用 scope，不纳入人类默认集合。
- 四项服务 active，Semattice `NRestarts=0`、warning 日志为空；edge health、匿名 401、能力发现和真实当前对象回读均通过。

## 2026-08-04 TASK-057 全代码生产发布

- CodeUp `main` 已普通快进至 `26d40f84e55f40863d3c86e081664aecbd63af2c`；GitHub 镜像因 ISSUE-004 的仓库写权限阻塞，仍为 `4bb3d733f5e7297409a813e952faeee8a50eeeec`，未使用 force push 或改写历史。
- 当前 release 为 `/opt/semattice/releases/20260804T050808Z-web-oidc-26d40f84e55f`，二进制 SHA-256 为 `c01284be0160f5f5fcab6c005f5527eee6832bc4896296ea10b4802fd9767c00`；上一成功 release `/opt/semattice/releases/20260804T050411Z-web-oidc-543921635320` 和原组织成员关系 release 均保留。
- 发布脚本现在把 Linux/amd64 二进制和 `.sha256` 清单放入静态归档，并在切换后从公网完整下载验证 SHA；归档清单先完整生成再匹配，避免大归档下 `tar | grep -q` 的 SIGPIPE 误报。
- PostgreSQL、Keycloak、Semattice、Nginx 均 active，Semattice `NRestarts=0`、发布后 warning 日志为 0；edge health、首页、控制台、HTTP 跳转、OIDC、匿名 401、MCP initialize、Keycloak discovery/certs 和下载校验全部通过。

## 2026-08-04 TASK-055 官网 Skill 指南静态发布

- 源码提交 `c74267e` 的 `deploy/semattice/www` 已以静态发布标识 `20260804T035355Z-skill-c74267e` 上线；只切换 `/var/www/semattice`，未重建、切换或重启 Semattice 二进制 release。
- 静态归档 SHA-256 为 `652c9de2887e300ff691fdfe0dd39718b69236003a26ca205062f23dc9371598`；线上 `index.html` / `styles.css` SHA-256 分别为 `748044469d965245fd1562e8edf9157531663fbae62754a9529566ec5f6daa3c` / `66eb4ee1b8f202ede21d3e4f9af364788eb6f1bbba0e36a67b3a3949f92989b1`。
- 切换前完整静态站保留于 `/var/www/semattice-backups/20260804T035355Z-skill-c74267e`；回滚时将当前静态目录移出、把该备份移回 `/var/www/semattice`，再校验并 reload Nginx。
- 公网 `#cli` 已提供官方 GitHub 仓库、`v1.1.0` 固定版本安装命令、`$cloudcc-semattice` 调用示例与凭据安全提醒；桌面/手机 Chrome、复制按钮、GitHub 链接、首页和 edge health 均通过。

## 2026-08-04 TASK-054 组织成员关系能力生产基线

- 当前 release 为 `/opt/semattice/releases/20260804T030035Z-identity-membership-3a0d9daa281a`，二进制 SHA-256 为 `1187b727e05582cba2c4d8b9251895ddcb95da2eef997ef378d7e8136c808e6b`；上一 release `/opt/semattice/releases/20260803T051441Z-web-oidc-2329787b57ff` 保留为回滚点。
- 能力注册表为 55 项；新增 `identity.principal.set-organization-membership` 复用既有 `authorization.manage` scope，不改变生产 26 项 OACT scope allowlist。
- 该能力只允许 HUMAN 管理主体携带已验签独立审批调用；维护一个 active primary membership、结束旧 primary、使权限快照失效并写审计，事务继续受租户 RLS 约束。
- 哪吒已通过该能力加入研发交付部，随后以 `identity.principal.set-status` 设为 `suspended`；这表示“休息中、不可派单”，不是删除或永久 revoke。恢复必须由 HUMAN manager 携带独立审批执行。

## 2026-08-01 TASK-051 研发身份治理生产基线

- 当前 release 为 `/opt/semattice/releases/20260803T051441Z-web-oidc-2329787b57ff`，二进制 SHA-256 为 `bdbd5e9547654c4c1142206b46fc8fa129efc61d72e3519b03b471eee6fd027c`；migration 17 已应用。该统一 release 包含 Principal/JWKS、手工元数据发布、零字段对象和 members 聚合排序修复。
- 目标 tenant `cbcb9ad2-1ac1-50b2-a833-605884b566c1` 对应 company `org5nszpgj99jaysxv6y`；活动 metadata `019fbde4-76cf-73d9-b36a-324692b10d05` 固定为 5 objects / 42 fields。
- OACT verifier 的 JWKS 配置使用 `https://semattice.agentcici.com/.well-known/agentcici-oact-jwks.json`，Nginx 将其 301 到固定 AgentCiCi JWKS；服务端仍只信任配置的 issuer/audience/JWKS，不信任 token 自带地址。
- 研发交付部 organization ID 为 `8ed52e19-be8e-492a-bea7-ab1b2adba0b2`；三类 Principal 均有 active primary membership，5 个研发对象授权策略均为 enforced/private。
- Principal status 只能由 HUMAN manager 通过 `identity.principal.set-status` 并携带已验签独立审批 ID 修改；SERVICE 不能自恢复。执行任务的开发者必须为 active；受控 `suspended` 开发者映射为休息中并禁止领派任务。
- 数据库只读验收使用 `ai_native_runtime` 并显式设置 `app.tenant_id` / `app.tenant_bucket`，不得使用 migrator、关闭 RLS 或输出数据库 URL。

`devops.md` 是构建、运行、部署和运维知识的事实源。

## 源代码管理

- GitHub `github`：`git@github.com:OlivierZEN/ai-native-platform.git`。
- 阿里云 CodeUp `origin`：`git@codeup.aliyun.com:627b18115b46541dd2ff340e/cloudcc-aidev-projects/cloudcc-semattice.git`；本地 `main` 跟踪 `origin/main`。
- 双远端都使用 `main`；只允许普通快进或保留双方历史的真实合并，禁止 force push 或改写历史。
- 发布后校验：`git ls-remote origin refs/heads/main` 与 `git ls-remote github refs/heads/main` 必须同时等于本地 `git rev-parse HEAD`。

## 构建

- 当前发布制品使用 Go 1.26.5 交叉构建：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=go1.26.5 \
  go build -trimpath -ldflags='-s -w' -o semattice ./cmd/ai-native-platform
```

- 当前部署制品 SHA-256：`ed58228e1f3f893b387eb5b6a0892a39fe6d5c672423a49134eb57d70fba9c3c`。
- 公网下载：`https://semattice.agentcici.com/downloads/semattice-linux-amd64`；同目录提供 `.sha256`。

## 启动

- 应用：`systemctl start|stop|restart|status semattice`。
- Nginx：修改配置后先运行 `nginx -t`，再执行 `systemctl reload nginx`。
- PostgreSQL：`systemctl start|stop|restart|status postgresql-16`。
- Semattice 以非 root `semattice` 用户运行，监听 `127.0.0.1:8080`。
- 运行配置：`/etc/semattice/semattice.env`，owner `root:semattice`、mode `0640`。不得在工单、日志或命令输出中打印该文件。

## 依赖服务

- 目标机：Alibaba Cloud Linux 4，8 vCPU、30 GiB 内存。
- PostgreSQL 16.13：源码安装于 `/opt/postgresql/16.13`，数据目录 `/var/lib/pgsql/16/data`，只监听 `127.0.0.1:5432` 与 `/run/postgresql`。
- Nginx 1.30.2：公网监听 80/443。
- 数据库身份：`semattice_migrator`、`ai_native_control`、`ai_native_runtime`。三者均非 superuser、非 BYPASSRLS、非 CREATEROLE；应用只保存 control/runtime URL。
- 新库迁移时 migrator 需要临时 `CREATEROLE` 以创建 migrations 内定义的两个运行角色，迁移成功后必须立即撤销。
- Keycloak 26.7.0：以非特权 `keycloak` 用户运行，JDK 为 Amazon Corretto 21，监听 `127.0.0.1:8180`；管理健康端口仅监听 `127.0.0.1:9000`。其独立 PostgreSQL 数据库和 role 均为 `keycloak`，不得与 Semattice 运行数据库角色或连接串混用。

## 部署与发布

- 2026-07-31 TASK-040 真实租户治理控制台曾发布为 `/opt/semattice/releases/20260731T012059Z-console`。发布脚本交叉编译 Linux amd64 二进制、校验 SHA-256、原子切换 `/opt/semattice/current` 并保留上一 release / 静态站备份。控制台已不再使用内存 fixture；OACT 会话经 runtime RLS 读取真实租户 published metadata、RBAC、组织和审计。该版本线上验证为 active、Nginx valid、edge health 200、匿名治理 API 401；目标研发交付公司读取为 metadata v1 / 5 objects / 37 active fields，本地成员、角色和组织投影均为 0。

- 当前目标：`115.29.222.70`；域名：`https://semattice.agentcici.com`。
- 当前 release 目录：`/opt/semattice/releases/20260804T071457Z-web-oidc-0398ebebe3b9`；当前链接：`/opt/semattice/current`。上一成功 release `/opt/semattice/releases/20260804T050808Z-web-oidc-26d40f84e55f` 与更早 release 均保留为原子回滚点。
- 合并后的 `metadata.version.publish` 仍为高风险、异步、要求 `metadata.publish` scope 和非空 `approval_id`，但该能力不再要求手动标识存在于 OACT `approvals` 声明中。服务端在发布事务内记录 `approval_id`、`approval_mode=manual` 和版本 ID；其他审批能力的可信声明校验不变。
- 网站OIDC环境备份为`/etc/semattice/semattice.env.backup.20260731T074537Z-before-web-oidc`；Nginx与静态站使用同一release标识创建发布前备份。Keycloak `semattice-cli` client历史备份仍为`/opt/keycloak/backups/20260731T045751Z-standalone-auth-before-sematttice-auth`。
- `semattice-web`是confidential server-side client。现有Client Secret仅保存于`/etc/semattice/secrets/semattice-web-client-secret`，Secret目录必须为`root:semattice 0750`，文件为`root:semattice 0640`；环境仅以`AI_NATIVE_CONSOLE_OIDC_CLIENT_SECRET_FILE`引用该文件，不得把Secret写入env、日志、仓库或浏览器。
- 网站登录入口为`GET /auth/oidc/login`，callback为`https://semattice.agentcici.com/auth/oidc/callback`。登录使用Authorization Code + S256 PKCE、state和nonce；成功后只创建最长15分钟的`Secure; HttpOnly; SameSite=Lax`签名Session Cookie，不在Cookie中保存Keycloak Token。真实Chrome登录已验证回到`/console/`并显示当前租户和退出按钮。
- Semattice不再配置或调用 AgentCiCi开户/OACT接口。`/v1/auth/token` 固定验证 Keycloak issuer、`semattice-api` audience、JWKS、`azp=semattice-cli` 和唯一 Organization alias，再映射 active `tenant_registry.company_id` 并签发 Semattice短期 OACT。
- `semattice-cli` 必须保持 public、Authorization Code、PKCE S256、`http://127.0.0.1` redirect；`semattice-api-audience` mapper必须唯一且写入 access token，`organization` client scope必须分配。当前人类 CLI OACT allowlist 为 26 个唯一 scope，覆盖除服务身份专用 `identity.principal.sync` 外的公开 Capability；scope 只是入口上限，Principal/RBAC、RLS、审批和审计继续独立执行。TASK-044 配置备份为 `/etc/semattice/semattice.env.backup.20260731T052514Z-all-capability-scopes`。
- systemd unit：`/etc/systemd/system/semattice.service`；仓库模板为 `deploy/semattice/semattice.service`。
- Nginx server block：`/etc/nginx/conf.d/semattice.conf`；仓库模板为 `deploy/semattice/nginx.conf`。
- Streamable HTTP MCP：Nginx `location = /mcp` 代理至 `127.0.0.1:8080`，必须透传 `Authorization`、将上游 `Host` 固定为 `127.0.0.1`，并关闭 `proxy_buffering`、`proxy_request_buffering`、`proxy_cache`。这使 SDK loopback DNS-rebinding 防护继续有效；当前远程配置备份为 `/etc/nginx/conf.d/semattice.conf.backup.20260724T153300Z` 与 `.backup.20260724T155400Z-mcp-host`。
- 静态说明与控制台：`/var/www/semattice`；TLS：`/etc/semattice/tls`，私钥 mode `0600`。控制台根页和静态资产均以 `Cache-Control: no-store` 发送，HTML 通过带版本的 CSS/JS URL 防止客户端复用旧样式。当前发布前静态站备份位于`/var/www/semattice-backups/20260731T080337Z-web-oidc-ffdbec4fada7`，对应Nginx备份为`/etc/nginx/conf.d/semattice.conf.backup.20260731T080337Z-web-oidc-ffdbec4fada7`。
- 管理中心：`https://semattice.agentcici.com/console/`。`GET /console/session` 无 Cookie 返回 200 的公开 `authenticated:false` 状态；所有 `/console/api/*` 必须为短时签名 Cookie，匿名为 401。`POST /console/session` 仅接收 OACT Bearer，伪造/过期 Token 为 401。顶栏产品菜单回到 `https://x.agentcici.com/admin`，不传递或持久化 OACT。运行环境必须配置独立的 `AI_NATIVE_CONSOLE_SESSION_HMAC_KEY`，不得输出其值。
- Keycloak 当前 release：`/opt/keycloak/releases/keycloak-26.7.0`，当前链接为 `/opt/keycloak/current`；systemd unit 为 `/etc/systemd/system/keycloak.service`，Nginx vhost 为 `/etc/nginx/conf.d/sso.agentcici.com.conf`。受控安装前备份在 `/root/keycloak-backups/20260724T083102Z-before-keycloak`。
- Keycloak 登录主题源码为 `deploy/keycloak/themes/agentcici`；在 Keycloak 主机上以 root 运行 `deploy/keycloak/apply-agentcici-login-theme.sh <theme-source>`。脚本会备份现有主题和 realm 的 `loginTheme` 字段、原子替换 `/opt/keycloak/current/themes/agentcici`、设置 `agentcici` Realm 的 theme/中文 locale 并重启 Keycloak。主题只改变浏览器外观，绝不复制或输出密码、Token、client secret 或数据库配置。
- 当前主题发布备份为 `/opt/keycloak/backups/20260724T120956Z-before-agentcici-login-theme`。当前主题直接携带 AgentCiCi `login_mode2` 所用的 6 张立方体图片和原始旋转/倾斜结构；恢复时停止 Keycloak、将该备份中的 `agentcici` 目录移回 `themes/`，把 realm `loginTheme` 恢复为备份字段（或内置 `keycloak.v2`），再启动并以本机 `/health/ready` 验证。
- Keycloak 运行配置在 `/etc/keycloak`，目录为 `root:keycloak 0750`；配置/数据库/bootstrap env 均为 `root:keycloak 0640`，初始管理员凭据文件为 `/root/keycloak-initial-admin.txt`（`0600`）。不得输出或复制这些值；首次管理员登录后应执行受控密码轮换。
- 业务 Realm 为 `agentcici`；已登记 `agentcici-bff`、`semattice-api`、`official-access-context`、`followup-worker`。本次只创建非秘密 client 注册，后续应用接入再按最小权限读取并安全分发所需 secret。
- 更新时先安装新的不可变 release 目录，核对 checksum，再原子切换 `/opt/semattice/current` 并重启 `semattice`。不要覆盖或删除旧 release。
- 回滚时将 `current` 指回前一 release 并重启；数据库 migration 不自动回滚，数据目录不得删除。
- 独立登录生产 smoke：匿名 `POST /v1/auth/token` 必须为401；真实 `semattice-cli` Authorization Code + S256 PKCE登录只向Semattice换取短期 OACT，返回 scope 必须与 26 项人类 CLI allowlist 一致，并明确排除服务身份专用 `identity.principal.sync`；使用 `system.capability.list` 与至少一个非发现类只读能力验证。旧 `/internal/v1/company-provisionings` 已删除，不再作为活动发布门禁。

## 排障

- 应用日志：`journalctl -u semattice -n 100 --no-pager`。
- PostgreSQL 日志：`journalctl -u postgresql-16 -n 100 --no-pager`。
- Nginx 日志：`journalctl -u nginx -n 100 --no-pager`，并检查 `/var/log/nginx/`。
- 监听检查：`ss -lntp`。预期只有 Nginx 暴露 80/443；PostgreSQL 和 Go API 均为 loopback。
- 服务失败先检查 `systemctl status postgresql-16 semattice nginx --no-pager -l`，随后检查日志；不要输出 secret env。
- 页面正常但 API 502：检查 `semattice` 是否 active 及 `127.0.0.1:8080` 是否监听。
- TLS 异常：检查证书 SAN、到期日、私钥权限与 `nginx -t`。当前证书到期日为 2026-11-24，自动续证未实现。
- Keycloak 故障：先检查 `systemctl status keycloak --no-pager -l`、`journalctl -u keycloak -n 100 --no-pager` 与本机 `curl -fsS http://127.0.0.1:9000/health/ready`；再检查 Nginx。不要把 `/etc/keycloak/*.env` 输出到终端。

## 健康检查

- Edge：`GET https://semattice.agentcici.com/healthz`，预期 `{"status":"ok","service":"semattice-edge"}`。
- 首页：`GET https://semattice.agentcici.com/`，预期 HTTP 200。
- HTTP：`GET http://semattice.agentcici.com/`，预期 301 到 HTTPS。
- API 未鉴权：调用 Capability invoke，预期 401。
- 应用真实检查：使用统一身份服务签发的短期 JWT 调用 `POST /v1/capabilities/system.capability.list/invoke`，预期 `status=succeeded`。
- MCP Streamable HTTP：不带 Bearer 的 `initialize`、`notifications/initialized`、`tools/list` 预期成功；不带 Bearer 的 `tools/call` 预期 401；使用有效短期 Bearer JWT 的客户端执行真实 tool call 验证。MCP stdio 仍可通过受信主机启动 `/usr/local/bin/semattice mcp stdio` 使用。
- Keycloak：`GET https://sso.agentcici.com/realms/agentcici/.well-known/openid-configuration` 与 `/protocol/openid-connect/certs` 均应 HTTPS 200；公网 `/health` 和 `/metrics` 必须为 404，本机 `/health/ready` 必须为 UP。
- 官方 OACT：Semattice 环境变量 `AI_NATIVE_IDENTITY_TRUSTED_ISSUERS` 固定为 `official_access|https://x.agentcici.com|semattice-api|https://semattice.agentcici.com/.well-known/agentcici-oact-jwks.json`；不得接受 Token 自带的 issuer、audience 或 JWKS URL。JWKS 缓存 5 分钟、未知 `kid` 仅刷新一次。

## 维护规则

- 仅记录已验证可用的命令、步骤和配置。
- 若步骤未验证，必须显式标注 `pending verification`。
- 仅在构建、启动、部署或运维知识发生变化时更新。
