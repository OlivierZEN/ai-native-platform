---
kind: feature-spec
feature_id: FEAT-034
title: Semattice 管理中心真实租户治理数据
status: approved
owner_role: fullstack-agent
task_ids: TASK-040
related_decisions: ADR-009, ADR-014
related_issues: none
updated_at: 2026-07-31T01:20:00Z
updated_by: root
---

# FEAT-034 - Semattice 管理中心真实租户治理数据

## 目标

将 Semattice 管理中心由只读内存演示 fixture 改为只读查询当前已验证 OACT 租户的真实治理事实。该变更尤其覆盖已发布研发交付模型，目标租户 `org5nszpgj99jaysxv6y` 必须显示发布版本 1、5 个对象与 37 个有效字段。

## 约束与边界

- 仅使用控制台短时签名会话中已验证的 `tenant_id`、`company_id` 和 `subject` 建立 TenantContext；不接受客户端传入的租户标识。
- 元数据、RBAC、组织和审计读取必须经过现有 runtime RLS；不新增控制台写接口、不写入演示数据、不输出 OACT 或数据库凭据。
- 当前目标租户的 Semattice 本地成员、角色与组织投影均为 0。页面必须说明“尚未投影”，不可再展示 `example.demo` 或虚构治理成员。
- 保留现有管理中心的浅色高密度信息结构；状态标签从“演示环境 / 模拟治理数据”改为“已发布租户数据 / 只读”。

## 数据投影

| 页面 | 真实来源 | 无数据行为 |
| --- | --- | --- |
| 概览 | tenant registry、当前 published metadata、audit event | 显示 0 和明确空态 |
| 成员与角色 | principal projection、role assignment / authorization role | 显示尚未投影，不伪造成员 |
| 组织架构 | organization node / membership | 显示尚未投影，不伪造组织 |
| 对象与字段 | 当前 published metadata version 的 object / active field definition | 显示真实对象、字段、版本、约束及描述 |
| 运行审计 | 当前租户 audit event | 仅显示真实最小审计投影 |
| 系统配置 | 固定运行时安全契约与真实元数据发布状态 | 不显示样例配置值 |

## 验收

1. 所有 `/console/api/*` 数据来自真实数据库读取；fixture、`演示环境`、`模拟治理数据` 和 `example.demo` 不再作为控制台输出。
2. `org5nszpgj99jaysxv6y` 显示研发交付模型 v1、5 个对象、37 个有效字段，且对象目录与字段矩阵同真实数据库一致。
3. 本地成员/角色/组织未投影时显示可理解的空态而非虚构身份数据。
4. 仍保持短期 Cookie、管理 scope、RLS、匿名 401 和错误租户 fail-closed 边界。
5. 定向及全量 Go 测试、静态 JS 检查、生产构建与线上真实租户读验证通过。
