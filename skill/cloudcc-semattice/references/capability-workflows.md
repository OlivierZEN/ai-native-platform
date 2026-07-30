# 常用操作流程

## 目录

- [发现线上能力](#发现线上能力)
- [查看元数据版本](#查看元数据版本)
- [新建一个对象模型](#新建一个对象模型)
- [操作对象、字段和关系](#操作对象字段和关系)
- [演进已发布元数据](#演进已发布元数据)
- [查询记录](#查询记录)
- [创建并关联记录](#创建并关联记录)
- [更新记录](#更新记录)
- [删除记录](#删除记录)
- [配置角色和权限](#配置角色和权限)
- [配置共享](#配置共享)
- [查看租户和用量](#查看租户和用量)

以下流程全部使用统一 HTTP Capability API。先阅读 [API 调用契约](api-contract.md)，再用 [API 能力目录](api-catalog.md) 核对所需 scope 和输入。

## 发现线上能力

```bash
python3 scripts/semattice_api.py \
  --capability system.capability.list \
  --input '{}'
```

从结果中读取 `required_scope`、`risk_level`、`input_schema`、`idempotency.enabled` 和 `execution`。技能目录与线上结果冲突时停止写操作，以线上注册表和当前仓库代码为准更新技能。

## 查看元数据版本

```bash
python3 scripts/semattice_api.py \
  --capability metadata.version.get \
  --input '{"metadata_version_id":"<uuid>"}'
```

结果包含版本以及有序的对象、字段和关系定义。禁止猜测版本 UUID；必须来自可信的先前结果。

## 新建一个对象模型

按以下顺序执行，每步保留返回的稳定 ID：

1. 调用 `metadata.version.create` 创建草稿。
2. 调用 `metadata.object.upsert` 创建对象。
3. 对每个字段调用 `metadata.field.upsert`。
4. 需要对象关联时调用 `metadata.relation.upsert`。
5. 使用 `metadata.version.get` 回读草稿并核对完整定义。
6. 首次简单发布可在取得独立审批后调用 `metadata.version.publish`。
7. 已有活动版本或记录演进时，改用下方变更集流程。

对象输入示例：

```json
{
  "metadata_version_id": "<draft-version-uuid>",
  "api_name": "customer",
  "label": "客户",
  "description": "企业客户主数据"
}
```

文本字段示例：

```json
{
  "metadata_version_id": "<draft-version-uuid>",
  "object_id": "<customer-object-uuid>",
  "api_name": "name",
  "label": "客户名称",
  "data_type": "text",
  "required": true,
  "indexed": true,
  "constraints": {"min_length": 1, "max_length": 200}
}
```

关系示例：

```json
{
  "metadata_version_id": "<draft-version-uuid>",
  "api_name": "customer",
  "source_object_id": "<contact-object-uuid>",
  "target_object_id": "<customer-object-uuid>",
  "relation_type": "lookup",
  "delete_behavior": "restrict"
}
```

## 操作对象、字段和关系

元数据定义不是普通记录。先确认目标版本是草稿还是已发布，再选择操作路径：

| 资源 | 创建 | 读取 | 修改 | 删除或退役 |
|---|---|---|---|---|
| 元数据版本 | `metadata.version.create` | `metadata.version.get` | 版本本身不可直接修改；修改其中的草稿定义 | 没有版本删除能力；未激活 Changeset 可取消，已发布版本保持不可变 |
| 对象 | 在草稿中调用 `metadata.object.upsert`；省略 `object_id` 时生成稳定 ID | 通过 `metadata.version.get` 随版本读取 | 草稿中携带相同 `object_id` 再次 upsert；已发布后在候选版本中保留稳定 ID 并走 Changeset | 当前没有公开对象删除或退役能力；对象不能从候选版本中消失 |
| 字段 | 在草稿中调用 `metadata.field.upsert` | 通过 `metadata.version.get` 随版本读取 | 草稿中携带相同 `field_id` upsert；已发布后的受支持变更走候选版本和 Changeset | 不能直接删除；按生命周期逐步进入 `deprecated_*`、`hidden`、`purging`、`tombstone`，破坏性清除使用 `metadata.changeset.purge` |
| 关系 | 在草稿中调用 `metadata.relation.upsert` | 通过 `metadata.version.get` 随版本读取 | 草稿中携带相同 `relation_id` upsert；已发布后保持 API 名称、端点和类型身份并走 Changeset | 当前没有公开关系删除或退役能力；关系不能从候选版本中消失 |

### 修改草稿定义

1. 使用可信结果中的 `metadata_version_id` 和稳定资源 ID。
2. 调用对应 `*.upsert`，只修改目标属性，不生成新的资源 ID。
3. 使用 `metadata.version.get` 回读整个版本，确认没有意外改变其他定义。

`upsert` 同时承担草稿内的创建和修改。不要因为用户说“修改对象”就创建第二个同名对象。

### 修改已发布定义

1. 创建下一候选草稿。
2. 使用可信的活动版本 ID 调用 `metadata.version.get`，读取完整的对象、字段和关系定义。
3. 把全部未变和待变定义提交到候选版本，并保持已有资源的稳定 ID；`metadata.version.create` 只创建空草稿，不会自动复制活动版本。
4. 对允许原位演进的属性执行 upsert。
5. 字段换名或换类型时创建新字段，并用 `predecessor_field_id` 指向旧字段；不要改写旧字段身份。
6. 按 [演进已发布元数据](#演进已发布元数据) 执行校验、模拟、审批、回填、覆盖率验证和发布。

对象的 `api_name`、关系的 API 名称/端点/类型，以及既有字段的对象/API 名称/数据类型在已发布后属于稳定身份边界。不要通过猜测输入尝试强行改写。

当前没有元数据版本列表或“获取活动版本”公开 Capability。活动版本 ID 必须来自可信的先前结果或调用方上下文；缺失时停止实施并请求该标识，不要枚举或猜测 UUID。

### 处理“删除对象”请求

当前公开 Capability 没有对象删除或对象退役操作。遇到此类请求时：

1. 明确区分用户要删除的是对象定义，还是某条对象记录。
2. 如果是记录，按 [删除记录](#删除记录) 执行软删除。
3. 如果是字段数据，使用字段生命周期和经审批的 purge/tombstone 流程。
4. 如果确实是对象定义，报告当前能力缺口，不把对象从候选版本中移除，也不伪造未发布的删除能力。

## 演进已发布元数据

1. 创建候选草稿并完成对象、字段和关系修改。
2. 调用 `metadata.changeset.validate`，保存 `changeset_id`、风险和执行计划。
3. 调用 `metadata.changeset.simulate`，向用户展示记录数、预计索引行和破坏性影响。
4. 需要审批时，通过外部治理流程取得真实审批标识，再调用 `metadata.changeset.approve`。
5. 如果 `requires_backfill=true`，重复调用 `metadata.changeset.backfill`，每次使用有界 `batch_size`，直到没有剩余记录。
6. 调用 `metadata.changeset.validate-coverage`。
7. 只有状态满足发布前置条件时，调用 `metadata.changeset.publish`。
8. 调用 `metadata.changeset.get-status` 和 `metadata.version.get` 验证激活结果。

字段清除必须使用 `metadata.changeset.purge`，同时提供真实审批和有界批次。禁止把字段直接从候选版本中消失当作删除方案。

## 查询记录

```bash
python3 scripts/semattice_api.py \
  --capability runtime.record.query \
  --input '{
    "object_api_name":"customer",
    "limit":20,
    "filters":[{"field":"name","op":"prefix","value":"Cloud"}]
  }'
```

只过滤具有活动类型化索引的字段。使用 `next_cursor` 作为下一次请求的 `after`，不要通过不断增大 `limit` 拉取全部数据。

## 创建并关联记录

```bash
python3 scripts/semattice_api.py \
  --capability runtime.record.create \
  --idempotency-key 'idem-contact-alice-v1' \
  --input '{
    "object_api_name":"contact",
    "data":{
      "name":"Alice",
      "customer":"<customer-record-uuid>"
    }
  }'
```

关系值写在 `data` 中关系 API 名称对应的属性下。`lookup` 和 `master_detail` 使用一个 UUID，`many_to_many` 使用 UUID 数组。

## 更新记录

1. 调用 `runtime.record.get` 获取当前 `revision` 和允许查看的数据。
2. 构造只包含目标变化的 `patch`。
3. 使用当前 `revision` 作为 `expected_revision` 调用 `runtime.record.update`。
4. 回读记录确认结果。

```json
{
  "object_api_name": "contact",
  "record_id": "<uuid>",
  "expected_revision": 3,
  "patch": {"phone": "+86..."}
}
```

发生修订冲突时读取最新状态；值有实质差异时询问用户如何协调，禁止盲目覆盖。

## 删除记录

1. 读取记录并展示对象、记录 ID、修订号和关键非敏感标识。
2. 取得明确删除授权。
3. 使用 `runtime.record.delete` 和当前 `expected_revision` 执行软删除。
4. 需要验证时使用 `runtime.record.get` 并显式传 `include_deleted=true`。

传入关系可能阻止删除。禁止自动改删关联记录，也不要承诺 `cascade` 或 `set_null` 已生效。

## 配置角色和权限

推荐顺序：

1. `authorization.permission-set.create`
2. 经审批调用 `authorization.permission-set.grant`
3. `authorization.role.create`
4. 经审批调用 `authorization.role.attach-permission-set`
5. 按对象和动作调用 `authorization.role.set-data-scope`
6. 经职责冲突检查调用 `authorization.role.assign`
7. 使用 `authorization.access.explain` 验证一条具体记录的访问来源

对象授权从 `disabled` 切换为 `enforced` 会改变默认拒绝边界，必须先确认角色、字段权限、数据范围和共享来源已经准备完成。

## 配置共享

- 临时或例外访问：使用 `record.share.grant`；撤销时使用返回的 `share_grant_id`。
- 长期协作主体：使用 `record.team.add-member`。
- 可复用受让者集合：先创建 `authorization.group.create`，再用 `authorization.group.set-membership` 管理成员。
- 按数据组织批量共享：创建 `record.sharing-rule.upsert`，然后重复 `record.sharing-rule.refresh` 直到投影完成；失败时使用 `record.sharing-rule.retry`。

每个高风险调用都必须携带真实 `approval_id`，并在调用后保存 `audit_id` 和资源 ID。

## 查看租户和用量

- `tenant.get-status`：检查生命周期、路由、服务等级和权益。
- `usage.summary.get`：查看累计请求、执行、RU、记录和逻辑字节。
- `usage.timeseries.get`：查看 1 至 2160 小时的趋势。
- `usage.platform-storage.sample`：只供平台观测；返回共享物理表大小，不可作为租户账单或配额数据。

暂停、恢复或更新权益时使用单调递增的 `product_revision` 和唯一 `operation_id`。退役调用只创建待审批请求，后续状态必须通过 `tenant.get-status` 核对。
