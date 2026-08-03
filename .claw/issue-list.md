---
kind: issue-list
version: 3
updated_at: 2026-08-03T05:02:04Z
updated_by: root
---

# 问题追踪列表

`issue-list.md` 是 bug、阻塞、风险和待跟进事项的唯一事实源。

推荐严重级别：`critical` / `high` / `medium` / `low`  
推荐状态值：`open` / `in_progress` / `blocked` / `fixed` / `verified` / `closed`

## 活跃问题

- 暂无活跃问题。

## 已解决问题

### ISSUE-002 - 管理中心成员与角色接口返回 500

- severity: `high`
- status: `closed`
- root_cause: `verified` — `internal/console/reader.go` 的成员聚合查询按未加入 GROUP BY 的 `p.created_at` 排序，PostgreSQL 拒绝执行。
- resolution: TASK-050 将 `p.created_at` 纳入 GROUP BY，新增真实 PostgreSQL reader 回归测试，并发布 release `20260803T045726Z-web-oidc-2b29dc5efb47`。
- evidence: PostgreSQL 16 全量集成通过；生产 HTTPS members=200，返回三名研发主体与三个角色；概览、组织、对象、匿名 401、服务健康和 warning 日志均通过。

### ISSUE-001 - 当前生产 release 尚未包含合并后的控制台与登录组合

- severity: `medium`
- status: `closed`
- resolution: 提交`dcf2b811b7ec88d0685938f6d6564c818ba24314`构建并发布为`/opt/semattice/releases/20260731T074549Z-web-oidc-dcf2b811b7ec`，同一制品同时包含真实治理控制台、Semattice CLI自有登录和网站OIDC登录。
- evidence: 服务、健康、匿名负例、OIDC 303/S256 PKCE参数与真实Chrome登录均通过；登录后回到`/console/`并显示当前租户和退出按钮，浏览器控制台错误数为0。

## 维护规则

- 发现新问题、风险或阻塞时新增条目。
- 只把真实存在的问题写进这里，不写猜测性的“也许有问题”。
- 根因必须区分 `verified` 和 `inferred`。
- 任务依赖和交接说明写进 `task-board.md`，不要混写到这里。
- `current-status.md` 只引用 issue ID，不重复完整内容。
