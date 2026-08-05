# API 能力目录

## 目录

- [使用说明](#使用说明)
- [系统能力（1）](#系统能力1)
- [租户能力（5）](#租户能力5)
- [元数据能力（27）](#元数据能力27)
- [用量能力（3）](#用量能力3)
- [记录能力（5）](#记录能力5)
- [身份投影能力（4）](#身份投影能力4)
- [角色、权限和对象策略能力（8）](#角色权限和对象策略能力8)
- [直接共享能力（2）](#直接共享能力2)
- [组织合并能力（3）](#组织合并能力3)
- [访问组、团队和共享规则能力（6）](#访问组团队和共享规则能力6)
- [职责冲突和访问解释能力（2）](#职责冲突和访问解释能力2)
- [总数核对](#总数核对)

## 使用说明

当前主程序在数据库模式下注册 67 项公开 Capability API。本目录来自注册表定义和输入 Schema，用于选择能力和准备输入；调用前仍应通过 `system.capability.list` 核对线上版本。

表格中的输入只列能力 `input` 对象。每次 HTTP 请求还必须包含顶层 `request_id`，写操作通常应包含顶层 `idempotency_key`。`高/异步/审批` 表示能力契约要求审批确认。开发阶段的 `metadata.version.publish` 和 `metadata.changeset.approve` 允许显式手工确认；其他输入含 `approval_id` 的能力仍要求该标识存在于可信令牌的 `approvals` 声明中。

## 系统能力（1）

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `system.capability.list` | v1 | 获取全部已发布能力的描述、Schema、scope、风险、幂等和执行策略 | `system.capability.read` | 低/同步 | 无必填；可选 `include_deprecated` |

## 租户能力（5）

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `tenant.get-status` | v2 | 查看当前租户生命周期、路由、服务等级、版本和权益投影 | `tenant.status.read` | 低/同步 | `{}` |
| `tenant.suspend` | v2 | 暂停当前产品访问，不影响其他产品 | `tenant.lifecycle.write` | 中/同步 | 必填 `operation_id`、`product_revision` |
| `tenant.resume` | v2 | 在全局生命周期允许时恢复产品访问 | `tenant.lifecycle.write` | 中/同步 | 必填 `operation_id`、`product_revision` |
| `tenant.update-entitlement` | v2 | 更新版本化服务权益投影 | `tenant.entitlement.write` | 中/同步 | 必填 `operation_id`、`product_revision`、`entitlements` |
| `tenant.decommission` | v2 | 创建租户产品退役请求，状态进入 `pending_approval` | `tenant.decommission` | 高/异步/审批 | 必填 `operation_id`、`product_revision`；当前输入没有 `approval_id`，调用本身不代表完成退役 |

`tenant.provision` 虽然领域服务存在，但主程序显式不把它注册为公开能力；租户开通只能走内部 HMAC 接口，因此不计入本目录。

## 元数据能力（27）

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `metadata.version.create` | v1 | 创建当前租户的下一草稿元数据版本 | `metadata.version.write` | 中/同步 | `{}` |
| `metadata.object.create` | v1 | 在草稿版本创建对象定义；可选指定 `object_id` | `metadata.definition.write` | 中/同步 | 必填 `metadata_version_id`、`api_name`、`label`；可选 `object_id`、`description`、`semantic` |
| `metadata.object.get` | v1 | 按稳定 ID 获取一个对象定义 | `metadata.read` | 低/同步 | 必填 `metadata_version_id`、`object_id` |
| `metadata.object.list` | v1 | 列出一个版本中的全部对象定义 | `metadata.read` | 低/同步 | 必填 `metadata_version_id` |
| `metadata.object.update` | v1 | 完整替换草稿对象的可变属性 | `metadata.definition.write` | 中/同步 | 必填 `metadata_version_id`、`object_id`、`api_name`、`label`；可选 `description`、`semantic` |
| `metadata.object.delete` | v1 | 从草稿删除对象及其字段；存在关系引用时拒绝 | `metadata.definition.write` | 中/同步 | 必填 `metadata_version_id`、`object_id` |
| `metadata.object.upsert` | v1 | 在草稿版本创建或更新对象定义 | `metadata.definition.write` | 中/同步 | 必填 `metadata_version_id`、`api_name`、`label`；可选 `object_id`、`description`、`semantic` |
| `metadata.field.create` | v1 | 在草稿对象中创建字段定义；可选指定 `field_id` | `metadata.definition.write` | 中/同步 | 必填 `metadata_version_id`、`object_id`、`api_name`、`label`、`data_type`；其他字段属性可选 |
| `metadata.field.get` | v1 | 按稳定 ID 获取一个字段定义 | `metadata.read` | 低/同步 | 必填 `metadata_version_id`、`field_id` |
| `metadata.field.list` | v1 | 列出版本中的字段，可按对象过滤 | `metadata.read` | 低/同步 | 必填 `metadata_version_id`；可选 `object_id` |
| `metadata.field.update` | v1 | 完整替换草稿字段的可变属性 | `metadata.definition.write` | 中/同步 | 必填 `metadata_version_id`、`field_id`、`object_id`、`api_name`、`label`、`data_type`；其他字段属性可选 |
| `metadata.field.delete` | v1 | 从草稿删除字段定义 | `metadata.definition.write` | 中/同步 | 必填 `metadata_version_id`、`field_id` |
| `metadata.field.upsert` | v1 | 在草稿版本创建或更新字段定义 | `metadata.definition.write` | 中/同步 | 必填 `metadata_version_id`、`object_id`、`api_name`、`label`、`data_type`；其他字段控制属性均可选 |
| `metadata.relation.upsert` | v1 | 在草稿版本创建或更新对象关系 | `metadata.definition.write` | 中/同步 | 必填 `metadata_version_id`、`api_name`、`source_object_id`、`target_object_id`、`relation_type`、`delete_behavior`；可选 `relation_id`、`description`、`semantic` |
| `metadata.version.publish` | v1 | 使用显式手动确认发布首个不可变元数据快照 | `metadata.publish` | 高/异步/审批 | 必填 `metadata_version_id`、非空手动 `approval_id`；发布整版定义并持久审计 |
| `metadata.version.get` | v1 | 获取一个版本及其有序对象、字段和关系定义 | `metadata.read` | 低/同步 | 必填 `metadata_version_id` |
| `metadata.version.get-current` | v1 | 获取当前租户已发布版本及其有序对象、字段和关系定义 | `metadata.read` | 低/同步 | `{}`；尚无已发布版本时返回 `FAILED_PRECONDITION` |
| `metadata.changeset.validate` | v1 | 校验候选版本、冻结配额快照并生成演进计划 | `metadata.changeset.write` | 中/同步 | 必填 `candidate_metadata_version_id` |
| `metadata.changeset.simulate` | v1 | 查看冻结的影响模拟和执行计划 | `metadata.changeset.read` | 低/同步 | 必填 `changeset_id` |
| `metadata.changeset.get-status` | v1 | 查看变更集状态、覆盖率、错误、审批和激活状态 | `metadata.changeset.read` | 低/同步 | 必填 `changeset_id` |
| `metadata.changeset.approve` | v1 | 使用显式手工确认审批已校验变更集 | `metadata.changeset.approve` | 高/异步/审批 | 必填 `changeset_id`、非空手工 `approval_id`；持久审计确认模式 |
| `metadata.changeset.publish` | v1 | 在演进工作完成后原子激活已审批变更集 | `metadata.changeset.publish` | 高/异步/审批 | 必填 `changeset_id` |
| `metadata.changeset.backfill` | v1 | 分批执行非破坏性的记录和派生状态回填 | `metadata.changeset.execute` | 中/同步 | 必填 `changeset_id`；可选 `batch_size`，1 至 1000 |
| `metadata.changeset.validate-coverage` | v1 | 冻结租户写入并验证记录、索引、唯一性和引用覆盖率 | `metadata.changeset.execute` | 中/同步 | 必填 `changeset_id` |
| `metadata.changeset.purge` | v1 | 分批清除已审批的破坏性字段数据或墓碑 | `metadata.changeset.purge` | 高/异步/审批 | 必填 `changeset_id`、`approval_id`；可选 `batch_size`，1 至 1000 |
| `metadata.changeset.cancel` | v1 | 在激活前取消已校验或已审批的变更集 | `metadata.changeset.write` | 中/同步 | 必填 `changeset_id` |
| `metadata.changeset.rollback` | v1 | 在非破坏性且仍为当前版本时回滚活动指针 | `metadata.changeset.rollback` | 高/异步/审批 | 必填 `changeset_id`、`approval_id` |

## 用量能力（3）

这三项能力当前没有启用顶层 `idempotency_key`，调用时不要提供。

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `usage.summary.get` | v1 | 获取请求、执行、RU、活动记录和逻辑存储汇总 | `usage.read` | 低/同步 | `{}` |
| `usage.timeseries.get` | v1 | 获取按小时聚合的请求、执行、RU 和业务数据增长 | `usage.read` | 低/同步 | 可选 `hours`，1 至 2160，省略时为 24 |
| `usage.platform-storage.sample` | v1 | 采样共享平台物理表大小，并保存采样记录 | `usage.platform.read` | 低/同步 | `{}`；结果不是当前租户独占存储量 |

## 记录能力（5）

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `runtime.record.create` | v1 | 创建经过元数据、权限、关系、唯一性和配额校验的业务记录 | `runtime.record.create` | 中/同步 | 必填 `object_api_name`、`data`；可选 `record_id` |
| `runtime.record.get` | v1 | 按对象 API 名称和记录 UUID 获取一条记录 | `runtime.record.read` | 低/同步 | 必填 `object_api_name`、`record_id`；可选 `include_deleted` |
| `runtime.record.update` | v1 | 使用合并补丁和乐观锁更新记录 | `runtime.record.update` | 中/同步 | 必填 `object_api_name`、`record_id`、`expected_revision`、`patch` |
| `runtime.record.delete` | v1 | 使用乐观锁软删除记录 | `runtime.record.delete` | 中/同步 | 必填 `object_api_name`、`record_id`、`expected_revision` |
| `runtime.record.query` | v1 | 通过有界类型化索引 DSL 查询活动记录 | `runtime.record.read` | 低/同步 | 必填 `object_api_name`；可选 `filters`、`after`、`limit` |

## 身份投影能力（4）

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `identity.principal.sync` | v1 | 将当前已验证 AgentCiCi Principal 投影到租户 | `identity.principal.sync` | 中/同步 | 可选 `display_name`、`public_id`；主体、租户、owner 和 client 只取可信令牌 |
| `identity.principal.list` | v1 | 列出当前租户受治理的 Principal 投影 | `authorization.manage` | 低/同步 | 可选 `principal_type`、`status` |
| `identity.principal.set-status` | v1 | 暂停、恢复或禁用目标 Principal 投影 | `authorization.manage` | 高/异步/审批 | 必填 `principal_id`、`status`、`reason`、`approval_id` |
| `identity.principal.set-organization-membership` | v1 | 建立、更新或结束目标 Principal 的组织成员关系 | `authorization.manage` | 高/异步/审批 | 必填 `principal_id`、`organization_id`、`active`、`primary`、`approval_id` |

## 角色、权限和对象策略能力（8）

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `authorization.role.create` | v1 | 创建租户业务角色 | `authorization.manage` | 中/同步 | 必填 `name`；可选 `role_id`、`description` |
| `authorization.permission-set.create` | v1 | 创建可复用权限集 | `authorization.manage` | 中/同步 | 必填 `name`；可选 `permission_set_id`、`description` |
| `authorization.permission-set.grant` | v1 | 向权限集附加原子允许权限 | `authorization.manage` | 高/异步/审批 | 必填 `permission_set_id`、`resource_type`、`resource_ref`、`action`、`approval_id`；资源类型为 `platform`、`object` 或 `field` |
| `authorization.permission-set.revoke` | v1 | 从权限集撤销一项原子允许权限 | `authorization.manage` | 高/异步/审批 | 输入同 grant；只删除权限集关联，不删除共享原子权限定义 |
| `authorization.role.attach-permission-set` | v1 | 把权限集附加到角色 | `authorization.manage` | 高/异步/审批 | 必填 `role_id`、`permission_set_id`、`approval_id` |
| `authorization.role.assign` | v1 | 经职责冲突检查后向已投影主体分配角色 | `authorization.manage` | 高/异步/审批 | 必填 `principal_id`、`role_id`、`approval_id`；可选 RFC3339 `expires_at` |
| `authorization.role.revoke` | v1 | 结束有效角色分配并使授权快照失效 | `authorization.manage` | 高/异步/审批 | 必填 `principal_id`、`role_id`、`approval_id` |
| `authorization.role.set-data-scope` | v1 | 设置角色对某对象及动作的数据范围 | `authorization.manage` | 高/异步/审批 | 必填 `role_id`、`object_id`、`action`、`scope_type`、`approval_id`；按范围类型可选 `organization_id` 或 `condition` |
| `authorization.object-policy.set` | v1 | 发布对象授权执行策略 | `authorization.manage` | 高/异步/审批 | 必填 `object_id`、`enforcement_state`、`default_record_access`、`approval_id` |

## 直接共享能力（2）

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `record.share.grant` | v1 | 把一条记录直接共享给主体或访问组 | `record.share.manage` | 高/异步/审批 | 必填 `object_id`、`record_id`、`grantee_type`、`grantee_ref`、`access_level`、`approval_id`；可选 `share_grant_id` |
| `record.share.revoke` | v1 | 撤销有效直接共享 | `record.share.manage` | 高/异步/审批 | 必填 `share_grant_id`、`approval_id` |

## 组织合并能力（3）

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `organization.merge.start` | v1 | 启动已审批的非破坏性组织合并 | `organization.manage` | 高/异步/审批 | 必填 `source_organization_id`、`target_organization_id`、`approval_id` |
| `organization.merge.execute` | v1 | 分批处理已启动的组织合并 | `organization.manage` | 高/异步/审批 | 必填 `operation_id`、`approval_id`；可选 `batch_size`，1 至 1000 |
| `organization.merge.cancel` | v1 | 取消尚未开始执行的组织合并 | `organization.manage` | 高/异步/审批 | 必填 `operation_id`、`approval_id` |

## 访问组、团队和共享规则能力（6）

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `authorization.group.create` | v1 | 创建租户访问组 | `authorization.manage` | 中/同步 | 必填 `name`；可选 `group_id`、`group_type`，类型为 `manual` 或 `rule` |
| `authorization.group.set-membership` | v1 | 添加或结束主体的访问组成员关系 | `authorization.manage` | 高/异步/审批 | 必填 `group_id`、`principal_id`、`active`、`approval_id` |
| `record.team.add-member` | v1 | 向记录协作团队添加已投影主体 | `record.share.manage` | 高/异步/审批 | 必填 `object_id`、`record_id`、`principal_id`、`access_level`、`approval_id` |
| `record.sharing-rule.upsert` | v1 | 创建或替换按数据组织到访问组的共享规则 | `record.share.manage` | 高/异步/审批 | 必填 `object_id`、`name`、`data_organization_id`、`grantee_group_id`、`access_level`、`approval_id`；可选 `rule_id` |
| `record.sharing-rule.refresh` | v1 | 分批把共享规则投影为记录到组的共享边 | `record.share.manage` | 高/异步/审批 | 必填 `rule_id`、`approval_id`；可选 `batch_size`，1 至 1000 |
| `record.sharing-rule.retry` | v1 | 重新启动失败的共享规则投影 | `record.share.manage` | 高/异步/审批 | 必填 `rule_id`、`approval_id` |

## 职责冲突和访问解释能力（2）

| 能力 ID | 版本 | 用途 | scope | 风险/执行 | `input` |
|---|---:|---|---|---|---|
| `authorization.role.set-conflict` | v1 | 声明两个角色之间的对称职责分离冲突 | `authorization.manage` | 高/异步/审批 | 必填 `role_id`、`conflicting_role_id`、`approval_id` |
| `authorization.access.explain` | v1 | 解释主体对记录执行动作的最小匹配授权来源 | `authorization.read` | 低/同步 | 必填 `principal_id`、`object_id`、`record_id`、`action`；动作为 `read`、`update` 或 `delete` |

## 总数核对

| 分类 | 数量 |
|---|---:|
| 系统 | 1 |
| 租户 | 5 |
| 元数据 | 27 |
| 用量 | 3 |
| 记录 | 5 |
| 身份投影 | 4 |
| 授权、组织和共享 | 21 |
| 合计 | 67 |
