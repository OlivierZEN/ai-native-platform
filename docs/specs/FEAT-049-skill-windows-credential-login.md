---
kind: feature-spec
feature_id: FEAT-049
title: Semattice skill Windows Credential Manager login
status: implemented
owner_role: integration-agent
task_ids: TASK-058
related_decisions: ADR-014
related_issues: none
updated_at: 2026-08-04T06:25:00Z
updated_by: root after Windows credential implementation
---

# FEAT-049 - Semattice Skill Windows 登录支持

## 背景与目标

`cloudcc-semattice` 1.4.1 的人工登录只为 macOS Keychain 和 Linux Secret Service 提供 refresh token 存储。Windows 用户在浏览器登录前会因找不到受支持的安全凭据库而失败；尝试转入 WSL 也无法复用原生浏览器、用户会话和 Credential Manager。

本次让 Skill 在原生 Windows Python 环境完成同一套 Keycloak Authorization Code + S256 PKCE 登录，同时保持 refresh token 不进入普通文件、命令行或日志。

## 范围与方案

- 使用 Win32 `CredWriteW`、`CredReadW`、`CredDeleteW` 和 `CredFree` 操作当前用户的 Windows Credential Manager，不引入第三方 Python 依赖。
- 使用 Generic Credential，target 为 `cloudcc-semattice/<issuer-client-hash>`，refresh token 按 UTF-8 字节保存。
- Windows 默认将短期 OACT 与非敏感会话元数据缓存到当前用户 LocalAppData；保留原子替换和拒绝符号链接，但不使用 Windows 无意义的 POSIX `0600/0700` mode-bit 判断。
- Windows 直接执行 `py -3 .\scripts\semattice_api.py login`，不探测或依赖 WSL。
- macOS Keychain、Linux Secret Service、外部 `SEMATTICE_TOKEN` 和现有登录协议保持不变。

## 验收标准

- Windows 平台选择 Credential Manager 后端。
- UTF-8 refresh token 可保存、读取、轮换和幂等删除；不存在凭据时要求重新登录，错误不包含凭据内容。
- Windows 默认缓存路径位于 `%LOCALAPPDATA%\CloudCC\Semattice\credentials.json`，读取不被 POSIX 权限位误阻断。
- 认证测试覆盖 Windows 后端、路由和缓存分支；Skill 校验、Python 语法、CLI help/dry-run、版本一致性和 diff check 通过。
- 真实 Windows 用户完成 `login`、`status`、一次只读调用与 `logout` 冒烟后，才把平台验收状态提升为 verified。

## 风险与回滚

- 当前开发环境不是 Windows，Win32 API 行为由 ctypes 合约级回归测试覆盖，仍需真实 Windows 冒烟。
- Credential Manager 不可用时继续 fail closed，不回退到明文 refresh token。
- 回滚到 1.4.1 会恢复 Windows 不支持状态，不影响 macOS/Linux 已有凭据。
