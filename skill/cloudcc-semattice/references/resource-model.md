# 资源模型

## 目录

- [身份与租户资源](#身份与租户资源)
- [元数据版本](#元数据版本)
- [对象](#对象)
- [字段](#字段)
- [关系](#关系)
- [记录](#记录)
- [授权与共享资源](#授权与共享资源)
- [用量资源](#用量资源)

## 身份与租户资源

| 资源 | 含义 | 使用范围 |
|---|---|---|
| 公司 `company_id` | AgentCiCi 侧的企业边界，格式为 `org` 加 17 位小写字母或数字 | 令牌和租户注册表绑定；请求正文不能切换公司 |
| 租户 `tenant_id` | Semattice 中的数据、元数据、授权和用量隔离边界 | 所有业务表通过租户字段、复合外键和 RLS 隔离 |
| 主体 `principal_id` | 发起操作或被授权的人类、服务或组投影 | 角色分配、组成员、团队和共享均引用已投影主体 |
| 组织 `organization_id` | 租户内层级组织节点 | 用于主体归属、记录数据组织范围、共享规则和组织合并 |
| 租户状态 | 路由、服务等级、生命周期、版本号和权益的控制面投影 | `tenant.*` 能力只操作当前令牌绑定的租户 |

公开 API 不提供租户创建能力。租户开通只能由受信 AgentCiCi 服务调用内部 HMAC 接口。

## 元数据版本

元数据版本是对象、字段和关系定义的一致快照。状态包括：

- `draft`：可修改对象、字段和关系。
- `published`：已经发布，定义和快照不可变。
- `retired`：历史已发布版本，仍保持不可变。

新版本先以草稿创建。简单发布可使用 `metadata.version.publish`；涉及已有记录、索引、唯一性或字段演进时，使用变更集验证、模拟、回填、覆盖率验证和发布流程。

## 对象

对象是一个业务实体的版本化结构定义，例如 `customer`、`contact` 或 `order`。对象本身不是业务数据；业务数据由该对象下的记录承载。

对象主要属性：

| 属性 | 含义 |
|---|---|
| `object_id` | 稳定 UUID；省略时由平台生成 UUIDv7 |
| `metadata_version_id` | 对象所属的元数据版本 |
| `api_name` | API 标识，必须匹配 `^[a-z][a-z0-9_]{0,95}$`，同一版本内唯一 |
| `label` | 面向用户的显示名称，必填 |
| `description` | 可选说明 |
| `semantic` | 可选 JSON 对象，用于保存语义标签，不改变核心运行时类型 |

使用范围：对象作为字段、关系、记录、对象授权策略、数据范围和共享规则的共同边界。对象只能在草稿版本修改。

## 字段

字段定义对象记录中一个命名属性的类型、必填性、索引、唯一性、默认值和生命周期。字段 API 名称与对象内的关系 API 名称不能冲突。

### 字段类型

| `data_type` | 接受的 JSON 值 | 格式和规范化 | 当前查询能力 |
|---|---|---|---|
| `text` | 字符串 | 支持 `constraints.min_length`、`constraints.max_length`，按 Unicode 字符计数 | 活动索引支持 `eq`、`prefix` |
| `number` | JSON 数字 | 规范化为可精确解析的十进制数，不接受字符串数字 | 活动索引支持 `eq`、`gt`、`gte`、`lt`、`lte` |
| `boolean` | `true` / `false` | 只接受 JSON 布尔值 | 活动索引只支持 `eq` |
| `date` | 字符串 | 必须为 `YYYY-MM-DD` | 活动索引支持 `eq`、`gt`、`gte`、`lt`、`lte` |
| `datetime` | 字符串 | 必须为 RFC3339，写入后规范化为 UTC | 活动索引支持 `eq`、`gt`、`gte`、`lt`、`lte` |
| `uuid` | 字符串 | 必须为非零 UUID，写入后使用规范格式 | 活动索引只支持 `eq` |
| `json` | 任意合法 JSON 值 | 单字段最大 64 KiB、最大深度 8、单数组最多 1000 项 | 当前不支持类型化索引过滤，也不能设置 `unique_value` |

### 字段控制属性

| 属性 | 含义和约束 |
|---|---|
| `required` | 记录创建和更新后的完整数据中必须存在非空值 |
| `indexed` | 请求维护类型化索引；只有 `index_state=active` 的受支持类型可用于查询过滤 |
| `unique_value` | 对象内唯一；必须同时 `indexed=true`，不能用于 `json` 或 `tombstone` 字段 |
| `default_value` | 创建记录时可写入的默认 JSON 值，必须符合字段类型 |
| `default_semantics` | `on_create` 只影响新记录；`backfill_required` 表示已有记录需要受治理回填 |
| `predecessor_field_id` | 字段演进时指向被替代字段，保持身份链路 |
| `constraints` | 当前运行时只执行文本 `min_length` 和 `max_length`；不要假设任意 JSON 约束都会生效 |
| `semantic` | 语义和分类扩展信息，不替代类型及授权规则 |

字段生命周期：

| 状态 | 可写 | 对普通读取可见 | 用途 |
|---|---:|---:|---|
| `active` | 是 | 是 | 正常字段 |
| `deprecated_read_write` | 是 | 是 | 兼容期字段，仍允许读写 |
| `deprecated_read_only` | 否 | 是 | 只读退役阶段 |
| `hidden` | 否 | 否 | 从普通响应隐藏 |
| `purging` | 否 | 否 | 正在执行受审批的数据清除 |
| `tombstone` | 否 | 否 | 保留名称和历史身份；不能必填或建立索引 |

索引状态包括 `none`、`building`、`validating`、`active`、`failed`、`retiring`。只有 `active` 可供 `runtime.record.query` 使用。

字段和索引数量由租户服务等级策略控制。当前标准策略最多 500 个字段、20 个活动索引；`dedicated-16g` 最多 500 个字段、40 个活动索引；系统绝对索引上限为 50。

## 关系

关系是源对象到目标对象的版本化引用定义。主要属性包括 `relation_id`、`api_name`、`source_object_id`、`target_object_id`、`relation_type`、`delete_behavior` 和可选语义信息。

| `relation_type` | 记录中的写入形式 | 使用范围 |
|---|---|---|
| `lookup` | 一个目标记录 UUID | 普通单值引用 |
| `master_detail` | 一个目标记录 UUID | 当前运行时的写入形态与 `lookup` 相同；不要假设已实现额外所有权语义 |
| `many_to_many` | 目标记录 UUID 数组 | 多值引用；平台去重并为每个目标生成关系边 |

`delete_behavior` 可声明 `restrict`、`cascade`、`set_null`。当前记录删除逻辑只要发现传入引用就阻止删除，实际效果统一接近 `restrict`；不得承诺已经执行级联删除或自动置空。

关系目标必须属于同一租户，定义必须引用同一元数据版本中的对象，写记录时目标记录必须存在且状态为 `active`。关系值保存在源记录 `data` 中，同时投影到 `record_relation` 用于完整性维护。

## 记录

记录是某个已发布对象的业务数据实例。主要输出属性：

- `record_id`：UUIDv7；创建时可选传入合法 UUID。
- `object_id` / `object_api_name`：记录所属对象。
- `metadata_version_id`：记录使用的元数据版本。
- `data`：字段值和关系值组成的 JSON 对象。
- `revision`：乐观锁修订号；更新和删除必须传 `expected_revision`。
- `lifecycle_state`：当前主要为 `active` 或软删除状态。
- `owner_principal_id`、`data_organization_id`：授权判断使用的所有者和数据组织边界。
- 创建者、更新者及时间戳。

单条 `data` 最大 256 KiB。创建和更新会执行字段类型、必填、约束、关系目标、对象/字段权限、唯一值和配额检查，并重建类型化索引和关系边。

查询范围：

- 只查询活动记录。
- `limit` 为 1 至 100；默认值由运行时决定。
- 最多 8 个过滤条件。
- `after` 使用记录 UUID 作为游标。
- 过滤字段必须具有活动类型化索引，多个条件组合为交集。
- 当前没有独立的关系遍历、反向关系列表或任意联表查询能力。

删除为软删除。记录被其他记录引用时会返回 `FAILED_PRECONDITION`；删除源记录会清除它的出向派生关系边。

## 授权与共享资源

| 资源 | 含义 | 使用范围 |
|---|---|---|
| 角色 `role` | 面向业务职责的权限集合入口 | 分配给已投影主体，可设置到期时间和角色冲突 |
| 权限集 `permission_set` | 可复用的原子权限集合 | 先授予 `platform`、`object` 或 `field` 权限，再附加到角色 |
| 角色分配 | 主体与角色的有效关系 | 支持分配、撤销、有效期和职责分离冲突检查 |
| 数据范围 | 角色在某对象和动作上的记录范围 | `own`、`organization`、`organization_descendants`、`assigned_organizations`、`all_tenant`、`conditional` |
| 对象策略 | 是否启用对象/字段/记录 PDP 以及默认记录读取边界 | `enforcement_state`: `disabled` / `enforced`；默认访问：`private` / `read_all` |
| 访问组 | 共享的主体集合 | `manual` 或 `rule`；当前成员 API 操作主体成员关系 |
| 直接共享 | 某条记录对主体或组的例外授权 | 受让者类型 `principal` / `group`；级别 `read` / `update` / `delete` |
| 记录团队 | 某条记录的长期协作主体 | 当前 API 支持添加主体成员及访问级别 |
| 共享规则 | 按数据组织把记录投影给访问组 | 定义后需要分批 `refresh`，失败后可 `retry` |
| 组织合并 | 把源组织的数据和层级迁移到目标组织 | 启动、分批执行、未执行前取消；全程需要审批 |
| 访问解释 | 解释某主体能否对记录执行动作及最小匹配来源 | 只读审计能力，不授予权限 |

`conditional` 数据范围只支持 `condition.equals`，包含 1 至 5 个字段；字段名必须为字母开头的字母数字下划线，值只能是字符串、数字或布尔值，总表达式不超过 4096 字节。

## 用量资源

| 资源 | 含义 | 使用范围 |
|---|---|---|
| 用量汇总 | 请求数、实际执行数、RU 毫单位、活动记录数、逻辑数据字节 | 当前令牌绑定租户的累计视图 |
| 小时序列 | 每小时请求、执行、RU、记录变化和逻辑字节变化 | 查询 1 至 2160 小时，默认 24 小时 |
| 平台物理存储采样 | 共享表的物理存储大小 | 平台级观测，不代表调用租户的独占存储；需要 `usage.platform.read` |
