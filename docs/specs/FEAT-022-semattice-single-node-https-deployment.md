---
kind: feature-spec
feature_id: FEAT-022
title: CloudCC Semattice single-node HTTPS deployment
status: approved
owner_role: release-agent
task_ids: TASK-022
related_decisions: ADR-007, ADR-008, ADR-012
related_issues: none
updated_at: 2026-07-23T07:06:06Z
updated_by: ai after verified capability matrix deployment
---

# FEAT-022 - CloudCC Semattice 单节点 HTTPS 部署

## 目标

将当前工作树的 CloudCC Semattice 部署到用户明确授权的阿里云 ECS `115.29.222.70`，通过 `https://semattice.agentcici.com` 提供：

- 静态产品与调用说明首页；
- 经 Nginx TLS 终止和反向代理的 Capability API；
- 可下载的 Linux amd64 CLI 二进制；
- CLI 与 MCP stdio 的准确配置示例。
- 从运行时 Capability Contract 派生的平台能力介绍与完整能力矩阵。

## 边界

- 本任务只授权目标 ECS，不扩展到其他云资源、生产数据库、Agent CC 或统一运营控制面。
- PostgreSQL、Go API 只监听 `127.0.0.1`；公网只开放 Nginx 的 80/443。
- MCP 当前只支持 stdio，由 MCP 客户端在受信主机启动二进制；不宣称远程 HTTP MCP 已实现。
- CLI 是同一进程内 Capability 入口，不直接充当远程 API 客户端。
- TLS 私钥、JWT HMAC key 和数据库密码只保存在服务器受限配置中，不进入仓库、日志、网页或项目状态。
- 当前为单节点验证部署，不提供 HA、备份恢复、自动续证或生产 SLA 承诺。

## 部署拓扑

```text
Internet
  -> semattice.agentcici.com:443
  -> Nginx
       /                 static usage guide
       /downloads/       Semattice Linux CLI
       /v1/              127.0.0.1:8080 Capability API
  -> Semattice systemd service
  -> PostgreSQL 16 on 127.0.0.1:5432
```

## 安全与运行约束

- Nginx 使用用户提供、覆盖 `*.agentcici.com` 的证书和私钥。
- API 必须验证 `issuer`、`audience`、`HS256`、过期时间、租户、组织、主体和 scopes。
- migrator、control、runtime 使用独立数据库登录身份；应用服务不持有 migrator URL。
- systemd 使用非 root `semattice` 用户、只读系统目录、私有临时目录和最小写路径。
- 服务器配置文件权限必须阻止非服务用户读取凭据。

## 验收标准

- [x] PostgreSQL 16.13 初始化并成功执行全部 12 个内置 migrations。
- [x] control/runtime 登录角色均非 owner、非 superuser、非 CREATEROLE、非 BYPASSRLS。
- [x] `semattice.service` 启动并只监听 `127.0.0.1:8080`。
- [x] Nginx 配置检查通过，80 重定向至 443，证书覆盖目标域名。
- [x] 域名首页返回 200，并包含 API、CLI、MCP stdio 的可复制示例。
- [x] 使用五分钟短期 smoke JWT 经公网 HTTPS 调用 `system.capability.list` 成功并发现 49 项能力。
- [x] 未授权 API 调用返回 401。
- [x] CLI 与 MCP stdio 在服务器使用短期令牌完成能力发现和真实工具调用验证。
- [x] 服务重启后 HTTPS 与 edge health 继续成功。
- [x] 部署命令、服务路径、日志与排障信息写回 `.claw/devops.md`。
- [x] 首页能力介绍覆盖租户、语义元数据、Changeset、记录、授权、共享/组织和能力发现七个域。
- [x] 能力矩阵与已部署 `system.capability.list` 的 49 项发布能力逐项一致，包括 required scope 与风险等级。

## 已部署基线

- 部署时间：2026-07-23（Asia/Shanghai）。
- 应用制品：当前授权工作树的 Linux amd64、CGO-free 二进制；SHA-256 `24fa672c9399e2f60cce4412ed754654141594e76e82b2d253bf309c93b3db59`。
- 运行组件：Alibaba Cloud Linux 4、PostgreSQL 16.13、Nginx 1.30.2、Semattice systemd 服务。
- TLS：用户提供的 `*.agentcici.com` 证书，目标域名校验通过，有效期至 2026-11-24；续证仍为后续运维责任。
- 数据库迁移角色只在迁移事务期间临时获得 `CREATEROLE`，创建运行角色后立即撤销；应用配置中不保存 migrator 凭据。
- 静态说明 release `20260723T0658Z` 新增能力介绍和可展开矩阵；上一版三个静态文件保存在目标机 `/var/www/semattice-backups/20260723T0658Z`。

## 回滚

- 停止并禁用 `semattice.service` 和对应 Nginx server block。
- 保留 PostgreSQL 数据目录与 `/opt/semattice/releases`，不得在回滚中自动删除数据。
- 将 `/opt/semattice/current` 切回前一 release 后重启服务；首次部署没有前一 release 时只停止服务。
