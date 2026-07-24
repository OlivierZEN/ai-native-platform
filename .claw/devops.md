---
kind: devops
version: 3
updated_at: 2026-07-24T08:48:00Z
updated_by: integration-agent after Keycloak production baseline deployment
verification_status: passed
---

# 项目部署运维手册

`devops.md` 是构建、运行、部署和运维知识的事实源。

## 源代码管理

- 远端仓库：`https://github.com/OlivierZEN/ai-native-platform.git`
- 默认分支：`main`
- 架构与治理基线已提交至默认分支

## 构建

- 当前发布制品使用 Go 1.26.5 交叉构建：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=go1.26.5 \
  go build -trimpath -ldflags='-s -w' -o semattice ./cmd/ai-native-platform
```

- 2026-07-24 部署制品 SHA-256：`c1617398e9ddf3b83a942fa8b5852e54f7caf943900771703e8b1bacbf712962`。
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

- 当前目标：`115.29.222.70`；域名：`https://semattice.agentcici.com`。
- 当前 release 目录：`/opt/semattice/releases/20260724T094721Z-keycloak-jwks`；当前链接：`/opt/semattice/current`。该 release SHA-256 为 `5647f68d192c1d4be2ebec7a67671b5d513fef06767a17eb877f04ada5d0922d`。
- systemd unit：`/etc/systemd/system/semattice.service`；仓库模板为 `deploy/semattice/semattice.service`。
- Nginx server block：`/etc/nginx/conf.d/semattice.conf`；仓库模板为 `deploy/semattice/nginx.conf`。
- 静态说明：`/var/www/semattice`；TLS：`/etc/semattice/tls`，私钥 mode `0600`。
- 当前静态说明 release：`20260723T0658Z`；发布前的 HTML/CSS/JS 备份位于 `/var/www/semattice-backups/20260723T0658Z`。
- Keycloak 当前 release：`/opt/keycloak/releases/keycloak-26.7.0`，当前链接为 `/opt/keycloak/current`；systemd unit 为 `/etc/systemd/system/keycloak.service`，Nginx vhost 为 `/etc/nginx/conf.d/sso.agentcici.com.conf`。受控安装前备份在 `/root/keycloak-backups/20260724T083102Z-before-keycloak`。
- Keycloak 运行配置在 `/etc/keycloak`，目录为 `root:keycloak 0750`；配置/数据库/bootstrap env 均为 `root:keycloak 0640`，初始管理员凭据文件为 `/root/keycloak-initial-admin.txt`（`0600`）。不得输出或复制这些值；首次管理员登录后应执行受控密码轮换。
- 业务 Realm 为 `agentcici`；已登记 `agentcici-bff`、`semattice-api`、`official-access-context`、`followup-worker`。本次只创建非秘密 client 注册，后续应用接入再按最小权限读取并安全分发所需 secret。
- 更新时先安装新的不可变 release 目录，核对 checksum，再原子切换 `/opt/semattice/current` 并重启 `semattice`。不要覆盖或删除旧 release。
- 回滚时将 `current` 指回前一 release 并重启；数据库 migration 不自动回滚，数据目录不得删除。
- 受控开户生产 smoke：从受信 AgentCiCi 主机向 `POST /internal/v1/company-provisionings` 发送 HMAC 请求。无签名请求必须为 403；对格式合法但不存在的 `company_id`，签名请求应在 AgentCiCi 组织校验后返回 `FAILED_PRECONDITION` / 412，且不创建 tenant、reservation 或 operation。

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
- MCP 是 stdio，不存在远程 HTTP health endpoint；通过受信主机启动 `/usr/local/bin/semattice mcp stdio` 并执行 initialize/tools/list/tool call 验证。
- Keycloak：`GET https://sso.agentcici.com/realms/agentcici/.well-known/openid-configuration` 与 `/protocol/openid-connect/certs` 均应 HTTPS 200；公网 `/health` 和 `/metrics` 必须为 404，本机 `/health/ready` 必须为 UP。
- 官方 OACT：Semattice 环境变量 `AI_NATIVE_IDENTITY_TRUSTED_ISSUERS` 固定为 `official_access|https://x.agentcici.com|semattice-api|https://x.agentcici.com/.well-known/agentcici-oact-jwks.json`；不得接受 Token 自带的 issuer、audience 或 JWKS URL。JWKS 缓存 5 分钟、未知 `kid` 仅刷新一次。

## 维护规则

- 仅记录已验证可用的命令、步骤和配置。
- 若步骤未验证，必须显式标注 `pending verification`。
- 仅在构建、启动、部署或运维知识发生变化时更新。
