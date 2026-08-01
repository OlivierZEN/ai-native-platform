---
kind: feature-spec
feature_id: FEAT-045
title: Deploy Semattice web Keycloak login
status: verified
owner_role: release-agent
task_ids: TASK-046
related_decisions: ADR-014
related_issues: ISSUE-001
updated_at: 2026-07-31T07:49:52Z
updated_by: root after committed production rollout and real browser acceptance
---

# FEAT-045 - Semattice 网站 Keycloak 登录生产发布

## 目标

从包含真实治理控制台、Semattice自有CLI登录和网站OIDC登录的同一提交构建不可变制品，安全配置现有`semattice-web` Client Secret并原子发布到当前授权Semattice服务器。

## 范围

- 提交当前已经过本地验证的完整工作树，不提交任何Secret、Token、密码或私钥。
- 从Keycloak现有`semattice-web` confidential client读取现有Client Secret；不重新生成、不输出到本地终端或项目文件。
- 服务器创建`/etc/semattice/secrets/semattice-web-client-secret`，权限固定为`root:semattice 0640`。
- 备份并只增加三个`AI_NATIVE_CONSOLE_OIDC_*`环境变量。
- 从提交SHA构建Linux amd64二进制，原子切换应用release、静态站和Nginx配置，保留上一release与配置备份。
- 验证服务、健康检查、匿名治理API、OIDC登录跳转、真实Keycloak登录回调、组织映射和只读Console Session。

## 明确不做

- 不重新生成或展示Client Secret。
- 不修改用户密码、MFA、Organization成员、tenant_registry或业务数据。
- 不删除旧release、备份、Keycloak client或CLI登录配置。
- 不执行治理或业务写操作。

## 验收标准

- 提交不含凭据，远端构建输入与本地HEAD一致。
- Secret文件和环境文件权限符合约束，服务日志不输出Secret或Token。
- 新release、Nginx、PostgreSQL和Keycloak均active且Nginx配置有效。
- `/healthz`为200，匿名`/console/api/overview`为401，`/auth/oidc/login`为303并跳转精确Keycloak authorization endpoint。
- 真实浏览器登录成功后进入`/console/`，Session Cookie为Secure/HttpOnly/SameSite=Lax且不含Keycloak Token。
- 回滚所需上一release、环境备份、Nginx备份和静态站备份均存在。

## 实施结果

- 代码提交：`dcf2b811b7ec88d0685938f6d6564c818ba24314`。
- 生产 release：`/opt/semattice/releases/20260731T074549Z-web-oidc-dcf2b811b7ec`。
- Linux amd64二进制 SHA-256：`d000e922e0231d39cca9040821bc42cdfa7b96411ad782d5b679bd083db93b87`。
- 现有`semattice-web` Client Secret仅在服务器内从Keycloak读取并写入`/etc/semattice/secrets/semattice-web-client-secret`，未重新生成、未进入终端输出或项目文件。
- Semattice、Nginx、PostgreSQL和Keycloak均active；Nginx配置、健康检查、匿名401负例及OIDC 303/S256 PKCE参数验证通过。
- 真实Chrome验收从匿名登录页进入Keycloak并成功回到`/console/`；页面显示企业管理中心、当前租户和退出按钮，控制台错误数为0。
- 上一应用release、环境备份、Nginx备份和静态站备份均保留，可执行原子回滚。
