# 常用操作流程

## 目录

- [发现线上能力](#发现线上能力)
- [查看元数据版本](#查看元数据版本)
- [新建一个对象模型](#新建一个对象模型)
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
