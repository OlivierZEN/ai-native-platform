---
kind: feature-spec
feature_id: FEAT-051
title: Discover the current published metadata bundle
status: completed
owner_role: integration-agent
task_ids: TASK-060
related_decisions: ADR-003, ADR-014
related_issues: ISSUE-005
updated_at: 2026-08-04T07:20:13Z
updated_by: root after production and Skill release verification
---

# FEAT-051 - 当前已发布元数据发现

## 背景与目标

`metadata.version.get` 要求调用方预先持有 `metadata_version_id`。全新登录的 Skill 没有可信版本标识，只能停止并询问用户，无法独立完成“查看当前对象列表”。平台已经通过 `tenant_registry.metadata_version_id` 维护当前已发布元数据指针，记录运行时也以该指针解释当前对象模型。

本次新增一个只读 Capability，让受信租户上下文直接返回当前已发布版本的完整 Bundle，并更新 Skill 形成“登录 → 能力发现 → 当前对象列表”的闭环。

## 范围

### In Scope

- 新增 `metadata.version.get-current`：输入 `{}`，scope 为 `metadata.read`，低风险、同步执行。
- 从当前令牌绑定租户的 `tenant_registry.metadata_version_id` 读取当前版本，并在 TenantContext/RLS 事务中返回与 `metadata.version.get` 相同的 `version / objects / fields / relations` Bundle。
- 没有当前已发布版本时返回稳定 `FAILED_PRECONDITION`，不得枚举或猜测版本 UUID。
- 覆盖当前指针、更新草稿不生效、跨租户隔离、空租户、Capability Schema 和主程序三入口注册测试。
- 更新 `cloudcc-semattice` 工作流和能力目录；从顶层 `fields` 按 `object_id` 归组，不假设字段嵌套在对象中。
- 统一公开能力数量文案，关闭 ISSUE-005；新能力复用现有 scope，不改变默认 scope 集合。

### Out Of Scope

- 不新增 `metadata.version.list`、分页、历史版本搜索或对象专用列表能力。
- 不改变 `metadata.version.get` 的输入或版本语义。
- 不新增数据库表、列或迁移，不使用管理控制台或直连数据库作为 Skill 事实来源。

## 方案设计

- 在 metadata Service 中增加 `GetCurrent`，先绑定受信 TenantContext，再在同一 runtime 事务中读取当前指针和 Bundle。
- 当前指针为空表示租户尚未发布元数据，映射为 `FAILED_PRECONDITION`；指针存在但目标资源缺失继续按现有数据库错误映射失败。
- Capability 输出复用 `Bundle`，ID 独立注册为 `metadata.version.get-current`，避免静默放宽既有 v1 输入契约。
- Skill 首次连接仍先调用 `system.capability.list`；能力存在时以空输入调用。能力不存在时明确报告服务端未升级，不回退到缓存或项目状态文件。

## 接口与数据影响

- 新增公开 Capability，主程序能力总数由 55 增至 56。
- 复用 `metadata.read`，登录默认 scope 数量保持不变。
- 无数据库迁移、无业务数据修改；发布回滚可直接恢复上一平台二进制和 Skill 标签。

## 验收标准

- 当前租户有已发布版本时，空输入返回正确版本及有序对象、字段和关系。
- 更新草稿存在时仍返回 `tenant_registry.metadata_version_id` 指向的已发布版本。
- 无当前版本时返回 `FAILED_PRECONDITION`；跨租户版本不可见。
- API、MCP、CLI 从同一 Registry 投影新能力，`system.capability.list` Schema、scope、风险和执行模式正确。
- 全量 Go test/race、vet、module verify、Skill Python/结构/YAML/目录一致性和项目状态校验通过。
- 生产 `system.capability.list` 发现新能力；全新 Skill 上下文可只用当前登录查询对象列表并保存审计 ID。
- Skill `v1.5.0` 以新不可移动标签发布，远程 VERSION、README、默认分支和本机安装副本一致。

## 风险与回滚

- 指针读取和 Bundle 读取若分属不同事务会产生发布切换竞态；实现必须在同一 TenantContext 事务中完成。
- 返回完整 Bundle 可能大于仅对象列表，但与既有 `metadata.version.get` 一致，受 1 MiB 响应和元数据配额约束；对象专用投影留待真实容量需求。
- 平台能力必须先于 Skill 发布；若生产发布失败，Skill 不发布并保持 v1.4.2。

## 实现进展

- 用户已批准方案与发布顺序；平台 Capability、租户指针读取、错误映射、三入口/隔离测试和 Skill v1.5.0 开发副本已完成。
- 定向 PostgreSQL、全量 PostgreSQL、Go test/race/vet/module/build、Skill 认证 19 项、结构校验和无 Token dry-run 已通过。
- CodeUp `main` 已合并并推送为 `0398ebebe3b97e343a49bb342d07d4d7d61e3226`；生产 release `20260804T071457Z-web-oidc-0398ebebe3b9` 上线，线上发现 56 项能力并以空输入回读当前两个对象。
- 独立 Skill 仓库以提交 `685687e71e41d2e08fd157e8fce9729b8d25c174` 原子发布 `main + v1.5.0`；远程和本机安装副本均验证为 1.5.0。

## 交接说明

- 先查看 `internal/metadata/capabilities.go`、`internal/metadata/service.go`、`internal/record/service.go` 和 `skill/cloudcc-semattice/references/capability-workflows.md`。
- 当前无外部阻塞；生产和 Skill 发布只允许普通快进及新标签，禁止 force push 或移动历史标签。
