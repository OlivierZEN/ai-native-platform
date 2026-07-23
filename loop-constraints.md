# L2 Loop Constraints — PostgreSQL、租户与元数据内核

## Allowed paths and actions

- 本仓库 `go.mod`、`go.sum`、`cmd/**`、`internal/**`、`schemas/**`、`migrations/**`、`scripts/**`、`docs/**`、`.claw/**` 和根 Loop 控制文件。
- 使用专用、临时、本地 PostgreSQL 16 Docker 容器执行 schema、迁移、RLS、128 bucket、连接池、路由和集成测试。
- 实现当前仓库的共享身份验证边界、Native 租户投影、持久幂等/审计、元数据模型和 API/MCP/非交互 CLI 投影。
- 只读盘点 Agent CC 仓库的现有接口与身份声明，用版本化 port/adapter 记录差异；不得写入该仓库。

## Denylist and human gates

- 不读取或写入 `.env*`、密钥、凭据或真实租户数据；测试凭据必须是仓库外、临时且仅本地使用。
- 不连接或迁移生产、预生产、共享远程数据库；不修改本机已有 `cici-postgres`、`cloudcc-postgres` 容器。
- 不修改 Agent CC、运营端或其他仓库；不调用真实身份/运营服务。
- 不执行 push、PR、merge、release、module/image publish、部署、CI、HA、备份或恢复演练。
- 请求体和普通 header 不能成为受信租户/主体来源；错误 issuer/audience/algorithm/expiry/scope/租户绑定必须 fail closed。
- 所有租户表强制 RLS；应用角色不得为 owner 或拥有 `BYPASSRLS`；事务结束后连接池不得残留 TenantContext。
- 新直接依赖进入 `go.mod` 前记录精确版本、用途、许可证和传递图；许可不明立即暂停。
- 每项最多三次失败尝试。不得禁用测试、放宽安全检查、伪造数据库证据或复制业务逻辑来通过 parity。
