---
kind: devops
version: 3
updated_at: 2026-08-03T01:13:35Z
updated_by: root after merging local and origin/main release histories
verification_status: passed
---

# 项目部署运维手册

## 2026-08-01 TASK-051 研发身份治理生产基线

- 当前 release 为 `/opt/semattice/releases/20260801T143342Z-web-oidc-20fe64ee83e2`，migration 17 已应用；源码中的私网 JWKS 路由修复为 `4f5580a` 并已推送 GitHub/CodeUp。
- 目标 tenant `cbcb9ad2-1ac1-50b2-a833-605884b566c1` 对应 company `org5nszpgj99jaysxv6y`；活动 metadata `019fbde4-76cf-73d9-b36a-324692b10d05` 固定为 5 objects / 42 fields。
- OACT verifier 的 JWKS 配置使用 `https://semattice.agentcici.com/.well-known/agentcici-oact-jwks.json`，Nginx 将其 301 到固定 AgentCiCi JWKS；服务端仍只信任配置的 issuer/audience/JWKS，不信任 token 自带地址。
- 研发交付部 organization ID 为 `8ed52e19-be8e-492a-bea7-ab1b2adba0b2`；三类 Principal 均有 active primary membership，5 个研发对象授权策略均为 enforced/private。
- Principal status 只能由 HUMAN manager 通过 `identity.principal.set-status` 并携带已验签独立审批 ID 修改；SERVICE 不能自恢复。生产开发者必须保持 active，异常演练后立即恢复并以 CLI 读任务复核。
- 数据库只读验收使用 `ai_native_runtime` 并显式设置 `app.tenant_id` / `app.tenant_bucket`，不得使用 migrator、关闭 RLS 或输出数据库 URL。

`devops.md` 是构建、运行、部署和运维知识的事实源。

## 源代码管理

- GitHub `origin`：`https://github.com/OlivierZEN/ai-native-platform.git`。
- 阿里云 CodeUp `codeup`：`https://codeup.aliyun.com/627b18115b46541dd2ff340e/cloudcc-aidev-projects/cloudcc-semattice.git`。
- CodeUp 发布分支：`main`；首次发布使用当前工作分支的完整 `HEAD` 快照，不改写 GitHub `origin`、本地分支名或 upstream。
- 发布后校验：`git ls-remote codeup refs/heads/main` 必须与本地 `git rev-parse HEAD` 完全相等。

## 构建

- 当前发布制品使用 Go 1.26.5 交叉构建：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=go1.26.5 \
  go build -trimpath -ldflags='-s -w' -o semattice ./cmd/ai-native-platform
```

- 当前部署制品 SHA-256：`f26f18994494365efe030bbe29739967b9c60c704dbfea189e28d8fee5e11528`。
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
- 当前 release 目录：`/opt/semattice/releases/20260801T143342Z-web-oidc-20fe64ee83e2`；当前链接：`/opt/semattice/current`。该 release 包含 migration 17 的受治理 Principal 投影、控制台身份/角色读取和固定 JWKS 验签；旧 release 继续保留为原子回滚点。
- 历史 release `/opt/semattice/releases/20260731T101946Z-web-oidc-36e1c0a32b2e` 包含零字段控制台修复；更早的 `/opt/semattice/releases/20260731T092534Z-web-oidc-34023d0a5598` 包含手动元数据发布确认。当前 `20fe64e` release 不以二者为祖先；合并后的源码须构建新 release 才能同时承载三条交付线。
- 合并后的 `metadata.version.publish` 仍为高风险、异步、要求 `metadata.publish` scope 和非空 `approval_id`，但该能力不再要求手动标识存在于 OACT `approvals` 声明中。服务端在发布事务内记录 `approval_id`、`approval_mode=manual` 和版本 ID；其他审批能力的可信声明校验不变。
- 网站OIDC环境备份为`/etc/semattice/semattice.env.backup.20260731T074537Z-before-web-oidc`；Nginx与静态站使用同一release标识创建发布前备份。Keycloak `semattice-cli` client历史备份仍为`/opt/keycloak/backups/20260731T045751Z-standalone-auth-before-sematttice-auth`。
- `semattice-web`是confidential server-side client。现有Client Secret仅保存于`/etc/semattice/secrets/semattice-web-client-secret`，Secret目录必须为`root:semattice 0750`，文件为`root:semattice 0640`；环境仅以`AI_NATIVE_CONSOLE_OIDC_CLIENT_SECRET_FILE`引用该文件，不得把Secret写入env、日志、仓库或浏览器。
- 网站登录入口为`GET /auth/oidc/login`，callback为`https://semattice.agentcici.com/auth/oidc/callback`。登录使用Authorization Code + S256 PKCE、state和nonce；成功后只创建最长15分钟的`Secure; HttpOnly; SameSite=Lax`签名Session Cookie，不在Cookie中保存Keycloak Token。真实Chrome登录已验证回到`/console/`并显示当前租户和退出按钮。
- Semattice不再配置或调用 AgentCiCi开户/OACT接口。`/v1/auth/token` 固定验证 Keycloak issuer、`semattice-api` audience、JWKS、`azp=semattice-cli` 和唯一 Organization alias，再映射 active `tenant_registry.company_id` 并签发 Semattice短期 OACT。
- `semattice-cli` 必须保持 public、Authorization Code、PKCE S256、`http://127.0.0.1` redirect；`semattice-api-audience` mapper必须唯一且写入 access token，`organization` client scope必须分配。当前OACT allowlist包含51项公开Capability所需的全部26个唯一scope；scope只是入口上限，Principal/RBAC、RLS、审批和审计继续独立执行。TASK-044配置备份为 `/etc/semattice/semattice.env.backup.20260731T052514Z-all-capability-scopes`。
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
- 独立登录生产 smoke：匿名 `POST /v1/auth/token` 必须为401；真实 `semattice-cli` Authorization Code + S256 PKCE登录只向Semattice换取短期OACT，返回scope必须与线上51项公开Capability归并出的26个唯一`required_scope`完全一致，并使用`system.capability.list`与至少一个非发现类只读能力验证。旧 `/internal/v1/company-provisionings` 已删除，不再作为活动发布门禁。

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
