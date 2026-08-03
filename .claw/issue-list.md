---
kind: issue-list
version: 3
updated_at: 2026-08-03T01:46:01Z
updated_by: root after merged production rollout
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
- verification: 合并修复制品 `20260803T013913Z-web-oidc-df308b1b981f` 已上线，服务、静态契约、正式 Capability API 和零错误日志 smoke 通过；真实 Chrome 刷新后旧 Session 已失效并停在 Keycloak 登录页，待用户登录后执行页面验收再转为 `verified`。

### ISSUE-003 - 人类 CLI 换票 allowlist 未包含新增 Principal 自同步 scope

- severity: `medium`
- status: `open`
- root_cause: `verified`；合并后的线上注册表发布 54 项 Capability、27 个唯一 `required_scope`，但生产 `AI_NATIVE_OACT_ALLOWED_SCOPES` 与当前技能登录默认值仍保留此前 51 项能力对应的 26 个 scope。
- impact: 正式服务主体 OACT、既有治理投影和其他 53 项能力不受影响；通过 `semattice-cli` 人类登录换取的 OACT 不能调用 `identity.principal.sync`。
- next_action: 取得明确权限扩展授权后，同步更新生产 allowlist、技能默认 scope/目录与版本，并以人类 PKCE 登录和 Principal 自同步做正负验收。

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
