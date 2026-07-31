---
kind: issue-list
version: 3
updated_at: 2026-07-31T05:09:53Z
updated_by: root after merging production feature histories
---

# 问题追踪列表

`issue-list.md` 是 bug、阻塞、风险和待跟进事项的唯一事实源。

推荐严重级别：`critical` / `high` / `medium` / `low`  
推荐状态值：`open` / `in_progress` / `blocked` / `fixed` / `verified` / `closed`

## 活跃问题

### ISSUE-001 - 当前生产 release 尚未包含合并后的控制台与登录组合

- severity: `medium`
- status: `open`
- evidence: 当前生产 release `/opt/semattice/releases/20260731T045751Z-standalone-auth` 在远端 TASK-040 真实治理控制台源码合入本地 `main` 前构建；合并后的仓库源码已同时包含 TASK-040 与 TASK-043。
- root_cause: `verified`；两个功能从分叉的本地/远端历史分别发布，生产认证 release 的构建输入不含后来合并的控制台提交。
- next_action: 本次 CodeUp 推送完成后，从同一合并 HEAD 构建新的不可变生产 release；依次验证真实控制台读取、Keycloak PKCE、`/v1/auth/token`、OACT Capability 调用、匿名负例和回滚证据。

## 已解决问题

- 暂无已解决问题。

## 维护规则

- 发现新问题、风险或阻塞时新增条目。
- 只把真实存在的问题写进这里，不写猜测性的“也许有问题”。
- 根因必须区分 `verified` 和 `inferred`。
- 任务依赖和交接说明写进 `task-board.md`，不要混写到这里。
- `current-status.md` 只引用 issue ID，不重复完整内容。
