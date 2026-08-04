---
kind: test-report
version: 3
updated_at: 2026-08-04T06:24:00Z
updated_by: ai
last_run_at: 2026-08-04T06:24:00Z
last_run_status: passed
---

# 测试报告

## 2026-08-04 TASK-055 活动元数据首次权限引导本地验证

- 真实缺口复现：DEV Autopilot 新事件对象已发布，但产品总监首次调用 `authorization.permission-set.grant` 返回 `UNAUTHORIZED`，未产生绕过 API 的数据库写入。
- 修复后 PostgreSQL 用例证明策略管理员可首次授予并绑定当前活动 metadata 中的对象/字段权限；随机对象 UUID 和未持有的平台权限继续返回 `UNAUTHORIZED`，既有防权限升级断言保持通过。
- `./scripts/test-postgres.sh run`：全仓 PostgreSQL 16 测试通过；`GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、`git diff --check` 均通过。
- Linux amd64 CGO-free 构建 SHA-256 为 `1e5d12298cca534e99220944953ec993ce7baedc04b440ae2816e132ac81c24e`。生产发布与租户权限回读待执行。

## 2026-08-04 TASK-054 组织成员关系能力与哪吒生产验收

- 新增正式高风险能力 `identity.principal.set-organization-membership`，要求 `authorization.manage`、HUMAN 管理主体和 OACT 内已验签独立审批；SERVICE、自审、缺失审批和跨租户输入均 fail closed。
- 定向 Principal/main wiring 测试、独立 PostgreSQL 集成测试、`./scripts/test-postgres.sh run`、`GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、状态 validator 与 diff check 全部通过。
- 提交 `3a0d9daa281a` 已发布为 `/opt/semattice/releases/20260804T030035Z-identity-membership-3a0d9daa281a`，二进制 SHA-256 为 `1187b727e05582cba2c4d8b9251895ddcb95da2eef997ef378d7e8136c808e6b`；上一 release 保留。
- 发布后服务 active，公网 edge health 通过，本机未鉴权 Capability 请求返回预期 401；受认证 `system.capability.list` 返回 55 项且包含新能力。
- DEV Autopilot 哪吒 Principal 通过审批 `33f86ce8-6161-4ce0-a35b-60f527940556` 获得开发者角色和研发交付部 active primary membership，最终状态为 `suspended`。
- 受认证治理控制台精确返回 5 名成员；哪吒为 `SERVICE / dev-autopilot-developer-nezha / 开发者 / 研发交付部 / suspended / owner=Oliver`。全过程未输出 Token、client secret、数据库 URL 或私钥。

## 2026-08-03 TASK-053 DEV Autopilot 研发身份花名册生产验收

- AgentCiCi 权威身份回读：Oliver、大乔、悟空、后羿均为 active；后羿为 SERVICE / AUTOMATION，owner 为 Oliver，client_id 为 `dev-autopilot-developer-houyi`。
- Semattice `identity.principal.sync` 成功投影四名 Principal；后羿复用开发者角色 `8cb76dc3-04ed-4371-98da-87cb9ec081e1`，并拥有研发交付部 `8ed52e19-be8e-492a-bea7-ab1b2adba0b2` 的 primary membership。
- 独立审批 `9e5783ea-7713-462f-8388-24b763eca4a0` 已由不同于申请人的组织管理员批准；身份、角色和组织变更均有审计记录。
- 使用真实短期控制台 Session 调用 `/console/api/members` 与 `/console/api/overview`：返回 4 members / 3 roles / 1 organization / 5 objects / 42 fields；四名成员名称、角色、组织、状态和 owner 均与 FEAT-047 一致。
- 使用后羿和悟空各自机器凭据调用 DEV Autopilot CLI `tasks list --human`：均成功列出开发任务；使用大乔产品经理凭据调用同一入口返回退出码 3、`FORBIDDEN`。
- DEV Autopilot 公网 `/devautopilot/api/health` 返回 200、`mode=integrated`、AgentCiCi/Semattice 集成均为 true。全过程未输出 OACT、client secret、私钥或一次性凭据。

## 2026-08-03 TASK-052 成员与角色查询及统一生产发布验证

- 生产只读复现确认原 SQL 因 `p.created_at` 未加入 GROUP BY 而失败；修正后同租户可返回三名研发主体和三个角色。
- 新增真实 PostgreSQL reader 回归测试，覆盖 control 租户解析、runtime TenantContext/RLS、成员聚合结果与角色查询；旧 SQL 会在该测试中返回数据库错误。
- `./scripts/test-postgres.sh run`：PostgreSQL 16 全仓库集成测试通过，`internal/console` 用时 8.781s。
- `GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath`和`git diff --check`：通过。
- 验证构建 SHA-256 为 `08b654e8cfceca3c31b5dc446ed8bd5a6c7b587127f004a75922ff03fc705c68`。
- 提交 `2b29dc5efb47` 已发布为临时验证 release `/opt/semattice/releases/20260803T045726Z-web-oidc-2b29dc5efb47`；线上二进制 SHA-256 为 `36fbd1451d211865599ba6e0b0bcbb0d0de435cd35fddf57505f80e09bbb4749`。
- Semattice、Nginx、PostgreSQL 16、Keycloak 均 active，Nginx 配置有效，健康 200，匿名 members=401，发布后 Semattice warning 日志为空。
- 服务器内使用受保护会话密钥生成未输出、未落盘的 5 分钟租户只读烟测会话；HTTPS members=200，精确返回三名研发主体和三个角色。overview 为 members=3、roles=3、organizations=1、objects=5、fields=42；organizations 和 objects 接口同时通过。
- 发现 CodeUp `main=df308b1` 含 7 个独有生产提交后，普通快进门禁阻止覆盖；已通过 merge 保留手工元数据发布、零字段对象、研发身份、migration 17 checksum 和本次 members 修复。
- 合并后 `./scripts/test-postgres.sh run` 与 `GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1` 全仓通过；`internal/console` 同时覆盖空字段数组和 members 真实数据库查询。
- 合并后 vet、module verify、16 项 Skill Python 测试、Node 语法、release shell 语法、状态 validator、diff check 与 Linux/amd64 CGO-free 构建全部通过；构建 SHA-256 为 `b9e79c0b43645a274fcb69e8f6d844b2e926553fda2d99e722abab6405952425`。
- 合并提交 `2329787b57ff` 已发布为 `/opt/semattice/releases/20260803T051441Z-web-oidc-2329787b57ff`；线上二进制 SHA-256 为 `bdbd5e9547654c4c1142206b46fc8fa129efc61d72e3519b03b471eee6fd027c`，临时 release 仍保留为回滚点。
- 最终线上 members=200，精确返回三名研发主体和三个角色；overview 为 3 members / 3 roles / 1 organization / 5 objects / 42 fields，5 个研发对象的 fields 均为数组。
- 另一已发布元数据租户回读确认 `contact.fields=[]`、`large_backpack` 为 2 字段；Semattice、Nginx、PostgreSQL 16、Keycloak active，匿名 members=401，Nginx 有效且发布后 warning 日志为空。
- CodeUp 从 `df308b1`、GitHub 从 `4e2bc62` 分别普通快进至交付提交 `9644cb634362fe435a3fea280a763610c29cf8b1`，两个远端回读与本地一致；未使用 force push。

## 2026-08-03 origin/main 合并验证

- `origin/main` 的 6 个提交与本地 5 个提交已在本地 `main` 合并；5 个冲突均限于 `.claw` 状态文件，源码 `internal/console/reader.go` 自动合并成功，最终不存在冲突标记。
- 并行历史重复使用的研发身份治理 `TASK-049` 已按时间顺序调整为 `TASK-051`，对应任务卡、FEAT-035、看板、测试与运维引用一致；状态 validator 通过。
- `GOTOOLCHAIN=go1.26.5 go test ./internal/principal ./internal/console ./cmd/ai-native-platform -count=1` 与 `go test -race ./... -count=1` 通过。
- `GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、Linux amd64 CGO-free 构建、`node --check deploy/semattice/www/console/console.js`、`bash -n scripts/release-console.sh` 和 `git diff --cached --check` 全部通过。
- 本次仅完成本地合并与验证；未推送远端、未部署生产。当前生产 `20fe64e` 不包含本地 `34023d0` 与 `36e1c0a`，联合生产验收需等待新的合并 release。

## 2026-07-31 TASK-050 无字段对象控制台修复验证

- 根因回归测试确认nil字段集合经JSON编码为`fields: []`，并确认已有字段原样保留；`GOTOOLCHAIN=go1.26.5 go test ./internal/console -count=1`通过。
- Node行为测试以`fields: null`调用真实`objectPage`函数，确认不抛异常且响应对象被归一为长度0的数组；`node --check deploy/semattice/www/console/console.js`通过。
- `GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`和`go mod verify`全部通过。
- Linux amd64 CGO-free发布构建通过，验证制品SHA-256为`25f9f08d0b92baa9d4ce00b5567f57fcd194e5a87c602711d806d16907ed98d4`；`bash -n scripts/release-console.sh`与`git diff --check`通过。
- 提交`36e1c0a32b2ed0e00755a6b2fd857969868e586c`已原子发布为`/opt/semattice/releases/20260731T101946Z-web-oidc-36e1c0a32b2e`，生产二进制SHA-256为`f26f18994494365efe030bbe29739967b9c60c704dbfea189e28d8fee5e11528`；上一release和静态站/Nginx备份均由发布脚本保留。
- Semattice、Nginx、PostgreSQL 16和Keycloak均为active，Semattice重启计数为0，Nginx配置有效，`/healthz`与`/console/`为200，匿名对象API和匿名换票为401，近10分钟Semattice错误日志为0。
- 线上HTML引用`console.js?v=20260731-03`，线上JavaScript包含`Array.isArray(item.fields)`归一逻辑。真实Chrome旧管理会话已过期且Keycloak前台SSO未保留，已停在官方登录页等待用户登录；未输入或读取凭据，登录后页面验收仍待执行。

## 2026-07-31 生产元数据首版本发布验收

- 用户明确确认发布当前完整草稿中的全部对象和字段。发布前`system.capability.list`确认`metadata.version.publish`仍要求`metadata.publish`scope、高风险、异步、`approval_required=true`以及两个必填输入；能力发现审计标识为`audit:req-88eb9816-fc56-4967-9837-9cd6f0490722`。
- 发布前`tenant.get-status`确认企业`orgx2x8awt02djpp5xdp`、租户`ce85dabd-68be-503d-9d1b-9b63c536fa78`和Native生命周期均为active；`metadata.version.get`以审计标识`audit:req-2f290dc4-9334-4f79-a2e9-9c957a2bdf9a`确认目标仍为序号1草稿、2对象、2字段、0关系。
- 写操作先完成无Token dry-run，实际调用使用稳定幂等键`idem-publish-metadata-v1-manual-20260731`和手动确认标识`manual-draft-v1-user-confirmation-20260731`。`metadata.version.publish`返回`succeeded / published`，发布审计标识为`audit:req-f7b19407-5c65-48b3-8eaa-fdd6369c063b`。
- 不可变快照摘要为`9d56197d04962a3ab6ad60b4610fc4035dd51b2cb6b9f049d719eb88c9953f4f`；包含`contact / 联系人`与`large_backpack / 大书包`两个对象，后者包含`name / 书包名称`和`color / 颜色`两个text字段，零关系。
- 发布后`metadata.version.get`回读审计标识为`audit:req-8e8de8fb-72cf-463f-8936-38587349e4bc`，版本状态为`published`、快照摘要一致，两个字段均为`lifecycle_state=active`、`index_state=active`；随后`tenant.get-status`仍为active，审计标识为`audit:req-7fe4c51f-bd00-4b0d-8f21-eec9d406f3a1`。
- 追加管理中心UI验收时，既有Chrome管理中心Session已过期并跳转统一登录页；未代用户输入账号、密码或其他凭据。真实Capability API发布与回读验证已经完成，用户重新登录管理中心后可查看已发布模型。

## 2026-07-31 TASK-049 手动元数据发布确认生产验收

- 定向PostgreSQL用例验证：不在Principal `approvals`声明中的手动ID可完成首版本发布并在同一事务写入`approval_id`、`approval_mode=manual`和版本ID；缺失/空白ID失败，已有活动版本仍要求Changeset，未验证的Changeset审批仍fail closed。
- 临时`postgres:16`全仓`go test ./... -count=1`、全量`go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、16项Skill Python测试、官方Skill校验和无Token发布dry-run全部通过；临时数据库容器已清理。
- 实现提交`34023d0a55981761ed0642809b82a5f5b2f7db9f`构建并原子发布为`/opt/semattice/releases/20260731T092534Z-web-oidc-34023d0a5598`，二进制SHA-256为`6a24e4434b4eb97157d30e4284c0537147d3f8032ad2f1d10cd4e8a920a721f3`；上一release`20260731T080337Z-web-oidc-ffdbec4fada7`保留。
- Semattice、Nginx、PostgreSQL 16和Keycloak均为active，Semattice重启计数为0，Nginx配置有效，发布后Semattice错误日志为0；边缘健康和首页为200，匿名治理API与匿名换票均为401。
- 线上`system.capability.list`返回`succeeded`和51项能力；`metadata.version.publish`描述已更新为显式手动确认，仍要求`metadata.publish`、高风险、异步、`approval_required=true`，输入继续必填`metadata_version_id`与`approval_id`。
- 空白`approval_id`实际负例返回`FAILED_PRECONDITION / a manual approval id is required`，审计标识为`audit:req-7e8ae6fc-bb30-4de9-9fe9-a45f5d60ffdd`，没有改变草稿。
- Skill开发副本经预览后同步到本机安装目录，两边逐文件一致、官方校验通过且版本均为`1.4.0`；未同步或发布独立GitHub Skill仓库。
- 最终`metadata.version.get`回读审计标识为`audit:req-8323d1a2-83d5-4e51-a735-55ca06534d86`，草稿`019fb736-8c34-7f0c-a0e8-82f385ffd9b0`仍为`draft`，包含`contact`和`large_backpack`两个对象、后者两个字段、零关系。由于原发布授权发生在只有联系人对象时，本轮没有扩大授权并发布整版。

## 2026-07-31 生产大书包草稿对象验证

- 当前CLI会话为`authenticated`，绑定企业`orgx2x8awt02djpp5xdp`和生产地址`https://semattice.agentcici.com`；未在命令、输出或状态文件中暴露Token。
- `system.capability.list`返回`succeeded`、51项能力和26个唯一scope；线上Schema确认`metadata.object.upsert`和`metadata.field.upsert`均为中风险、同步、无需审批，要求`metadata.definition.write`。
- `metadata.version.get`先回读可信草稿`019fb736-8c34-7f0c-a0e8-82f385ffd9b0`为`draft`，当时仅含`contact / 联系人`对象；审计标识为`audit:req-c6017c92-025a-42f1-91ae-4a1d1ec05fa0`。
- 对象和两项字段写入均先完成无Token dry-run并使用稳定幂等键。`metadata.object.upsert`创建`large_backpack / 大书包`对象`019fb75e-29db-7726-8ff0-8c5033ae08d8`，审计标识为`audit:req-89a97093-2e38-4bb5-b24a-ac8181a1ca62`。
- `metadata.field.upsert`创建必填、索引的`text`字段`name / 书包名称`，字段ID为`019fb75e-d8fb-70ef-9275-1c90a31dd3b1`，审计标识为`audit:req-ef84b628-547f-44c6-b9c5-32f66ac70e52`；约束为1至200个Unicode字符。
- `metadata.field.upsert`创建可选、索引的`text`字段`color / 颜色`，字段ID为`019fb75e-dbc9-7576-9889-e0a67ad3902c`，审计标识为`audit:req-45f00d9d-a0f8-465f-a82f-e9297bce81ff`；约束为1至100个Unicode字符。
- 最终`metadata.version.get`回读返回`succeeded`，审计标识为`audit:req-378fd751-00f7-4762-9b8a-d9adc9f46dd1`；草稿为2对象、2字段、0关系，两个新字段均为`active`且索引状态为`active`。未调用发布、记录、权限、共享或其他高影响写能力。

## 2026-08-01 TASK-051 DEV Autopilot 研发身份与 PDP 生产验收

- `GOTOOLCHAIN=go1.26.5 go test ./internal/principal ./internal/console ./cmd/ai-native-platform -count=1`、项目状态 validator 与 `git diff --check` 均通过。
- migration 17 已应用，生产 release `/opt/semattice/releases/20260801T143342Z-web-oidc-20fe64ee83e2` active；`https://semattice.agentcici.com/healthz` 返回 200，Nginx 配置有效。
- 生产 runtime RLS 回读为 3 个 active Principal：产品总监 HUMAN、产品经理 SERVICE、开发者 SERVICE；两台 SERVICE 的 owner 均为产品总监 account `25deaf62-73c7-40cc-a107-99c56cff2ec9`，display/public/client ID 未被 CLI 同步覆盖。
- 活动 metadata 精确为 5 objects / 42 fields；3 个 active role assignment、3 个 primary organization membership、5 个 `enforced/private` 对象策略均存在。
- 正向：产品经理可创建项目/需求/任务/变更；开发者 CLI 可读取、认领、回报进度与 2.5h 工时并完成任务。负向：开发者直接创建项目和调用 `identity.principal.list` 均为 HTTP 403。
- 生命周期：独立审批 `f1591286-71bb-49ed-b874-80a7c7640fa9` 下，`identity.principal.set-status` 的 suspended/active 各成功一次；suspended 时 `identity.principal.sync` fail closed，管理员恢复后 CLI 成功。
- 审计回读：产品经理/开发者 actor 的 `identity.principal.sync=12`、`runtime.record.create=5`、`runtime.record.get=11`、`runtime.record.query=2`、`runtime.record.update=5`，均为 succeeded；未输出 OACT、secret 或私钥。

## 2026-07-31 CodeUp main发布验证

- 推送前工作树clean，`origin/main`是本地HEAD的祖先，左右提交计数为`0 / 5`；待发布提交依次为`dcf2b81`、`a1ad1d9`、`ffdbec4`、`55883c2`和`63484d3`。
- `git push origin main`以普通快进将CodeUp `main`从`27020e3`更新至`63484d36a5ffeaadd5162f4d9d13481cfe8ac99c`，未使用force push。
- 推送后本地HEAD、`origin/main`和`git ls-remote origin refs/heads/main`均为`63484d36a5ffeaadd5162f4d9d13481cfe8ac99c`。

## 2026-07-31 生产联系人草稿对象验证

- `system.capability.list`返回`succeeded`，线上共51项能力；`metadata.version.create`、`metadata.object.upsert`和`metadata.version.get`的scope、风险、同步执行、幂等与输入Schema符合Skill目录。
- `tenant.get-status`确认当前OACT绑定企业`orgx2x8awt02djpp5xdp`、租户`ce85dabd-68be-503d-9d1b-9b63c536fa78`，产品状态为`active`、服务等级为`standard`。
- 两项写能力均先完成无Token dry-run，正文不含`tenant_id`、`company_id`、actor或scopes；实际调用使用稳定幂等键。
- `metadata.version.create`创建序号1草稿`019fb736-8c34-7f0c-a0e8-82f385ffd9b0`，审计标识为`audit:req-4ef906ce-cc25-45f8-a007-51576a0aed82`。
- `metadata.object.upsert`创建`contact / 联系人`对象`019fb736-c3cb-7e1b-8f98-b93614102672`，审计标识为`audit:req-67b3d1c2-a715-4f3a-993e-f72338e4add7`。
- `metadata.version.get`回读返回`succeeded`，审计标识为`audit:req-1898aa6a-2f6d-42fd-b43e-933312ecf7de`；草稿恰好包含该对象、0字段、0关系。未调用发布、记录或授权写能力，未输出Token或其他凭据。

## 2026-07-31 TASK-048 控制台租户名称生产验收

- 发布前`GOTOOLCHAIN=go1.26.5 go test ./... -count=1`、`go vet ./...`、`go mod verify`、JavaScript、HTML、发布脚本、租户名称绑定和`git diff --check`均通过。
- 提交`ffdbec4fada7aa0169d75dd785bac8607cf927b8`构建为Linux amd64制品，SHA-256为`0e31e75dc59487c6bdf02a9eed169f826fa918d2427e3373a304c2584d8f57f0`，发布目录为`/opt/semattice/releases/20260731T080337Z-web-oidc-ffdbec4fada7`。
- Semattice、Nginx、PostgreSQL 16和Keycloak均active，Semattice重启计数为0，Nginx配置有效，发布后Semattice错误日志为空；健康200、匿名治理API 401、OIDC登录303。
- 生产静态契约确认存在`#tenant-name`和`data.tenant_name`绑定，不再包含`session-subject`。
- 真实Chrome复用有效登录Session打开`/console/`，`#tenant-name`精确显示“应用开发组织”，页面保持企业管理中心治理视图，浏览器控制台错误数为0。
- 上一release`20260731T074549Z-web-oidc-dcf2b811b7ec`、Nginx备份和静态站备份均存在，回滚证据通过。

## 2026-07-31 TASK-047 控制台租户名称顶栏验证

- 控制台右上角元素改为`tenant-name`，首次`/console/api/overview`响应以`tenant_name`更新展示；缺失名称时显示“当前租户”。
- 静态契约确认HTML不再包含`session-subject`，JavaScript不再读取`session.subject`，并将资源版本更新为`console.js?v=20260731-02`。
- `node --check deploy/semattice/www/console/console.js`、Python HTML解析、Node绑定断言、`GOTOOLCHAIN=go1.26.5 go test ./internal/console ./cmd/ai-native-platform -count=1`和`git diff --check`：通过。
- 本地显示层修复未修改后端接口、Session、Organization/tenant映射；后续已由TASK-048提交并部署。

## 2026-07-31 TASK-046 Semattice网站Keycloak登录生产验收

- 完整实现以提交`dcf2b811b7ec88d0685938f6d6564c818ba24314`构建，Linux amd64二进制SHA-256为`d000e922e0231d39cca9040821bc42cdfa7b96411ad782d5b679bd083db93b87`，发布目录为`/opt/semattice/releases/20260731T074549Z-web-oidc-dcf2b811b7ec`。
- 发布前全量`go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、Linux amd64构建、16项Skill Python测试、官方Skill校验、无Token dry-run、JS/HTML、Shell和diff门禁通过。
- 现有`semattice-web` Client Secret仅在服务器内读取并写入`/etc/semattice/secrets/semattice-web-client-secret`；Secret目录为`root:semattice 0750`，文件和环境文件为`root:semattice 0640`。未重新生成、打印或提交Secret。
- Semattice、Nginx、PostgreSQL 16和Keycloak均active，Semattice重启计数为0，Nginx配置有效，近期服务错误计数为0；`/healthz`和`/console/`为200，匿名治理API与`POST /v1/auth/token`为401。
- `/auth/oidc/login`为303，精确跳转Keycloak realm authorization endpoint；`client_id=semattice-web`、callback、S256 PKCE、state、nonce及`openid organization`均正确，查询参数不含Client Secret。状态Cookie限定`Path=/auth/oidc`并具有`Secure; HttpOnly; SameSite=Lax`。
- 真实Chrome从匿名登录页点击“使用 Keycloak 登录”，经Keycloak成功回到`https://semattice.agentcici.com/console/`；页面显示企业管理中心、当前租户和退出按钮，浏览器控制台错误数为0。
- 环境备份`/etc/semattice/semattice.env.backup.20260731T074537Z-before-web-oidc`、上一应用release、Nginx备份和静态站备份均保留。ISSUE-001据此关闭。

## 2026-07-31 TASK-045 Semattice网站Keycloak登录本地验收

- `GET /auth/oidc/login`测试验证精确client/redirect、`openid organization`、随机state/nonce、S256 challenge、安全状态Cookie，且跳转URL不含Client Secret。
- 回调测试验证`client_secret_basic`、authorization code + PKCE换码、access/ID token双验证、subject一致性、唯一Organization和active tenant映射；成功只创建`Secure; HttpOnly; SameSite=Lax`短期Session Cookie，Cookie和响应不含Keycloak Token。
- 负例覆盖state伪造、state重放、多Organization和停用tenant，均重定向到固定失败页、不创建Session且不重复调用Token Endpoint。现有OACT `/console/session`回归继续通过。
- `GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify`和`CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath`：通过。
- `node --check`、Console HTML解析、`bash -n scripts/release-console.sh`和`git diff --check`：通过。
- 项目状态validator仅报告早于TASK-045存在的FEAT-033 frontmatter/status问题；FEAT-044、TASK-045归档和本次证据未新增状态错误。
- 本轮没有读取或写入生产Client Secret，没有修改Keycloak或远端服务，没有部署、提交、推送或修改Skill/CLI。

## 2026-07-31 TASK-044 Skill全部能力Scope与生产验收

- Skill API目录中的51项公开Capability归并为26个唯一`required_scope`；新增测试逐项读取目录并确认默认scope集合完全一致、无重复且不包含未公开的`tenant.provision`。
- `python3 -B -m unittest discover -s tests -p 'test_*.py' -v`：16项通过；覆盖默认26 scope换票、续期保持scope、v2缓存拒绝、PKCE/loopback、凭据存储、redirect阻断和既有安全负例。
- 官方`quick_validate.py`通过；全量`go test ./...`、`go test -race ./...`、`go vet ./...`、`go mod verify`、YAML、Python语法、CLI help、无Token dry-run、VERSION/README `1.3.0`和`git diff --check`通过。
- 开发副本经`rsync --dry-run --delete`确认只更新预期Skill文件后同步到本机安装目录；两目录逐文件一致，安装版本为`1.3.0`。未同步或发布独立GitHub Skill仓库。
- 生产配置更新前确认服务active、allowlist为1项且环境文件保持`root:semattice 0640`；备份为`/etc/semattice/semattice.env.backup.20260731T052514Z-all-capability-scopes`，只替换allowlist后重启。结果为26项、SHA-256 `93d07dbd42079b2df5a1c44c5732d6a7bcaf9fea72c16fa48be087933d0cfdf1`、健康200、匿名换票401、日志错误计数0。
- 对`orgx2x8awt02djpp5xdp`完成真实Keycloak Authorization Code + S256 PKCE登录，新会话返回26个scope。线上`system.capability.list`返回`succeeded`、51项能力、26个唯一scope，客户端缺失/多余集合均为空；新增scope保护的`tenant.get-status`只读调用成功且包含审计标识。
- 全程未输出Token、密码、refresh token或密钥，未调用任何业务写能力，未修改业务数据、元数据、Keycloak或授权资源。scope仍不替代Principal/RBAC、RLS、独立审批、幂等和审计。
- 项目状态validator仅报告早于TASK-044存在的FEAT-033 frontmatter/status问题；FEAT-043、TASK-044归档和本次证据未新增状态错误。

## 2026-07-31 TASK-040～TASK-043 合并后组合源码回归

- 将 CodeUp `origin/main` 的真实租户治理控制台与本地 Semattice 自有 Keycloak登录历史合并；冲突处理中同时保留 PostgreSQL console reader、`POST /v1/auth/token`、非公开租户开户边界和现有 Capability/MCP/API 路由。合并树无冲突标记，`git diff --check` 通过。
- `GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify` 和 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath` 全部通过；`internal/console` 回归已覆盖真实 reader接线。
- `PYTHONDONTWRITEBYTECODE=1 python3 -B -m unittest discover -s tests -p 'test_*.py' -v`：15 项通过。新增回调页验证覆盖成功/失败 HTML、`no-store`、CSP和授权码/state不回显。
- `node --check deploy/semattice/www/console/console.js`、仓库 `deploy/keycloak/*.sh` 与 `scripts/*.sh` 全量 `bash -n` 通过。
- 官方 `skill-creator/scripts/quick_validate.py` 对开发副本和本机安装副本均返回 `Skill is valid!`；开发副本已同步至本机安装目录且逐文件一致，未包含 Python缓存。
- 常见私钥/Token、10 MiB大文件、Python缓存、冲突标记和 staged diff门禁通过。项目状态 validator仅报告既有 FEAT-033 frontmatter/status问题，本次任务编号合并、ISSUE-001与组合验证未新增状态错误。
- ISSUE-001保留为真实发布风险：当前线上 `20260731T045751Z-standalone-auth` 在合并远端控制台源码前构建。本轮只验证并推送组合源码，不把两个历史 release分别通过误报为同一线上制品已组合发布。

## 2026-07-31 TASK-043 Semattice 自有换票生产发布

- 发布前 `go test ./...`、14 项 Python登录测试、`git diff --check`、Keycloak脚本语法和 Python语法门禁均通过；`linux/amd64` CGO-free制品 SHA-256 为 `73c552daffcf3ee2dcc203a009f08acc7b8effe3754e9a1d69267a690b3074f0`，上传后远端校验和一致。
- 原子切换至 `/opt/semattice/releases/20260731T045751Z-standalone-auth`；上一 release、受保护环境文件和 Keycloak `semattice-cli` client配置均已备份。环境文件保持 `root:semattice 0640`，release二进制为 `root:root 0755`。
- `semattice`、`nginx`、`postgresql-16`、`keycloak` 均 active，Semattice `NRestarts=0`；公网 `/healthz` 为 200，匿名 `/v1/auth/token` 为 401 `invalid_token`且 `Cache-Control: no-store`，匿名 Capability invoke为 401。
- Keycloak复核：`semattice-api-audience` mapper恰好一个，类型为 `oidc-audience-mapper`，access token audience为 `semattice-api`；`organization` scope仍为 optional，其 Membership mapper仍写入 access token。
- `tenant_registry`只读复核确认 `orgx2x8awt02djpp5xdp` 对应 tenant `ce85dabd-68be-503d-9d1b-9b63c536fa78`，global/native均为 active。
- 使用本机安装的 Skill `1.2.2` 完成真实 Keycloak Authorization Code + S256 PKCE登录；Semattice换票返回 authenticated，随后 OACT调用 `system.capability.list` 返回 `succeeded`、51 项能力，审计标识为 `audit:req-14679929-3ea0-4d06-be34-39e3c053f340`。未输出 Token、密码或密钥，未写业务数据。
- TASK-043验收时production OACT allowlist仅为`system.capability.read`；后续TASK-044已按用户要求扩展为全部26个公开能力scope，同时保留Principal/RBAC、RLS、审批和审计门禁。
- 项目状态校验器已用正确的 `.claw` 目录执行；仅报告早于本任务存在的 FEAT-033 frontmatter/status问题，FEAT-042、TASK-043、归档和发布证据未新增状态错误。`git diff --check` 与凭据扫描通过。

## 2026-07-31 TASK-042 Semattice 自有 Keycloak Organization 换票

- `go test -race ./... -count=1`：全量通过。新增覆盖 Keycloak OIDC固定 issuer/audience/RS256/JWKS/`azp`/Organization验证、Organization alias到 active tenant映射、scope allowlist、Semattice自签 OACT与现有 verifier兼容，以及缺 Token、多组织、未映射/停用 tenant和越权 scope负例。
- `go vet ./...`、`go mod verify`、`CGO_ENABLED=0 go build -trimpath ./cmd/ai-native-platform`：通过。
- `PYTHONDONTWRITEBYTECODE=1 python3 -B -m unittest discover -s tests -p 'test_*.py' -v`：14 项通过；登录和续期只调用 Semattice `/v1/auth/token`，Organization alias进入 Keycloak scope，旧 v1缓存 fail closed。
- 官方 `skill-creator/scripts/quick_validate.py` 对开发副本和本机安装副本均返回 `Skill is valid!`；YAML、bash语法、CLI help/dry-run、VERSION/README `1.2.2`、两目录逐文件一致、`git diff --check` 和常见 Token/私钥扫描通过。
- 活动 Go/Python登录代码中未检出 AgentCiCi API、`available-tenants`、外部 mint或 `/internal/v1/company-provisionings`；保留的 `sso.agentcici.com` / `semattice.agentcici.com` 是现有 Keycloak与 Semattice基础设施域名，不是应用接口依赖。
- 项目状态 validator仅保留既有 FEAT-033 frontmatter/status问题；TASK-042/FEAT-041、任务归档和完成任务上限未新增错误。
- 本轮未读取未跟踪 `.env`，未访问真实用户/租户 Token，未部署生产、修改远程 Keycloak、发布技能仓库、创建 tag或推送 Git。

## 2026-07-30 TASK-041 cloudcc-semattice Keycloak PKCE 登录本地验证

- `python3 -B -m unittest discover -s tests -p 'test_*.py' -v`：13 项通过，覆盖 S256 PKCE、state/authorization code负例、真实本机 loopback listener、默认最小发现 scope、可用公司/OACT 换票、Keycloak refresh token轮换、401 单次同请求重试、显式 `SEMATTICE_TOKEN` 优先、logout、`0600/0700` 缓存权限、符号链接拒绝、URL 凭据/query/fragment拒绝、Bearer redirect阻断、错误描述脱敏和错误响应连接重置。
- 独立前向使用检查确认流程可发现、新旧 dry-run兼容；检查发现的跨来源 redirect Authorization 转发、服务端错误描述泄密和空默认 scope均已修复并加入回归测试。
- 官方 `skill-creator/scripts/quick_validate.py` 返回 `Skill is valid!`；`agents/openai.yaml` YAML、Keycloak `semattice-cli` JSON、`bash -n`、新旧 CLI help/dry-run、VERSION/README `1.2.1` 一致性、Organization Scope、常见 secret pattern和 `git diff --check` 均通过。
- `GOTOOLCHAIN=go1.26.5 go test ./... -count=1` 全量通过；本次未修改 Go 运行时代码。
- Keycloak 官方文档确认 native app 注册 `http://127.0.0.1` 时允许系统选择动态端口，当前 client不使用全 wildcard、端口 wildcard、Web origin、implicit、password grant或 service account。
- 项目状态 validator 已运行，仅报告既有 `FEAT-033` 缺 `feature_id` / `updated_at` / `updated_by` 且 status非标准；FEAT-040/TASK-041 未新增状态错误。
- 本轮没有读取既有未跟踪 `.env`，没有访问真实身份/租户服务，没有浏览器登录、部署、发布仓库同步、提交、标签或远程推送。

## 2026-07-31 TASK-040 管理中心真实租户数据发布验证

- `local`：`GOTOOLCHAIN=go1.26.5 go test ./...`、`go vet ./...`、`go mod verify`、`bash -n scripts/release-console.sh`、`node --check deploy/semattice/www/console/console.js` 与 `git diff --check` 全部通过。
- `security/data-source`：控制台 Handler 只把已验证 Cookie 的 tenant/company/subject 传给 reader；reader 从 control pool 解析 active tenant，再以 runtime `database.WithTenant` 查询 published metadata、RBAC、组织与审计。匿名 `/console/api/overview` 继续为 401，reader 错误不返回治理数据。
- `production`：release `/opt/semattice/releases/20260731T012059Z-console` 已启动且 `semattice` active；Nginx 校验、`https://semattice.agentcici.com/healthz`、控制台静态页 200 和匿名治理 API 401 均通过。
- `live-tenant-read`：在服务器内部使用一次性 120 秒官方签名会话读取 `org5nszpgj99jaysxv6y`，未输出或保存令牌、私钥或用户身份。概览返回 `objects=5, fields=37, members=0, roles=0, organizations=0`；对象目录精确为 `dev_change:7, dev_project:8, dev_requirement:8, dev_task:8, dev_worklog:6`，并返回“Semattice 已发布研发交付模型 · 只读”。

## 2026-07-30 TASK-039 cloudcc-semattice 1.1.0 GitHub 发布验证

- 开发副本与本机安装目录、独立发布仓库逐文件一致；同步前两个 `rsync --dry-run --delete` 均无删除项，独立仓库仅包含 6 个预期发布文件变化。
- 开发副本和独立发布仓库的官方 `quick_validate.py` 均返回 `Skill is valid!`；VERSION/README `1.1.0`、YAML、Markdown 链接、CLI help、无 Token dry-run、缓存/私钥/常见 Token 扫描与 diff check 均通过。
- 独立仓库创建 release commit `3ac29afc34366d66a2e9320975dc3be498d55181` 和 annotated tag `v1.1.0`，通过 `git push --atomic -u origin main v1.1.0` 同时发布，未使用 force push。
- 本地 HEAD、`origin/main` 与 `v1.1.0^{}` 均为 `3ac29afc34366d66a2e9320975dc3be498d55181`；远程 HEAD 指向 main，仓库页和标签页 HTTP 200，raw VERSION 为 `1.1.0`，README 的标题、安装标签、升级示例和 `product-guide.md` 入口均已验证。
- 项目内容与上述发布证据提交为 `a55d71d773446902598b28fb525c7562003f351b`，通过普通 `git push origin main` 快进推送至阿里云 CodeUp；本地 HEAD、tracking ref 和远程 `refs/heads/main` 回读一致，未使用 force push。

## 2026-07-30 TASK-038 cloudcc-semattice 1.1.0 本地指南验证

- 产品定位和模块场景以 `README.md`、`.claw/goals.md`、ADR-012 与 FEAT-009 为依据；当前可执行操作继续以运行时代码、51 项 API 能力目录和资源模型为依据。
- 新增 `references/product-guide.md`，覆盖产品定位、五类核心问题、非目标、业务需求映射、14 个模块/资源场景、客户与联系人对象示例和 AI 设计输出要求。
- `SKILL.md` 已区分理解设计、实施调用和设计后实施；`capability-workflows.md` 已增加元数据版本、对象、字段、关系的创建/读取/修改/删除或退役矩阵，并明确空候选草稿、稳定 ID、不支持对象/关系删除以及字段 purge/tombstone 边界。
- 官方 `skill-creator/scripts/quick_validate.py` 对开发副本和本地安装副本均返回 `Skill is valid!`；`agents/openai.yaml` YAML 有效且短描述长度满足约束，Markdown 文件链接、`git diff --check` 和无 Token `metadata.object.upsert` dry-run 均通过。
- `rsync --dry-run --delete` 只显示 6 项预期更新和新增 `product-guide.md`，没有删除项；同步后开发副本与 `/Users/xuhm/.codex/skills/cloudcc-semattice` 的 `diff -qr --exclude=.git` 无差异，安装版本为 `1.1.0`。
- 远程 `v1.1.0` 标签不存在；本次未同步独立发布仓库、未提交、未创建标签或推送，最新远程已发布版本仍为 `v1.0.0`。
- 项目状态 validator 在归档最旧的 `TASK-010` 后只报告既有 `FEAT-033` 缺少 `feature_id` / `updated_at` / `updated_by` 及非标准 status；TASK-038、完成卡数量和本次归档未产生新错误。

## 2026-07-29 TASK-037 cloudcc-semattice 1.0.0 发布验证

- 技能身份已统一为目录 `skill/cloudcc-semattice`、frontmatter `name: cloudcc-semattice`、显示名 `CloudCC Semattice（语义格）` 和调用名 `$cloudcc-semattice`；项目根 `AGENTS.md` 的开发/发布路径与 GitHub 仓库名也已同步。
- 由于调用名不兼容，`VERSION` 从 `0.1.1` 升为 `1.0.0`；README 当前版本、`v1.0.0` 安装标签、新仓库 URL、新安装目录、新调用名和 `0.x` 升级提示一致。
- 官方 `skill-creator/scripts/quick_validate.py` 返回 `Skill is valid!`；YAML 身份断言、Python AST 语法、CLI help、无 Token `system.capability.list` dry-run、当前文件旧身份零残留、无嵌套 `.git`、两技能目录一致性与两仓库 `git diff --check` 均通过。
- 项目状态 validator 复验仅报告既有 `FEAT-033` 缺少 `feature_id` / `updated_at` / `updated_by` 和非标准 status；TASK-037 任务卡与新路径没有产生新错误。
- GitHub 仓库的当前名称确认为 `CloudCCAI/cloudcc-semattice`；本地独立发布仓库 `/Users/xuhm/Documents/cloudcc-semattice-skill` 的 origin 已同步为该地址。提交 `5b156c057af7517c81f5892d1f8123ec74f00ea6` 和 annotated tag `v1.0.0` 已通过原子 push 发布，未使用 force push。
- 本地 HEAD、`origin/main` 与 `v1.0.0^{}` 均为 `5b156c057af7517c81f5892d1f8123ec74f00ea6`；远程 HEAD 指向 main，仓库页面 HTTP 200，raw VERSION 为 `1.0.0`，标签页面和 README 的标题、安装地址、安装标签、升级示例与 `$cloudcc-semattice` 调用名均已验证，发布仓库工作树 clean。

## 2026-07-29 TASK-036 发布流程文档边界纠正

- 已将准备版本、双目录边界、`rsync --dry-run`、发布前校验、提交/标签/原子 push 和发布后验证完整迁移至项目根 `AGENTS.md`，并明确禁止将这些内部维护步骤写回技能 README。
- 项目内开发副本和独立发布副本的 README 均未检出“维护与发布流程”、双目录边界或 1–5 发布步骤；`diff -qr --exclude=.git` 确认两个技能目录一致。
- 官方 `skill-creator/scripts/quick_validate.py` 返回 `Skill is valid!`；`agents/openai.yaml` YAML 和默认 `$skill` 名称、Python AST 语法、CLI help、无 Token dry-run、项目与发布仓库 `git diff --check` 均通过。
- 项目状态 validator 复验仅报告既有 `FEAT-033` 缺少 `feature_id` / `updated_at` / `updated_by` 和非标准 status；本次文档边界纠正未产生新错误。
- 本次只更新本地开发副本和本地独立发布仓库；未提交、未创建新标签、未推送远程，已发布版本仍为 `v0.1.1`。

## 2026-07-29 TASK-036 Semattice 技能发布流程与 v0.1.1 验证

- `v0.1.1` 发布时的 README 曾记录项目内开发副本不包含 `.git`、独立发布仓库边界、SemVer 准备、`rsync --dry-run` 同步、发布前校验、annotated tag、原子 push 和发布后验证流程；其后已按用户要求迁移至项目根 `AGENTS.md`。
- 同步 dry-run 只报告 `README.md` 和 `VERSION`；实际同步后 `diff -qr --exclude=.git` 无差异。
- `VERSION=0.1.1`；官方 `skill-creator/scripts/quick_validate.py` 返回 `Skill is valid!`；`agents/openai.yaml` YAML 与默认 `$skill` 名称、Python AST 语法、CLI help、无 Token `system.capability.list` dry-run、SemVer/README 一致性、Python 缓存、常见私钥/Token 模式和 `git diff --check` 均通过。
- 独立仓库在 `main` 创建提交 `228f6f737b53ce41cc3f51126ca58498d33a3f47` 和 annotated tag `v0.1.1`，通过 `git push --atomic origin main v0.1.1` 推送，未使用 force push。
- 本地 HEAD、`origin/main` 与 `v0.1.1^{}` 均为 `228f6f737b53ce41cc3f51126ca58498d33a3f47`；远程 HEAD 指向 main，仓库页面 HTTP 200，raw VERSION 为 `0.1.1`，README 的当前版本、安装标签、升级标签和发布流程标题已验证，发布仓库工作树 clean。
- 项目状态 validator 已实际运行，仅因既有 `docs/specs/FEAT-033-unified-principal-projection.md` 缺少 `feature_id`、`updated_at`、`updated_by` 且使用非标准 status 而失败；TASK-036 的任务数量、归档和状态引用未产生新错误。
- 项目仓库提交前复验官方技能校验、YAML、Python 语法、CLI help、无 Token dry-run、常见凭据模式、敏感扩展名、10 MiB 大文件、嵌套 `.git` 与全量 `git diff --check`，均通过。

## 2026-07-29 TASK-035 Semattice 技能 v0.1.0 发布验证

- `VERSION` 为合法 SemVer `0.1.0`，README 版本引用和目标 Git 标签 `v0.1.0` 一致；官方 `skill-creator/scripts/quick_validate.py` 返回 `Skill is valid!`，`agents/openai.yaml` YAML 与技能目录空白检查通过。
- `semattice_api.py --help`、Python AST 语法检查和 `system.capability.list` HTTPS dry-run 通过；dry-run 未使用或输出 Token。
- 独立本地仓库 `/Users/xuhm/Documents/semattice-customization-expert-universal` 已在 `main` 创建 root commit `93c2701` 和 annotated tag `v0.1.0`，工作树 clean。
- SSH 配置 `github-cloudcc-admin` 使用专用 ED25519 密钥，GitHub 返回 `Hi CloudCCAI!`。远程仓库创建后，以 `git push --atomic -u origin main v0.1.0` 同时发布分支与标签；未使用 force push。
- 本地 HEAD、远程 `refs/heads/main` 与远程 `refs/tags/v0.1.0^{}` 均为 `93c270124c7992612100380676cecf4affc31b5d`，远程 HEAD 指向 main；本地 main 已跟踪 origin/main 且工作树 clean。
- `https://github.com/CloudCCAI/semattice-customization-expert-universal` 返回 HTTP 200；raw `VERSION` 为 `0.1.0`，README 标题和 `v0.1.0` 安装/升级引用验证通过。

## 2026-07-29 TASK-034 Semattice 技能重命名验证

- `skill-creator/scripts/quick_validate.py skill/semattice-customization-expert-universal` 返回 `Skill is valid!`；临时 `PyYAML` 依赖仅安装于自动清理的一次性目录，未修改项目或全局依赖。
- 技能目录名与 `SKILL.md` frontmatter 名称完全一致，名称长度为 40；技能目录检索旧名 `semattice-operator` 为零结果。
- `scripts/semattice_api.py` Python 语法检查和技能目录尾随空白检查通过；references 与脚本 SHA-256 和重命名前一致，功能内容未改变。
- 项目状态 validator 已实际运行，但因既有 `docs/specs/FEAT-033-unified-principal-projection.md` 缺少 `feature_id`、`updated_at`、`updated_by` 且使用非标准状态值而失败；本次 TASK-034 新增状态记录未产生 validator 错误。

## 2026-07-27 TASK-033 Semattice Principal 投影发布验证

- `GOTOOLCHAIN=go1.26.5 go test ./...`、`go vet ./...`、`go mod verify`、Linux amd64 CGO-free 构建和 `git diff --check` 均通过；release binary SHA-256 为 `a8ef0f16af2f4c7944c69b77d29bbb75f48ab646dcce29e1a8bdf229cf95242d`。
- OACT parser 定向覆盖新 HUMAN/SERVICE principal claim、sub 与 principal_id 不一致、HUMAN 携带 service-only claim、无效 owner UUID/client_id 等负例；API、MCP、CLI 都通过 TrustedPrincipal 绑定同一 Actor，不接受 raw Keycloak service token。
- 已按不可变发布流程切换至 `/opt/semattice/releases/20260727T151437Z-console`；服务为 active，`https://semattice.agentcici.com/healthz` 返回 200，匿名 `/console/api/overview` 和未授权 capability invoke 均返回预期 401，Nginx 配置检查通过。旧 release 保留可回滚。
- 未执行真实机器 OACT 调用：AgentCiCi service-token-exchange 与受控开户仍保持关闭，等待 SMTP、OACT 签名配置和受权 service client 凭据后进行，不伪造 token 或读取 secret。

## 2026-07-25 TASK-032 Semattice 企业管理中心验证与上线

- `GOTOOLCHAIN=go1.26.5 go test -race ./...`、`GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、状态 validator 与 `git diff --check` 全部通过。
- `internal/console` 定向测试覆盖匿名 fixture 401、有效 OACT 交换为 Secure/HttpOnly 会话、管理 scope 访问和缺少管理 scope 的 403。fixture 仅在内存返回，不连接或写入 PostgreSQL。
- 演示 fixture 已与页面概览一致：12 个对象、84 个字段、8 位成员、6 个组织节点、24 条审计与 10 项系统配置；所有记录均明确为模拟治理数据。
- Playwright 桌面验收：对象与字段导航、对象选择、字段矩阵和对象检查器可用，浏览器控制台为 `0 errors / 0 warnings`；本地验收截图未提交。
- Linux amd64 静态构建、`bash -n scripts/release-console.sh` 通过；发布时脚本在服务器校验上传二进制 SHA-256、`nginx -t` 和 `/healthz`，并原子切换至 `/opt/semattice/releases/20260724T230626Z-console`。
- 生产 smoke：`/console/` 标题为“Semattice 管理中心”；`GET /console/session` 无 Cookie 为 200 `authenticated:false`；无 Cookie 的 `/console/api/overview` 为 401；伪造 OACT 的 `POST /console/session` 为 401；`/healthz` 为 200，`semattice` active，`nginx -t` 成功。真实浏览器无登录入口为 `0 errors / 0 warnings`。
- 回滚证据：上一应用 release `20260724T155000Z-mcp-public-discovery`、静态站备份 `/var/www/semattice-backups/20260724T230626Z-console` 与 Nginx 备份 `/etc/nginx/conf.d/semattice.conf.backup.20260724T230626Z-console` 均存在。未读取、打印或持久化 OACT、私钥或其他 secret。
- `product-switch` 热修复：`node --check deploy/semattice/www/console/console.js`、`GOTOOLCHAIN=go1.26.5 go test ./...`、`go vet ./...`、`go mod verify` 和 `git diff --check` 通过。控制台顶栏可展开当前 Semattice / AgentCiCi 管理端菜单，回跳只使用固定 `https://x.agentcici.com/admin`。release `/opt/semattice/releases/20260725T024148Z-console` 已上线；`semattice` active、`nginx -t` 与 `/healthz` 通过，匿名 `/console/api/overview` 为预期 401，公网静态页包含两端菜单文本与回跳地址。
- `asset-cache` 修复：控制台根页和静态资源均返回 `Cache-Control: no-store`，HTML 将 CSS/JS 引用更新为 `?v=20260725-03`。发布后的浏览器在无真实会话的安全门页中临时显示壳体以验证纯视觉结构，触发器为 96×30px，展开菜单为 224×116px，菜单项和 14px SVG 图标均正常；未伪造会话、Token 或治理数据。当前线上 release 为 `/opt/semattice/releases/20260725T025439Z-console`，控制台根页为 200、匿名治理 API 为 401、服务 active、Nginx 校验通过。

## 2026-07-24 TASK-031 公开 MCP 发现与认证调用发布验证

- 本机：`go test ./...`、`go vet ./...`、`go mod verify`、`git diff --check` 全部通过；`go test -race ./internal/mcp` 通过。新增 Streamable HTTP 测试覆盖匿名 initialize/tools/list、匿名同 session 的 tools/call 401、令牌在该 session 后续调用时重新验证并绑定 trusted principal。
- 代码提交 `9cbbd28` 已快进合并并推送到 CodeUp `main`。Linux amd64 静态制品 SHA-256 为 `05c714d24dafd6f438683640da317bcd03c0d05705a58184db96b344c830c38b`，线上 release 为 `/opt/semattice/releases/20260724T155000Z-mcp-public-discovery`，切换后 `semattice` 为 `active`。
- 首次公网 initialize 暴露 SDK loopback DNS-rebinding 对外 Host 的 403；未关闭该防护，而是将受控 Nginx `/mcp` upstream `Host` 固定为 `127.0.0.1`，`nginx -t` 与 reload 成功，备份为 `/etc/nginx/conf.d/semattice.conf.backup.20260724T155400Z-mcp-host`。
- 公网匿名 `initialize` 返回 200 与 MCP session ID；`notifications/initialized` 返回 202；`tools/list` 返回 200 和 49 个已发布工具，且 schema 仅要求 `request_id` 与 `input`；同一 session 的匿名 `tools/call` 返回 401 `no bearer token`；`https://semattice.agentcici.com/healthz` 返回 200。未读取、打印或存储生产 Bearer token。

## 2026-07-24 TASK-024 Streamable HTTP MCP 线上路由验证

- 变更前公网 `GET /mcp` 落入静态首页（200），`POST /mcp` 由 Nginx 返回 405；服务器 loopback `POST http://127.0.0.1:8080/mcp` 已返回 401，确认问题仅在边缘路由。
- 已备份 `/etc/nginx/conf.d/semattice.conf` 到 `.backup.20260724T153300Z`，新增精确 `/mcp` 代理、Authorization 透传和禁用 buffering/cache；`nginx -t` 通过后 reload Nginx，并重启 Semattice。
- 变更后 Nginx 与 Semattice 均为 `active`；公网 `POST https://semattice.agentcici.com/mcp` 返回 401，`/healthz` 返回 200。该验证未使用或输出生产 Token；带有效 Bearer JWT 的真实 MCP 客户端工具发现留给客户端接入验证。

## 2026-07-24 TASK-030 CodeUp 发布门禁

- CodeUp 目标在首次检查时为空仓库，非交互认证成功；`git push codeup HEAD:refs/heads/main` 成功创建远程 `main`。
- 当前工作树发布前为 clean；本地分支 `agent/go-capability-platform-baseline`，GitHub `origin` 保留，未修改 upstream、未 force push、未执行 mirror。
- `GOTOOLCHAIN=go1.26.5 go test ./... -count=1`、`go vet ./...`、`go mod verify`、状态 validator、`git diff --check`、`git fsck --full --no-dangling`：全部通过。
- 当前 157 个受跟踪文件无常见私钥头、阿里云/AWS/GitHub/OpenAI 密钥或 JWT；无敏感证书/密钥扩展名文件，无超过 10 MiB 的受跟踪文件。该扫描是发布门禁，不替代专用供应链 secret scanner。

## 当前运行摘要

- 状态：`passed`。覆盖 `TASK-010`、`TASK-011`、`TASK-012`、`TASK-013`、`TASK-015`、`TASK-016`、`TASK-024`、`TASK-025` 的当前本地实现，以及 `TASK-022` 的授权 ECS 部署验收。
- 数据库：Docker PostgreSQL `16.13 (Debian 16.13-1.pgdg13+1)`；镜像 digest `postgres@sha256:5d143123fdf80462d1778cd4f24b9f7ca13c87174bca19141fb194c5a1ebca59`。仅绑定 `127.0.0.1:55432` 的专用临时容器，结束后清理。
- 本轮 maker 从 fresh schema 执行 migrations 1–13；`schema_migration` checksum 可重复，128 个 object partitions 与五类共 640 个 typed-index partitions 完整。TASK-010–013 的第三轮独立 checker 历史结论继续有效；TASK-015 尚未声明新的独立 checker 结论。

## 2026-07-24 TASK-029 Keycloak 基础设施验证

## 2026-07-24 TASK-029 AgentCiCi IdP 登录主题验收

- `theme-source`：`bash -n deploy/keycloak/apply-agentcici-login-theme.sh` 与 `git diff --check` 通过；主题是 Keycloak `keycloak.v2` 的原生 login theme，不改 form action、认证代码或 Keycloak session 参数。
- `production`：在 `115.29.222.70` 将 `agentcici` Realm 设置为 `loginTheme=agentcici`、单一默认 locale `zh-CN`，Keycloak 重启后本机 `/health/ready` 为 `UP`。发布前主题/realm 设置备份为 `/opt/keycloak/backups/20260724T120956Z-before-agentcici-login-theme`；当前 theme 目录未保留 macOS sidecar 文件。
- `browser-desktop`：真实 Authorization Code + PKCE 入口确认生产 CSS 与 6 张原始 AgentCiCi `login_mode2` 立方体图片均由 Keycloak 加载；星空、立方体真实图片、旋转、居中框架与原页面一致。账号或邮箱、密码、密码可见性和“统一账号登录”均为中文，浏览器控制台无错误。截图：`.playwright-cli/page-2026-07-24T12-10-25-052Z.png`（本地验收产物，未提交）。

## 2026-07-24 TASK-029 应用认证链路发布验证

- AgentCiCi 后端 V96 已上线；24 个有效全局账户以 `issuer=https://sso.agentcici.com/realms/agentcici + sub` 映射到 `account_external_identity`，密码哈希迁移不读取或输出明文。`agentcici-bff` 使用 Authorization Code + PKCE；入口 `/auth/oidc/login` 返回 302 到 Keycloak 且设置 `CICI_OIDC_STATE` 安全 Cookie。
- AgentCiCi `2.8.11` 已启用 10 分钟 RS256 OACT 签名，公开 JWKS 返回 200、一枚 `RSA/sig/RS256` 公钥；签名私钥和 Keycloak client secret 均只在 root 受限配置中保存。
- Semattice release `20260724T094721Z-keycloak-jwks` 已运行。全量 `GOTOOLCHAIN=go1.26.5 go test ./...`、定向配置/身份/main 测试和 Linux amd64 CGO-free 构建通过。其固定 JWKS verifier 对无 Token 和非受信 issuer 均返回 HTTP 401；使用一次性、无业务数据技术烟测 OACT 调用 `system.capability.list` 返回 HTTP 200 / `succeeded`。验证仅发生本地 JWKS 验签，不调用 Keycloak。
- 初始发布阶段没有 `PROVISIONED` binding，因此技术烟测不作为真实业务租户验收；随后按本报告下一条完成了用户选择公司的受控开户恢复与两端绑定确认。
- 后续真实公司验收：用户选择 `org2sva14i4udjmi2t4s`。定位到 Semattice 的 AgentCiCi 回调域名 `onechat.agentcici.com` 无法在 ECS 解析，导致 reservation 返回 `FAILED_PRECONDITION`；改为 `https://x.agentcici.com` 后，以既有幂等键安全重试，签名 `POST /internal/v1/company-provisionings` 返回 HTTP 200 / `active`。AgentCiCi binding 与 Semattice `tenant_registry` 均确认 tenant `93ff0c87-a626-529e-b8cf-195825df2488`、公司 `org2sva14i4udjmi2t4s`、状态 `PROVISIONED/active`。
- 最终真实主体 OACT 验收：该公司有两名有效成员已映射 Keycloak `sub`。在不输出成员身份、密码、Token 或私钥的前提下，以其中一名真实 `sub`、该 tenant UUID、公司 ID、`aud=semattice-api` 和 `scope=system.capability.read` 构造 120 秒 RS256 OACT。公网 Semattice API 返回 HTTP 200，`system.capability.list` 状态为 `succeeded`；无 Token/非受信 issuer 的对照请求仍返回 401。该验证证明真实公司、真实已映射主体、AgentCiCi JWKS 和 Semattice 本地验签绑定一致，且没有逐请求 IdP 回调。

- ECS `115.29.222.70` 已安装 Keycloak `26.7.0`（Amazon Corretto 21、PostgreSQL 16 独立 `keycloak` 数据库/role、非特权 `keycloak` systemd、loopback `8180/9000`）并通过 `systemctl` 重启后启动。
- `https://sso.agentcici.com/realms/agentcici/.well-known/openid-configuration` 与 JWKS cert endpoint 均 HTTPS 200，证书校验成功；管理控制台重定向正常。
- 本机 `/health/live` 与 `/health/ready` 均为 `UP`；公网 `/health`、`/metrics` 均为 404；Nginx `nginx -t` 通过。
- `agentcici` Realm 与 `agentcici-bff`、`semattice-api`、`official-access-context`、`followup-worker` client 注册已创建；未读取/输出 client secret。
- 随机、自动删除的账号和直接授权测试客户端验证：AgentCiCi `PBKDF2WithHmacSHA256` / 120,000 轮 / 256-bit hash，可作为 Keycloak `pbkdf2-sha256` credential 导入并成功换取 Token。导入时 Keycloak salt 使用 AgentCiCi 存储 salt 字符串 UTF-8 字节的 base64，不是二次解码该字符串。无真实账号、密码或 credential 被读取。
- 回归保护：`https://semattice.agentcici.com/` 仍 HTTPS 200。此节只证明 IdP 基础设施，不证明 AgentCiCi/Semattice 运行时代码已完成 OACT/JWKS 切流。

## 2026-07-23 TASK-025 company_id 全局身份改名验证

- fresh Docker PostgreSQL 16 按 checksum 执行 migrations 1–13；migration 13 将 `tenant_registry.org_id` 和关联约束无损改名为 `company_id`。集成断言确认新列存在、旧列不存在，并保留既有编号值与唯一性。
- JWT 只接受 `company_id`；旧 `org_id` claim、旧 tenant capability input 均被拒绝。六项 tenant capability descriptor 升至 v2，输入/输出 schema 仅暴露 `company_id`。
- `organization_id` 授权组织树、组织合并和数据范围 PostgreSQL 回归随全量测试执行，未发生字段改名。
- `GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`./scripts/test-postgres.sh run`、`GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、本机临时路径 `go build -trimpath`、状态 validator 与 `git diff --check`：passed。
- 本轮仅验证当前工作树；未更新 ECS、远程 PostgreSQL、Nginx、外部运营控制面或 JWT 发行方。

## 2026-07-23 TASK-024 Streamable HTTP MCP 验证

- `mcp.StreamableClientTransport` 经 HTTP Bearer transport 连接新 `/mcp` handler，完成初始化并成功调用 `system_capability_list`；调用审计的 tenant 与 actor 均来自已验证 JWT principal。
- 未携带 Bearer token 的规范 MCP POST 返回 HTTP 401。
- `GOTOOLCHAIN=go1.26.5 go test ./... -count=1`、`GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、`GOTOOLCHAIN=go1.26.5 go build -trimpath ./cmd/ai-native-platform` 和 `git diff --check`：passed。
- 本轮仅验证当前工作树；未更新授权 ECS 制品、Nginx 配置或公网 endpoint。

## 2026-07-23 TASK-022 部署验收

- 发布前：`GOTOOLCHAIN=go1.26.5 go test ./... -count=1`、`go vet ./...`、HTML parse、状态 validator 与 `git diff --check` 均通过。
- Linux amd64 CGO-free 二进制 SHA-256 为 `24fa672c9399e2f60cce4412ed754654141594e76e82b2d253bf309c93b3db59`；公网下载与 `.sha256` 声明完全一致。
- ECS 运行 PostgreSQL 16.13、Nginx 1.30.2；12 个内置 migration 一次事务完成，809 张父表/分区表启用并强制 RLS。migrator 临时 CREATEROLE 已撤销，control/runtime/migrator 均为非 superuser、非 CREATEROLE、非 BYPASSRLS。
- 监听边界：PostgreSQL `127.0.0.1:5432`，Semattice `127.0.0.1:8080`，Nginx 公网 80/443。
- 公网验证：HTTPS 首页 200；HTTP 301；edge health 200；无令牌 Capability API 401；证书 SAN 覆盖 `*.agentcici.com`，有效期至 2026-11-24。
- 五分钟短期 smoke JWT：公网 API `system.capability.list` 返回 `succeeded` 和 49 项能力；服务器 CLI 返回同样 49 项；MCP stdio 以协议 `2025-11-25` 初始化、枚举 49 个工具并真实调用 `system_capability_list` 成功。
- `semattice.service` 重启后 PostgreSQL、Semattice、Nginx 均保持 active/enabled，首页与 health 再次通过；env mode `0640`、TLS 私钥 mode `0600`。
- 浏览器结构检查：标题、首屏主标题、API/CLI/MCP/安全导航均正确，控制台没有 error/warn。浏览器截图接口超时，因此不把截图声明为验收证据。
- 本轮为 maker 部署验收；未使用真实租户数据，未声明 HA、备份恢复、自动续证、容量或生产 SLA。

## 2026-07-23 TASK-023 能力矩阵验收

- 从已部署、经短期 smoke JWT 鉴权的 `system.capability.list` 取得 49 项 Capability Descriptor，页面按七域分组为 `6 + 6 + 10 + 5 + 12 + 9 + 1 = 49`。
- HTML parser 对本地页面和公网页面均验证 7 个 domain、49 个唯一 capability item；页面 ID 集合与 Runtime Registry 集合完全相等。
- 进一步逐项验证 49 个 `required_scope` 与 49 个 `risk_level`，全部和运行时 descriptor 一致。
- `node --check deploy/semattice/www/app.js`、CSS brace、HTML parse、`git diff --check` 均通过；Nginx 配置复验通过。
- 公网首页、CSS、JS 均返回 200；能力介绍、矩阵标题和四个能力支柱的可见文本断言通过，edge health 保持正常。
- 浏览器桌面 1440×900：四个支柱等宽四列，矩阵 7 域/49 项，默认展开 1 域；“展开全部”后 7 域全部打开且 `aria-expanded=true`。
- 浏览器移动 390×844：支柱单列、矩阵摘要和条目按移动布局收敛；body 禁止横向滚动。桌面和移动检查均无 console error/warn。
- 服务器发布前保留上一版静态文件于 `/var/www/semattice-backups/20260723T0658Z`；本次只更新静态 HTML/CSS/JS，未重启或修改应用、数据库和 TLS。

## 通过的门禁

- `TEST_DATABASE_URL=... go test ./... -count=1`
- `TEST_DATABASE_URL=... go test -race ./... -count=1`
- `go vet ./...`
- `go mod verify`：`all modules verified`
- `python3 /Users/owenmacbook/.agents/skills/y-agentic-project-guidelines/scripts/validate-state.py .claw`
- `git diff --check`
- `CGO_ENABLED=0`、`-trimpath`：`linux/amd64`、`linux/arm64`、`darwin/arm64`、`windows/amd64` 四目标构建

## 数据库与安全证据

- `ai_native_runtime` 和 `ai_native_control` 均非 owner、非 superuser、非 BYPASSRLS。
- control 仅对 `tenant_registry`、`tenant_operation`、`audit_event` 三表拥有按操作精确授权及三条 role policy；对 `shard_registry`、业务记录和元数据表均无权限。
- runtime 缺失 TenantContext 时对 registry、records 和 metadata 均读取 0；错误 tenant/bucket、跨租户写入与跨租户关系全部拒绝。
- `MaxConns=1` 的 commit/error/panic 路径均不残留 tenant/bucket/actor；分区裁剪只命中目标 bucket。
- 实际 `main` 双角色接线以 control 完成 `tenant.provision`，再以 runtime TenantContext + control router 创建元数据版本。
- typed-index parent/child tables 与 `record_operation` 均 FORCE RLS；control 无记录面权限，runtime 无 TenantContext 读取 0。typed rows 和 relation source 以复合 FK 绑定 tenant、metadata version、object 和 record。

## 能力与模型证据

- 一个 system、六个租户、十三个元数据/Changeset 和五个记录能力均从同一 Capability Registry/Invoker 投影到 authenticated API、authenticated MCP 和无交互 agent CLI；记录五能力和 Changeset get-status 的成功/错误 parity 通过。
- JWT issuer/audience/algorithm/key/expiry、tenant/org/subject/scope 负向用例 fail closed。
- 元数据 UUIDv7、同 tenant/version 复合约束、draft-only 变更、独立批准发布、发布后不可变及确定性 snapshot/SHA-256 digest 测试通过。
- 记录 create/get/update/delete/query、published-only 对象、兼容 metadata version 读取/查询与 update 惰性迁移、未知字段、类型/default/required、UUIDv7、revision 冲突、软删除、游标、未索引 filter 拒绝、关系 target/restrict、跨租户和连接池复用测试通过。

## TASK-015 百万记录物理基准

- 数据集：1,000,000 条 active records，每条一个 indexed text 和一个 indexed number；2,000,000 typed rows，写放大精确为 2 typed rows/record。
- 写入：object set insert 6.493 s；typed-index set insert 61.228 s。
- 尺寸：object 580,608,000 bytes；text tree 416,006,144 bytes；number tree 321,191,936 bytes；合计 1,317,806,080 bytes，平均 1,317.81 bytes/record。
- 查询：真实 `buildQuery` 的 200 次等值/范围样本 p50 0.503 ms、p95 1.400 ms；计划命中 `record_index_number_b007_value_idx`，无 `object_record_b007` full scan。
- 此结果只验证当前本机 PostgreSQL 物理路径，不是生产 SLA；8/16 GiB、50 并发、200 活跃用户与热点公平性仍属于 TASK-019。

## 依赖与范围

- 完整 module graph 为项目本身加 26 个外部模块；15 个进入生产二进制闭包，11 个仅属于完整 graph/test/tool graph。精确版本和许可记录在 `FEAT-010`；`FEAT-020` 的旧依赖快照已标记 superseded。
- 第一轮 checker 的 control/runtime 单 pool、metadata 预路由和许可图问题，以及第二轮 checker 的 control 多余 `shard_registry SELECT` 均已修复并有回归测试。
- 未执行生产/共享数据库访问、远端 Git 写入、CI、制品发布、部署、HA、备份或恢复。

## 2026-07-19 动态字段设计治理增量

- 新增 FEAT-014，更新 FEAT-009、FEAT-015、ADR-010、task board 和 current status；本次只有文档与项目状态变更，没有修改或重新验证 Go/数据库源码。
- `python3 /Users/owenmacbook/.agents/skills/y-agentic-project-guidelines/scripts/validate-state.py .claw`：passed。
- `git diff --check`：passed。
- PostgreSQL 官方 JSONB/TOAST/limits 文档与 TASK-015 百万记录尺寸重新核对；`737,198,080 / 2,000,000 = 368.60` bytes/record/indexed-field，20/40/50 字段线性估算为 7.37/14.74/18.43 GB。该估算不替代 TASK-019 的真实并发容量测试。

L1 历史审计结果归档在 `docs/archive/loop-engineering/2026-07-16-l1-test-report.md`，不作为本轮 Go/PostgreSQL 实现证据。

## 2026-07-24 TASK-026 本机真实 AgentCiCi 受控开户验收

- Semattice 使用项目内置的独立 `postgres:16` 容器（`127.0.0.1:55432`）完成 schema migration，并以分离的 `ai_native_control` / `ai_native_runtime` 登录角色启动；AgentCiCi 使用本机既有 `agentcici` PostgreSQL、Redis、RabbitMQ 和 Qdrant。
- 真实 AgentCiCi 运营平台认证成功后，创建组织 `orgc9h2xs5puanlbykmc`，随后调用 AgentCiCi 的 `POST /platform/tenants/{orgId}/semattice-provisionings`。这是 AgentCiCi 对 Semattice 的真实 HMAC 请求；Semattice 再以独立 HMAC 调用 AgentCiCi 的 reservation 与 completion 回调。
- 成功响应的 `company_id` 与新建 `org_id` 完全一致；Semattice tenant UUID 为 `22369429-94c0-5dc2-ad04-600673f62829`，产品状态为 `active`。AgentCiCi binding 为 `PROVISIONED`，其回写 tenant UUID 与 Semattice `tenant_registry` 一致。
- 使用同一 idempotency key 重放 AgentCiCi 开通请求后，返回相同 tenant UUID；Semattice `tenant_operation` 对该 tenant 只有 1 条 `succeeded` 记录。未创建重复开户。
- 浏览器端到端补充：在 AgentCiCi 运营端以真实平台账号登录，打开新组织 `orgnuctqa4lpdn9zz1qx` 的应用卡片并点击“开通 Semattice”。页面收到真实成功响应后，卡片变为“运行中”、应用数从 1 到 2、按钮变为禁用的“已开通”。
- 本轮只访问本机服务和本机专用数据库；临时 HMAC、数据库角色口令和平台 token 均未写入仓库或本报告。

## 2026-07-20 TASK-014 核心实现增量

- fresh `postgres:16` 专用容器执行 migrations 1–5；`metadata_changeset`、`metadata_changeset_object`、字段 lifecycle/index/default columns、128 bucket 和既有 640 typed-index partitions 一并验证。
- `./scripts/test-postgres.sh run`：全量通过。
- `TEST_DATABASE_URL=... GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`：全量通过。
- Changeset 非破坏性 `validate -> simulate/get-status -> approve -> publish -> rollback` 通过；API、MCP、CLI get-status 结果等价，直接后续 version publish 被拒绝，rollback 需要受信审批。
- 有存量记录时新增 indexed + `backfill_required` 字段被判定 high risk，候选索引固定为 `building`，coverage 未 ready 前 publish 返回稳定 `FAILED_PRECONDITION`，tenant metadata pointer 保持旧版本。
- 记录层新增 lifecycle 写入/可见性门禁、只允许 active typed index 过滤、256 KiB 总 JSON、64 KiB 单 JSON 字段、8 层和 1,000 数组元素硬限制；record write route lock 与 Changeset activation lock 消除旧 metadata route 写回窗口。
- 本节记录第一阶段 maker 自验；当时未完成的 backfill/coverage、purge/tombstone 和版本化配额已由下方最终实现验证补齐。`TASK-019` 容量认证仍未完成。

## 2026-07-20 TASK-014 最终实现验证

- migration 6 新增 `governance_policy`、`record_unique_value`、`field_tombstone`、字段 unique contract 和 Changeset checkpoint/progress；fresh PostgreSQL 16 可按 checksum 顺序执行 migrations 1–6，唯一值与墓碑表均启用并强制 RLS。
- 有界 backfill 真实覆盖按对象暂停/恢复、savepoint 错误隔离和 checkpoint。测试使用一个未提交的并发 record update 制造 PostgreSQL 行锁竞争；worker 读取旧 revision 后条件更新影响 0 行，当前批返回 `conflict_records=1` 且不覆盖用户值，下一批读取最新 revision 后成功。
- predecessor 测试证明原 `field_id` 改类型被拒绝，而新字段通过 `predecessor_field_id` 将文本 `42.50` 转换为 number、重建 typed index 并在 coverage ready 后激活。
- 两条重复 `code` 记录启用 unique 时，一条成功、一条因唯一冲突留在旧版本，coverage/publish 继续关闭；修复冲突后重跑完成并进入 ready。
- candidate 写入测试覆盖回填期间的新 customer/contact 记录投影、候选 required/default typed index 和 relation edge 双写；历史 contact 回填后 relation coverage 精确为 2。
- optional `on_create` 默认值测试确认旧记录的物理 metadata version 与 JSON key 均未重写，新记录才获得默认值。
- purge 测试覆盖 `deprecated_read_write -> deprecated_read_only -> hidden -> purging -> tombstone`、普通 backfill 对破坏性变更拒绝、独立 purge approval、JSON/typed/unique/relation 清理、tombstone 名称保留和破坏性回滚拒绝。
- 版本化 standard policy 在 Changeset validate 时冻结为 `policy_version=1`、500 字段/20 index；dedicated-16g 为 500/40。超过 256 KiB 的 record create 在 API/MCP/CLI 返回同一 `VALIDATION_FAILED`。
- `TEST_DATABASE_URL=postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable GOTOOLCHAIN=go1.26.5 go test ./... -count=1`：passed。
- `TEST_DATABASE_URL=postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`：passed。
- `GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、darwin/linux × arm64/amd64 四目标 `CGO_ENABLED=0 go build`：passed。
- 本轮为 maker 最终自验，未声明独立 checker；`TASK-019` 的 8/16 GiB、50 并发与热点公平性容量认证仍保持独立 `todo`。

## 2026-07-22 TASK-016 授权设计治理验证

- 新增用户确认的 `FEAT-016` 与 ADR-011；更新 TASK-016 为 `ready`，本次未修改 Go、数据库 migration 或运行时行为。
- `git diff --check`：passed。
- 交叉引用已核对：TASK-016 指向 FEAT-016；FEAT-016 指向 ADR-011；`current-status.md` 的 active task 和下一步与任务看板一致。
- 项目根目录缺少约定的 `scripts/validate-state.py`；改用已安装技能提供的 `/Users/owenmacbook/.agents/skills/y-agentic-project-guidelines/scripts/validate-state.py .claw`，结果：passed。

## 2026-07-23 TASK-016 首个实现增量

- fresh Docker PostgreSQL 16 成功执行 migrations 1–7；migration 7 新增角色中心授权、组织树/闭包、主体成员关系、数据范围、对象策略、团队/例外共享/规则定义/`record × group` 投影和 permission snapshot 表。新增租户表均启用 FORCE RLS，仅授予 runtime role。
- `TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope` 在真实数据库覆盖：无角色用户对象访问拒绝；字段写入默认拒绝；未授予字段不会回显；`organization_descendants` 可读取范围内记录；负责人调岗只变更 membership，不改写记录 `data_organization_id`，且 Owner 持续访问。
- `./scripts/test-postgres.sh run`：全量通过（migrations 1–7、全部 Go packages）。本轮未执行 race/vet/发布构建；未访问生产/共享数据库，也未进行远端 Git/CI/部署操作。
- TASK-016 尚未完成：平台管理授权 Capability、共享规则计算/投影刷新与失效、组织合并编排、三入口与跨租户负向覆盖仍待实现。

## 2026-07-23 TASK-016 第二实现增量

- migration 8 新增 `organization_merge_operation` 与 `record_organization_history`，组织不物理删除；merge 以 `running -> completed` 操作保存进度，记录数据归属按最多 1,000 条批次迁移，完成时迁移成员和固定组织范围、重建 closure 并失效 snapshot。
- 新增 9 个 Capability：角色/Permission Set 创建、授权、绑定和分配，直接共享授予/撤销，组织合并启动/执行。管理调用同时需 Capability Scope、角色中的 platform 原子权限；高风险动作还需受信 principal 携带的 approval。
- `TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope` 扩展验证角色/Permission Set 写路径、approval、直接共享即时赋予记录读取、组织合并完成。`./scripts/test-postgres.sh run`：migrations 1–8 与全量 Go packages 通过。
- 尚未实现：组/团队管理、共享规则定义/刷新、`record × group` 投影生成、access explain、角色冲突管理 Capability，以及三入口和跨租户的专门负向测试。

## 2026-07-23 CloudCC Semattice 命名治理验证

- 用户正式确认产品名称 **CloudCC Semattice（语义格）**，产品类别为 Agentic Business Data Runtime；ADR-012 是品牌、CLI/服务前缀和兼容标识边界的唯一事实源。
- `python3 /Users/owenmacbook/.agents/skills/y-agentic-project-guidelines/scripts/validate-state.py .claw`：passed。
- `git diff --check`：passed。
- README 正式标题、ADR-012、goals 与 FEAT-009/011/020 品牌引用存在性检查通过；未发现仍把 `AI Native Platform` 声明为正式标题或正式产品名称的冲突文本。
- 本次只修改命名与项目治理文档；未改动、运行或重新验证 Go/PostgreSQL 运行时代码。现有 `native_platform`、`ai-native-platform`、`AI_NATIVE_*` 和 Go module path 继续作为兼容标识。

## 2026-07-23 TASK-016 第四实现与发布门禁

- fresh PostgreSQL 16 按 checksum 执行 migrations 1–9；migration 9 记录共享规则的投影状态、游标、修订和错误字段。`record × group` 投影按最大 1,000 条批次刷新，只有 `active + ready` 规则可授权，构建中不暴露记录。
- 集成数据库用例覆盖对象策略受审批发布、组成员、团队、规则投影、直接共享、授权 explain 审计、职责分离、调岗 Owner 连续性及组织合并取消/完成。权限包新增未由操作者持有的原子权限返回 `UNAUTHORIZED`。
- 授权表完整纳入 FORCE RLS、非 owner 和 control 无权断言；runtime 缺失 TenantContext 读取授权角色为 0，错误 tenant/bucket 写入授权角色被拒绝。
- `TEST_DATABASE_URL=postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`：passed。
- `GOTOOLCHAIN=go1.26.5 go vet ./...`、`go mod verify`、`git diff --check`、`CGO_ENABLED=0 -trimpath` 的 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 构建：passed。
- 本轮是 maker 自验，未执行生产/共享数据库、远端 Git 写入、CI、制品发布或部署；百万记录、50 并发和热点组织规则投影容量结论仍属于 TASK-019。

## 2026-07-23 TASK-016 生命周期与投影可靠性复验

- migrations 10/11 分别加入无组织锚点数据范围的幂等约束，以及新建/组织迁移记录的 ready-rule `record × group` 投影触发器；测试验证新记录即时获得规则投影。
- 角色撤销、snapshot 失效、受审批数据范围配置及失败规则重试均有 Capability 与数据库集成覆盖；失败规则重试清理旧投影并回到 `building`，未 refresh 到 `ready` 前不产生授权。
- `TEST_DATABASE_URL=postgres://postgres@127.0.0.1:55432/postgres?sslmode=disable GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、状态校验、`git diff --check` 与 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 `CGO_ENABLED=0 -trimpath` 构建：passed。

## 2026-07-23 百万记录基准复跑尝试

- opt-in `TestRecordPhysicalBenchmark` 两次运行均在查询门禁处返回 `FAILED_PRECONDITION: filter field has no active typed index: name`。当前历史基准直接发布 metadata，未按现行 Changeset 流程完成 indexed field 的 active 生命周期；这证明门禁仍生效，不构成性能结果。
- 未保留任何绕过索引生命周期的测试改动。TASK-019 必须先将基准模型改为完整 Changeset 激活路径，再记录百万记录与热点组织的真实性能结果。

## 2026-07-23 TASK-016 本地百万记录授权模拟

- 专用本机 PostgreSQL 16 容器执行：`TEST_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:55432/postgres?sslmode=disable AI_NATIVE_RUN_AUTHORIZATION_SIMULATION=1 AI_NATIVE_AUTHORIZATION_SIMULATION_RECORDS=1000000 AI_NATIVE_AUTHORIZATION_SIMULATION_CONCURRENCY=50 GOTOOLCHAIN=go1.26.5 go test ./internal/record -run TestAuthorizationLocalSimulation -count=1 -v`。
- 当前谓词复验结果：1,000,000 条记录，规则投影只生成 100,000 条 `record × group` 边；组织闭包数据范围与仅组共享的两个边界断言均通过，未生成 `record × user` ACL。批量写入耗时 19,849.874 ms；50 个运行时租户连接并发读取的 p50 为 297.759 ms，p95 为 355.822 ms。
- 同轮 `GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、状态校验与 `git diff --check` 均通过。仅使用本机专用数据库，未访问生产/共享数据库、未执行远端写入或部署。
- 该结果是 FEAT-016 的本地回归与容量证据，不替代 TASK-019 对 200 活跃用户、热点组织公平性及 8/16 GiB 档位的完整验收，也不构成生产 SLA。

## 2026-07-23 TASK-016 投影一致性与组生命周期收口

- `TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope` 新增真实数据库回归：先以一条为批次建立投影，再让已越过游标的记录离开并回到规则组织；末批刷新必须返回 `building` 并重置游标，下一轮完整 catch-up 后才可转为 `ready`。因此 building 期间的并发变更不会造成“ready 但缺 share edge”。
- 同一用例验证 group rule 对一个无组织范围的主体实际授权、`access_group.lifecycle_state=disabled` 立即拒绝该路径、恢复 active 后重新可读、结束 membership 后再次拒绝。记录 PDP 与 access explanation 的组共享查询均要求 active group。
- `TEST_DATABASE_URL=... GOTOOLCHAIN=go1.26.5 go test ./internal/record -run TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope -count=1 -v`：passed；100,000 记录/10,000 投影/50 并发的本地授权模拟复验亦 passed（p50 51.396 ms、p95 65.718 ms）。
- 当前代码完整门禁：`TEST_DATABASE_URL=... GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 `CGO_ENABLED=0 -trimpath go build`、状态校验与 `git diff --check`：全部 passed。

## 2026-07-23 TASK-016 条件数据范围

- `authorization.role.set-data-scope` 的 `conditional` 只接受 `condition.equals`：1–5 个合法数据字段，各值仅可为 string/number/boolean。服务将其规范化为 JSONB containment 对象，记录 get/query/update/delete 共用参数化 `data @> condition` 谓词；嵌套对象、数组、null、未知 envelope 和任意 SQL 文本均被稳定拒绝。
- 真实 PostgreSQL 集成覆盖：只有 `name=Scoped customer` 的记录被条件角色读取，另一条记录拒绝；`access.explain` 返回 `conditional_scope`，API/MCP/CLI 成功与非法条件错误保持等价。
- 当前实现复跑：1,000,000 records、100,000 `record × group` projections、50 concurrent runtime readers；insert 20,220.986 ms，query p50 285.095 ms、p95 353.419 ms，passed。该本机实测不构成生产 SLA 或 TASK-019 的完整容量验收。

## 2026-07-23 TASK-016 组织重组多批可靠性

- 真实数据库集成用例先取消一个无记录 merge（验证只在 0 已迁移记录时可取消），随后在源组织创建两条受保护记录并重启 merge。源组织改为 `migrating` 后，owner 的 `runtime.record.create` 返回 `FAILED_PRECONDITION`，不会为正在退休的组织创建新的 data anchor。
- `organization.merge.execute` 以 `batch_size=1` 连续执行两次：第一批保持 `running` 且 `records_migrated=1`，第二批才变为 `completed` 且 `records_migrated=2`；两条记录的 `data_organization_id` 和 owner 的主成员关系均迁移至目标组织。
- `TEST_DATABASE_URL=... GOTOOLCHAIN=go1.26.5 go test ./internal/record -run TestEnforcedObjectUsesRoleFieldAndOrganizationDataScope -count=1 -v`：passed。

## 2026-07-23 TASK-016 三入口与最终本机门禁

- 受保护对象的真实集成用例现通过 API、MCP、CLI 分别验证：无对象 read 权限返回 `UNAUTHORIZED`；未经字段 write 授权的 create 返回 `UNAUTHORIZED`；条件数据范围对匹配记录允许、对不匹配记录返回 `RESOURCE_NOT_FOUND`。适配器的 HTTP 状态同时按稳定错误码验证为 403、403、404，CLI 和 MCP 错误消息保持一致。
- 当前最终门禁：`TEST_DATABASE_URL=postgres://postgres:postgres@127.0.0.1:55432/postgres?sslmode=disable GOTOOLCHAIN=go1.26.5 go test -race ./... -count=1`、`go vet ./...`、`go mod verify`、linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 `CGO_ENABLED=0 -trimpath go build`、状态校验与 `git diff --check`：全部 passed。

## 2026-07-24 TASK-026 发布前质量门

- AgentCiCi 与 Semattice 的真实本机 HTTP 联调通过：AgentCiCi 创建组织后，以内部 HMAC 调用 Semattice 受控开户入口；reserve、projection、complete 全部成功，同一幂等键重试没有新增 tenant 或 operation。
- `./scripts/test-postgres.sh run`、`go test -race ./...`、`go vet ./...`、`go mod verify` 和 `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GOTOOLCHAIN=go1.26.5 go build -trimpath -ldflags='-s -w' -o /tmp/semattice-release-check ./cmd/ai-native-platform`：全部通过。
- 修复并覆盖无数据库 MCP/CLI 模式下空计量服务的接口 nil 陷阱；公开 `tenant.provision` 能力保持未发布，数据库角色接线测试以已验证的 tenant fixture 覆盖 metadata 运行时路径。

## 2026-07-24 TASK-026 生产发布验收

- 专用 `semattice_migrator` 成功执行生产正向 migration，`schema_migration` 达到 version 16；临时 `CREATEROLE` 已在迁移完成后撤销。运行时 control/runtime 身份未用于 migration。
- 新二进制 SHA-256 `c1617398e9ddf3b83a942fa8b5852e54f7caf943900771703e8b1bacbf712962` 经远端校验后，`/opt/semattice/current` 原子指向 `20260724T0045Z-controlled-provisioning`；`systemctl` active、edge health 通过，近 20 分钟日志无 panic/fatal。
- 跨生产主机 HMAC smoke：匿名请求被 403 拒绝；来自 AgentCiCi 的有效签名请求经过 Semattice 和 AgentCiCi 双向校验后，因专门构造的不存在组织返回 `FAILED_PRECONDITION` / HTTP 412。该负向探针未创建 tenant、reservation 或 operation，证明 fail-closed 与无副作用失败路径。
