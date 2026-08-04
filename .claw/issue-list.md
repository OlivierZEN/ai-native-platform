---
kind: issue-list
version: 3
updated_at: 2026-08-04T05:09:55Z
updated_by: root after GitHub mirror permission check
---

# 问题追踪列表

`issue-list.md` 是 bug、阻塞、风险和待跟进事项的唯一事实源。

推荐严重级别：`critical` / `high` / `medium` / `low`  
推荐状态值：`open` / `in_progress` / `blocked` / `fixed` / `verified` / `closed`

## 活跃问题

### ISSUE-004 - GitHub 项目镜像缺少写权限

- severity: `medium`
- status: `blocked`
- root_cause: `verified` — `OlivierZEN/ai-native-platform` 明确拒绝当前 `androidxhm` 与 CloudCCAI 两个 SSH 身份的写入，GitHub CLI 也没有已登录身份；远端没有独有提交，不存在需要合并的分叉。
- impact: CodeUp `main` 与生产已更新至 `26d40f84e55f40863d3c86e081664aecbd63af2c`，但 GitHub 镜像仍停在 `4bb3d733f5e7297409a813e952faeee8a50eeeec`。
- resolution: 仓库所有者为其中一个现有身份授予写权限后，将 GitHub `main` 普通快进至 CodeUp 当前提交；禁止 force push、替换仓库或使用未授权凭据绕过权限。
- verification: 两次 push 分别返回 `Permission to OlivierZEN/ai-native-platform.git denied to androidxhm` 与 `denied to CloudCCAI`；GitHub `main` 回读保持不变。

### ISSUE-002 - 无字段已发布对象导致管理中心对象页崩溃

- severity: `high`
- status: `fixed`
- root_cause: `verified`；reader仅为存在字段的对象填充`fieldsByObject`，零字段对象取得nil slice并编码为JSON `null`，前端直接访问`item.fields.length`。
- impact: 已发布元数据本身正确，但当前租户的“对象与字段”页面无法渲染。
- fix: TASK-050 将后端空集合规范化为`[]`，并为前端与零字段场景补防御和回归测试。
- verification: 最终统一 release 已上线；HTTPS API 确认 `contact.fields=[]`、`large_backpack` 为 2 字段，服务与静态契约通过。待用户完成 Keycloak 登录后执行真实页面零错误验收，再转为`verified`。

## 已解决问题

### ISSUE-003 - 管理中心成员与角色接口返回 500

- severity: `high`
- status: `closed`
- root_cause: `verified` — `internal/console/reader.go` 的成员聚合查询按未加入 GROUP BY 的 `p.created_at` 排序，PostgreSQL 拒绝执行。
- resolution: TASK-052 将 `p.created_at` 纳入 GROUP BY，新增真实 PostgreSQL reader 回归测试，并将修复与 CodeUp 生产主线合并发布为最终统一 release `20260803T051441Z-web-oidc-2329787b57ff`。
- evidence: PostgreSQL 16 与全量 race 通过；生产 HTTPS members=200，返回三名研发主体与三个角色；概览、组织、对象、匿名 401、服务健康和 warning 日志均通过。

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
