# CloudCC Semattice（语义格）

当前版本：[`1.5.0`](VERSION)

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
- 通过 Keycloak Authorization Code + PKCE 完成人工 CLI 登录，默认请求人类 CLI 可用公开 Capability 所需的 scope，并自动为当前公司换取和续发短期 OACT。

本技能的业务操作只调用 `/v1/capabilities/{capability-id}/invoke`；人工登录助手只调用 `/v1/auth/token` 换取短期 OACT。它不使用 MCP，不直连数据库，不调用内部租户开通接口，也不绕过 OAuth、RBAC、租户隔离、审批、幂等或审计。

## 环境要求

- Codex 技能运行环境。
- Python 3.10 或更高版本；辅助脚本只使用 Python 标准库。
- 可访问的 Semattice HTTPS 服务地址。
- 人工登录需要可用的 Keycloak `semattice-cli` public client、Semattice `/v1/auth/token` 和操作系统凭据库（macOS Keychain、Linux Secret Service 或 Windows Credential Manager）；无交互调用可直接提供短期 Bearer Token。

## 安装

安装固定版本：

```bash
git clone \
  --branch v1.5.0 \
  --depth 1 \
  https://github.com/CloudCCAI/cloudcc-semattice.git \
  ~/.codex/skills/cloudcc-semattice
```

安装后在 Codex 中使用 `$cloudcc-semattice` 调用技能。

## 快速开始

安装完成后，在 Codex 中按以下顺序使用 `$cloudcc-semattice`。

### 1. 登录 Semattice

调用技能并输入提示词：

```text
登录
```

Skill 会打开系统浏览器进入统一登录页。用户名、密码和 MFA 只提交给 Keycloak，不会进入 Codex 对话；登录成功后返回 Codex，再执行下一步。如果账号关联多个公司，按 Skill 提示选择目标公司。

### 2. 查看当前对象列表

登录成功后，继续调用技能并输入提示词：

```text
查看当前对象列表。
```

Skill 会复用当前登录，通过 `metadata.version.get-current` 自动发现目标公司当前已发布的元数据版本并返回对象列表；账号关联多个公司时会提示选择公司，目标公司尚未发布元数据版本时会明确报告。该步骤不会创建、修改或删除对象。

## 版本与升级

本项目使用 [Semantic Versioning](https://semver.org/)：

- `MAJOR`：不兼容的技能流程或公开约定变更。
- `MINOR`：向后兼容的能力、工作流或参考资料扩展。
- `PATCH`：向后兼容的修正和文档更新。

根目录 `VERSION` 是版本号的唯一事实源，每个发布版本必须具有完全一致的 Git 标签 `v<version>`。升级到指定版本：

```bash
cd ~/.codex/skills/cloudcc-semattice
git fetch --tags
git checkout v1.5.0
```

`1.0.0` 将技能 ID 和调用名统一为 `cloudcc-semattice`。从 `0.x` 升级时，请安装到新目录并将调用名改为 `$cloudcc-semattice`；确认新技能可用后再移除旧目录。

`1.1.0` 增加产品定位、业务模块场景、设计/实施双模式，以及对象、字段和关系的操作边界。

`1.2.1` 增加 Keycloak Authorization Code + PKCE 人工登录、Organization Scope、系统凭据库 refresh token、当前公司 OACT 换票、自动续期和安全退出。

`1.2.2` 移除 AgentCiCi 应用换票依赖，改为 Keycloak Organization直接映射 Semattice tenant，并由 Semattice `/v1/auth/token` 签发短期 OACT；升级后必须重新登录。

`1.3.0` 将人工登录默认请求扩展为当前51项公开Capability所需的全部26个唯一scope；服务端Principal/RBAC、RLS、独立审批、幂等和审计门禁保持不变。升级后旧登录缓存会被拒绝，必须重新执行 `semattice login`。

`1.4.0` 为内测租户的首个元数据版本增加手动发布确认：`metadata.version.publish` 接受用户明确提供的非空 `approval_id` 并持久审计，不要求该值出现在 OACT 的 `approvals` 声明中。其他高风险能力仍要求可信令牌中的真实审批标识。

`1.4.1` 将快速开始重写为 Codex 提示词流程：先调用 `$cloudcc-semattice` 输入“登录”，登录成功后再输入“查看当前对象列表。”；不再要求普通用户手工配置 Token 或直接运行底层脚本。

`1.4.2` 增加 Windows 人工登录支持：refresh token 保存到当前用户的 Windows Credential Manager，短期会话缓存使用用户的 LocalAppData；Windows 上由 Skill 直接通过 Python 入口运行登录，不依赖 WSL。

`1.5.0` 增加 `metadata.version.get-current` 工作流：全新登录后可从令牌绑定租户自动发现当前已发布元数据版本，并直接列出对象及其字段，不再要求调用方预先知道 `metadata_version_id`；同时将能力目录更新为 56 项，并区分人类 CLI 默认 scope 与服务身份专用 scope。

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
