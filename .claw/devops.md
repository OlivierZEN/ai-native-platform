---
kind: devops
version: 3
updated_at: 2026-07-23T07:06:06Z
updated_by: ai after verified capability matrix static release
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

- 2026-07-23 部署制品 SHA-256：`24fa672c9399e2f60cce4412ed754654141594e76e82b2d253bf309c93b3db59`。
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

## 部署与发布

- 当前目标：`115.29.222.70`；域名：`https://semattice.agentcici.com`。
- release 目录：`/opt/semattice/releases/20260723T0535Z`；当前链接：`/opt/semattice/current`。
- systemd unit：`/etc/systemd/system/semattice.service`；仓库模板为 `deploy/semattice/semattice.service`。
- Nginx server block：`/etc/nginx/conf.d/semattice.conf`；仓库模板为 `deploy/semattice/nginx.conf`。
- 静态说明：`/var/www/semattice`；TLS：`/etc/semattice/tls`，私钥 mode `0600`。
- 当前静态说明 release：`20260723T0658Z`；发布前的 HTML/CSS/JS 备份位于 `/var/www/semattice-backups/20260723T0658Z`。
- 更新时先安装新的不可变 release 目录，核对 checksum，再原子切换 `/opt/semattice/current` 并重启 `semattice`。不要覆盖或删除旧 release。
- 回滚时将 `current` 指回前一 release 并重启；数据库 migration 不自动回滚，数据目录不得删除。

## 排障

- 应用日志：`journalctl -u semattice -n 100 --no-pager`。
- PostgreSQL 日志：`journalctl -u postgresql-16 -n 100 --no-pager`。
- Nginx 日志：`journalctl -u nginx -n 100 --no-pager`，并检查 `/var/log/nginx/`。
- 监听检查：`ss -lntp`。预期只有 Nginx 暴露 80/443；PostgreSQL 和 Go API 均为 loopback。
- 服务失败先检查 `systemctl status postgresql-16 semattice nginx --no-pager -l`，随后检查日志；不要输出 secret env。
- 页面正常但 API 502：检查 `semattice` 是否 active 及 `127.0.0.1:8080` 是否监听。
- TLS 异常：检查证书 SAN、到期日、私钥权限与 `nginx -t`。当前证书到期日为 2026-11-24，自动续证未实现。

## 健康检查

- Edge：`GET https://semattice.agentcici.com/healthz`，预期 `{"status":"ok","service":"semattice-edge"}`。
- 首页：`GET https://semattice.agentcici.com/`，预期 HTTP 200。
- HTTP：`GET http://semattice.agentcici.com/`，预期 301 到 HTTPS。
- API 未鉴权：调用 Capability invoke，预期 401。
- 应用真实检查：使用统一身份服务签发的短期 JWT 调用 `POST /v1/capabilities/system.capability.list/invoke`，预期 `status=succeeded`。
- MCP 是 stdio，不存在远程 HTTP health endpoint；通过受信主机启动 `/usr/local/bin/semattice mcp stdio` 并执行 initialize/tools/list/tool call 验证。

## 维护规则

- 仅记录已验证可用的命令、步骤和配置。
- 若步骤未验证，必须显式标注 `pending verification`。
- 仅在构建、启动、部署或运维知识发生变化时更新。
