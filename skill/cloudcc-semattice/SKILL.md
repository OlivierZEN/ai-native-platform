---
name: cloudcc-semattice
description: 通过统一 HTTPS Capability API 操作 CloudCC Semattice。当 AI 需要发现平台能力，理解对象、字段、关系、记录、租户、用量、授权、组织和共享资源，或者通过 API 查看及修改这些资源时使用。只允许调用已发布的 `/v1/capabilities/{capability-id}/invoke` 接口；禁止依赖 MCP、直连数据库、调用内部租户开通接口，或绕过 OAuth、RBAC、租户隔离、审批、幂等和审计。
---

# CloudCC Semattice（语义格）

只通过 Semattice 统一 HTTPS Capability API 操作平台。使用 `scripts/semattice_api.py` 发送请求，不依赖 MCP，不直接访问数据库，也不把只读演示控制台当作业务接口。

## 版本

- 将根目录 [VERSION](VERSION) 视为技能版本的唯一事实源。
- 使用语义化版本号和对应的 `v<version>` Git 标签升级；不要根据 README 文本猜测版本。

## 核心流程

1. 复述用户目标，明确本地、预发布或生产环境。目标环境不清晰时，不执行写操作。
2. 涉及认证、请求格式、错误处理或生产操作时，读取 [API 调用契约](references/api-contract.md)。
3. 首次连接或怀疑接口变化时，通过 `system.capability.list` 获取线上能力、scope、风险、执行模式及输入 Schema；技能内目录用于规划，线上返回用于调用前最终校验。
4. 根据任务读取 [API 能力目录](references/api-catalog.md) 中对应分类。目录覆盖当前主程序注册的全部 51 项公开 Capability API。
5. 用户询问对象、字段、关系、记录或权限概念，或者写入业务数据前，读取 [资源模型](references/resource-model.md)。
6. 执行组合操作时，读取 [常用操作流程](references/capability-workflows.md)，并按依赖顺序每次调用一个原子能力。
7. 构造最小请求正文。只提交 `request_id`、可选 `idempotency_key` 和 `input`；不要提交 `tenant_id`、`actor` 或 `scopes`，这些身份信息必须由短期 Bearer 令牌绑定。
8. 写操作前先用脚本 `--dry-run` 展示 URL 和正文。只有用户已经授权该操作且环境明确时才实际调用。
9. 检查 HTTP 状态、`status`、稳定错误码、`result` 和 `audit_id`。写操作成功后，使用最小只读 API 回读验证。
10. 报告调用的环境、能力 ID、修改结果、稳定资源标识和 `audit_id`；禁止输出令牌或完整敏感数据。

## API 调用

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

禁止把令牌放在命令行参数、技能文件、仓库或日志中。

## 授权门禁

- 为完成用户请求可以执行必要的能力发现和只读操作。
- 只有用户明确要求修改且环境无歧义时，才执行创建或更新。
- 删除、清除、发布、回滚、租户生命周期、权限、角色、组织合并和共享操作属于高影响操作。调用前展示准确能力 ID、资源目标和主要输入，并取得明确授权；用户在当前请求中已明确授权完全相同操作时无需重复确认。
- 能力要求 `approval_id` 时，只能使用令牌 `approvals` 声明中已经验证的真实审批标识。禁止生成、猜测或替换审批标识。
- `tenant.decommission` 会创建 `pending_approval` 操作，不代表租户已经完成退役。
- 收到 `UNAUTHORIZED` 时报告所需 scope，禁止自行扩大令牌权限或切换身份、公司、租户和环境。
- 禁止通过内部 `POST /internal/v1/company-provisionings` 开通租户；该接口只允许受信 AgentCiCi 服务使用 HMAC 调用。

## 事实边界

- 以当前主程序注册表、Capability Schema 和运行时代码为当前事实；历史 Spec 和演示控制台数据不作为可调用能力依据。
- `tenant.provision` 没有注册为公开 Capability API，不计入 51 项公开能力。
- 管理控制台 `/console/api/*` 返回只读模拟治理数据，不可用于验证真实业务状态。
- `/mcp` 不属于本技能的调用方式，也不需要配置任何 MCP 依赖。
