---
name: cloudcc-semattice
description: 理解、设计和通过统一 HTTPS Capability API 操作 CloudCC Semattice，并为人类 CLI 使用 Keycloak Authorization Code + PKCE 登录和续期短期 OACT。当 AI 需要说明 Semattice 的产品定位与业务价值，设计对象、字段、关系、记录、权限或共享方案，发现平台能力，登录平台，或者通过 API 查看及修改租户、元数据、业务数据、用量、授权、组织和共享资源时使用。业务操作只允许调用已发布的 `/v1/capabilities/{capability-id}/invoke`；登录助手只允许调用 `/v1/auth/token`。禁止依赖 MCP、直连数据库、调用内部租户开通接口，或绕过 OAuth、RBAC、租户隔离、审批、幂等和审计。
---

# CloudCC Semattice（语义格）

同时支持理解设计和安全实施 Semattice。设计阶段先解释产品、业务模块和资源模型；业务实施只通过统一 HTTPS Capability API 操作平台，登录只通过专用 `/v1/auth/token` 换取 OACT。使用 `scripts/semattice_api.py` 发送请求，不依赖 MCP，不直接访问数据库，也不把只读演示控制台当作业务接口。

## 版本

- 将根目录 [VERSION](VERSION) 视为技能版本的唯一事实源。
- 使用语义化版本号和对应的 `v<version>` Git 标签升级；不要根据 README 文本猜测版本。

## 工作模式

- **理解与设计**：用户询问 Semattice 是什么、能解决什么问题、业务模块如何组合，或要求设计对象、权限、共享和数据方案时，先读取 [产品定位与业务模块指南](references/product-guide.md)，再按需读取 [资源模型](references/resource-model.md)。说明设计、当前能力和边界，不调用写 API。
- **实施与调用**：用户明确要求查看或修改真实资源时，读取 [API 调用契约](references/api-contract.md)、[API 能力目录](references/api-catalog.md) 和对应的 [常用操作流程](references/capability-workflows.md)，再按门禁执行。
- **设计后实施**：用户同时要求方案和落地时，先输出并确认资源模型；区分“计划创建”与“已经创建”。只有环境明确、写操作已授权且所需 Scope/审批满足时才继续调用。

## 核心流程

1. 复述用户目标，判断是理解设计、实施调用，还是两者都有。
2. 设计任务先从业务问题映射到租户、元数据、记录、授权、共享和用量模块；不要从某个 API 名称反推业务需求。
3. 涉及对象、字段、关系、记录或权限时，读取 [资源模型](references/resource-model.md)，明确资源身份、生命周期和支持边界。
4. 仅要求设计时，输出候选模型、能力边界和实施前置条件，不请求令牌、不声称资源已经创建。
5. 进入实施前，明确本地、预发布或生产环境。目标环境不清晰时，不执行写操作。
6. 涉及人工登录或令牌续期时，读取 [登录与短期 OACT](references/authentication.md)；涉及认证边界、请求格式、错误处理或生产操作时，读取 [API 调用契约](references/api-contract.md)。
7. 首次连接或怀疑接口变化时，通过 `system.capability.list` 获取线上能力、scope、风险、执行模式及输入 Schema；技能内目录用于规划，线上返回用于调用前最终校验。
8. 根据任务读取 [API 能力目录](references/api-catalog.md) 中对应分类。目录覆盖当前主程序注册的全部 56 项公开 Capability API。
9. 执行组合操作时，读取 [常用操作流程](references/capability-workflows.md)，并按依赖顺序每次调用一个原子能力。
10. 构造最小请求正文。只提交 `request_id`、可选 `idempotency_key` 和 `input`；不要提交 `tenant_id`、`actor` 或 `scopes`，这些身份信息必须由短期 Bearer 令牌绑定。
11. 写操作前先用脚本 `--dry-run` 展示 URL 和正文。只有用户已经授权该操作且环境明确时才实际调用。
12. 检查 HTTP 状态、`status`、稳定错误码、`result` 和 `audit_id`。写操作成功后，使用最小只读 API 回读验证。
13. 报告调用的环境、能力 ID、修改结果、稳定资源标识和 `audit_id`；禁止输出令牌或完整敏感数据。

## 登录与 API 调用

人类首次使用 CLI 时，通过 Keycloak Authorization Code + PKCE 登录；脚本不接收系统密码。默认换取的短期 OACT 请求人类 CLI 可用公开 Capability 所需的 26 个唯一 scope；服务身份专用的 `identity.principal.sync` 不在默认集合中。scope 不替代 Principal/RBAC、RLS、审批或审计：

```bash
./scripts/semattice login
```

Windows PowerShell 不使用 WSL；从技能目录通过 Python 入口执行：

```powershell
py -3 .\scripts\semattice_api.py login
```

系统没有 `py` launcher 时改用 `python`。Windows refresh token只进入当前用户的 Credential Manager。

登录脚本使用系统凭据库保护 Keycloak refresh token；Semattice从已验签的 Keycloak Organization映射当前 tenant并签发短期 OACT。详细配置、续期和退出流程见 [登录与短期 OACT](references/authentication.md)。

登录后调用：

```bash
./scripts/semattice call \
  --capability system.capability.list \
  --input '{}'
```

无交互环境也可继续显式注入短期 OACT：

```bash
export SEMATTICE_BASE_URL='https://semattice.agentcici.com'
export SEMATTICE_TOKEN='<short-lived-oact>'

python3 scripts/semattice_api.py \
  --capability system.capability.list \
  --input '{}'
```

执行写操作时提供稳定幂等键：

```bash
python3 scripts/semattice_api.py \
  --capability runtime.record.create \
  --idempotency-key 'idem-contact-alice-v1' \
  --input '{"object_api_name":"contact","data":{"name":"Alice"}}' \
  --dry-run
```

禁止把令牌放在命令行参数、技能文件、仓库或日志中。raw Keycloak token只允许由登录脚本发送到 Semattice `/v1/auth/token`，不得用于 Capability API。

## 授权门禁

- 为完成用户请求可以执行必要的能力发现和只读操作。
- 只有用户明确要求修改且环境无歧义时，才执行创建或更新。
- 删除、清除、发布、回滚、租户生命周期、权限、角色、组织合并和共享操作属于高影响操作。调用前展示准确能力 ID、资源目标和主要输入，并取得明确授权；用户在当前请求中已明确授权完全相同操作时无需重复确认。
- `metadata.version.publish` 是唯一例外：首次直接发布时，用户可明确提供非空的手动 `approval_id`；调用前必须回读并展示整个草稿版本，因为该能力发布的是整版对象、字段和关系。服务端会持久审计该标识，但不要求它存在于令牌 `approvals` 声明中。
- 其他能力要求 `approval_id` 时，只能使用令牌 `approvals` 声明中已经验证的真实审批标识。禁止生成、猜测或替换审批标识。
- `tenant.decommission` 会创建 `pending_approval` 操作，不代表租户已经完成退役。
- 收到 `UNAUTHORIZED` 时报告所需 scope，禁止自行扩大令牌权限或切换身份、公司、租户和环境。
- 禁止尝试通过登录或 Capability请求创建租户；Keycloak Organization必须已经映射到现有 Semattice tenant。

## 事实边界

- 以当前主程序注册表、Capability Schema 和运行时代码为当前事实；历史 Spec 和演示控制台数据不作为可调用能力依据。
- `tenant.provision` 没有注册为公开 Capability API，不计入 56 项公开能力。
- 管理控制台 `/console/api/*` 返回只读模拟治理数据，不可用于验证真实业务状态。
- `/mcp` 不属于本技能的调用方式，也不需要配置任何 MCP 依赖。
