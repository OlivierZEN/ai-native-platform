---
kind: decisions
version: 3
updated_at: 2026-07-16T15:34:20Z
updated_by: ai
---

# 技术决策记录

`decisions.md` 是架构和技术选型的唯一事实源。

## 决策索引

| 编号 | 标题 | 状态 | 日期 | 替代/被替代 |
|------|------|------|------|-------------|
| ADR-003 | AI 原生 CRM + PaaS 目标架构基线 | proposed | 2026-07-13 | - |

推荐状态值：`proposed` / `accepted` / `rejected` / `superseded`

## 新增决策时记录

每条 ADR 至少记录以下信息：

- 编号，如 `ADR-001`
- 状态
- 日期
- 背景
- 备选方案
- 最终结论
- 为什么这个方案胜出
- 后续影响
- 验证方式
- 参考资料

## ADR-003 - AI 原生 CRM + PaaS 目标架构基线

- 状态：`proposed`
- 日期：2026-07-13
- 背景：需要从零构建可横向扩展、可安全配置并可由 Agent 受控操作的企业 CRM + PaaS 平台。
- 结论：以逻辑 OneDatabase、物理多 PostgreSQL 分片为租户数据底座；使用显式租户路由与 RLS；以版本化元数据和 Changeset 统一人工与 Agent 配置；以 JSONB 权威记录与按需类型化索引支持自定义对象；将搜索、分析和异步集成从 OLTP 事务面分离。
- 原因：该方案在租户隔离、无 DDL 开户、动态扩展、事务一致性和后续横向扩容间取得可验证的平衡。
- 后续影响：Phase 0 必须验证分区/RLS、记录与索引、Changeset 编译发布，以及沙箱和 outbox 等高风险假设；具体技术栈与组件选型仍待独立 ADR。
- 验证方式：见 `docs/specs/FEAT-009-greenfield-ai-native-crm-platform.md` 的 Phase 0、测试与验收章节。

## 维护规则

- 只记录非平凡技术决策。
- 决策变更时，不删除历史，新增或更新状态。
- 必须写清楚为什么选它，而不只是选了什么。
