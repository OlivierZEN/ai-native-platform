---
kind: issue-list
version: 3
updated_at: 2026-07-31T10:35:02Z
updated_by: root after deploying the fieldless object console fix
---

# 问题追踪列表

`issue-list.md` 是 bug、阻塞、风险和待跟进事项的唯一事实源。

推荐严重级别：`critical` / `high` / `medium` / `low`  
推荐状态值：`open` / `in_progress` / `blocked` / `fixed` / `verified` / `closed`

## 活跃问题

### ISSUE-002 - 无字段已发布对象导致管理中心对象页崩溃

- severity: `high`
- status: `fixed`
- root_cause: `verified`；reader仅为存在字段的对象填充`fieldsByObject`，零字段对象取得nil slice并编码为JSON `null`，前端直接访问`item.fields.length`。
- impact: 已发布元数据本身正确，但当前租户的“对象与字段”页面无法渲染。
- fix: TASK-050 将后端空集合规范化为`[]`，并为前端与零字段场景补防御和回归测试。
- verification: 修复制品已上线且服务/静态契约smoke通过；待用户完成Keycloak登录后执行真实页面验收，再转为`verified`。

## 已解决问题

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
