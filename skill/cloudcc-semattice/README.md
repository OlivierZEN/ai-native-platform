# CloudCC Semattice（语义格）

当前版本：[`1.2.2`](VERSION)

`cloudcc-semattice` 是帮助 AI 理解、设计并通过统一 HTTPS Capability API 安全操作 CloudCC Semattice（语义格）的 Codex 技能。它先说明产品定位、业务模块和资源模型，再在用户授权后执行租户、元数据、记录、用量、授权、组织和共享等受控操作。

## 能力范围

- 说明 Semattice 是什么、解决什么问题，以及各功能模块适用的业务场景。
- 把业务需求设计为对象、字段、关系、记录、权限、数据范围和共享方案。
- 发现线上已发布的 Capability、scope、风险等级和输入 Schema。
- 查看和演进对象、字段、关系及元数据版本。
- 查询、创建、更新和软删除业务记录。
- 查看租户状态与用量信息。
- 配置角色、Permission Set、对象策略、组织、团队和共享规则。
- 使用统一脚本构造请求、执行 dry-run，并检查稳定错误码与审计标识。
- 通过 Keycloak Authorization Code + PKCE 完成人工 CLI 登录，自动为当前公司换取和续发短期 OACT。

本技能的业务操作只调用 `/v1/capabilities/{capability-id}/invoke`；人工登录助手只调用 `/v1/auth/token` 换取短期 OACT。它不使用 MCP，不直连数据库，不调用内部租户开通接口，也不绕过 OAuth、RBAC、租户隔离、审批、幂等或审计。

## 环境要求

- Codex 技能运行环境。
- Python 3.10 或更高版本；辅助脚本只使用 Python 标准库。
- 可访问的 Semattice HTTPS 服务地址。
- 人工登录需要可用的 Keycloak `semattice-cli` public client、Semattice `/v1/auth/token` 和操作系统凭据库；无交互调用可直接提供短期 Bearer Token。

## 安装

安装固定版本：

```bash
git clone \
  --branch v1.2.2 \
  --depth 1 \
  https://github.com/CloudCCAI/cloudcc-semattice.git \
  ~/.codex/skills/cloudcc-semattice
```

安装后在 Codex 中使用 `$cloudcc-semattice` 调用技能。

## 快速开始

人工 CLI 首次使用时登录：

```bash
./scripts/semattice login
```

该命令打开系统浏览器到 Keycloak，使用 Authorization Code + S256 PKCE 回调本机 `127.0.0.1`。Semattice从已验签的 Keycloak Organization声明映射自己的 tenant并签发短期 OACT；用户名、密码和 MFA只提交给 Keycloak，refresh token只保存在 macOS Keychain或 Linux Secret Service。

登录后发现线上能力：

```bash
./scripts/semattice call \
  --capability system.capability.list \
  --input '{}'
```

查看或清除登录状态：

```bash
./scripts/semattice status
./scripts/semattice logout
```

无交互环境可继续配置短期 Token：

```bash
export SEMATTICE_BASE_URL='https://semattice.agentcici.com'
export SEMATTICE_TOKEN='<short-lived-oact>'
```

然后使用兼容调用形式：

```bash
python3 scripts/semattice_api.py \
  --capability system.capability.list \
  --input '{}'
```

写操作应先执行 dry-run：

```bash
python3 scripts/semattice_api.py \
  --capability runtime.record.create \
  --idempotency-key 'idem-contact-alice-v1' \
  --input '{"object_api_name":"contact","data":{"name":"Alice"}}' \
  --dry-run
```

不要把 Token 放入命令行参数、技能文件、Git 仓库或日志。

## 版本与升级

本项目使用 [Semantic Versioning](https://semver.org/)：

- `MAJOR`：不兼容的技能流程或公开约定变更。
- `MINOR`：向后兼容的能力、工作流或参考资料扩展。
- `PATCH`：向后兼容的修正和文档更新。

根目录 `VERSION` 是版本号的唯一事实源，每个发布版本必须具有完全一致的 Git 标签 `v<version>`。升级到指定版本：

```bash
cd ~/.codex/skills/cloudcc-semattice
git fetch --tags
git checkout v1.2.2
```

`1.0.0` 将技能 ID 和调用名统一为 `cloudcc-semattice`。从 `0.x` 升级时，请安装到新目录并将调用名改为 `$cloudcc-semattice`；确认新技能可用后再移除旧目录。

`1.1.0` 增加产品定位、业务模块场景、设计/实施双模式，以及对象、字段和关系的操作边界。

`1.2.1` 增加 Keycloak Authorization Code + PKCE 人工登录、Organization Scope、系统凭据库 refresh token、当前公司 OACT 换票、自动续期和安全退出。

`1.2.2` 移除 AgentCiCi 应用换票依赖，改为 Keycloak Organization直接映射 Semattice tenant，并由 Semattice `/v1/auth/token` 签发短期 OACT；升级后必须重新登录。

## 目录结构

```text
.
├── VERSION
├── SKILL.md
├── agents/openai.yaml
├── references/
│   ├── api-catalog.md
│   ├── api-contract.md
│   ├── authentication.md
│   ├── capability-workflows.md
│   ├── product-guide.md
│   └── resource-model.md
└── scripts/
    ├── semattice
    ├── semattice_api.py
    └── semattice_auth.py
```

详细执行规则见 [`SKILL.md`](SKILL.md)。
