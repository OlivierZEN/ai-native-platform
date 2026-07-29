# Semattice Customization Expert Universal

当前版本：[`0.1.1`](VERSION)

`semattice-customization-expert-universal` 是通过统一 HTTPS Capability API 安全操作 CloudCC Semattice（语义格）的 Codex 技能。它帮助 AI 发现平台能力，理解资源模型，并在租户、元数据、记录、用量、授权、组织和共享等领域执行受控操作。

## 能力范围

- 发现线上已发布的 Capability、scope、风险等级和输入 Schema。
- 查看和演进对象、字段、关系及元数据版本。
- 查询、创建、更新和软删除业务记录。
- 查看租户状态与用量信息。
- 配置角色、Permission Set、对象策略、组织、团队和共享规则。
- 使用统一脚本构造请求、执行 dry-run，并检查稳定错误码与审计标识。

本技能只调用 `/v1/capabilities/{capability-id}/invoke`。它不使用 MCP，不直连数据库，不调用内部租户开通接口，也不绕过 OAuth、RBAC、租户隔离、审批、幂等或审计。

## 环境要求

- Codex 技能运行环境。
- Python 3.10 或更高版本；辅助脚本只使用 Python 标准库。
- 可访问的 Semattice HTTPS 服务地址。
- 与目标租户绑定的短期 Bearer Token。

## 安装

安装固定版本：

```bash
git clone \
  --branch v0.1.1 \
  --depth 1 \
  https://github.com/CloudCCAI/semattice-customization-expert-universal.git \
  ~/.codex/skills/semattice-customization-expert-universal
```

安装后在 Codex 中使用 `$semattice-customization-expert-universal` 调用技能。

## 快速开始

先配置环境变量：

```bash
export SEMATTICE_BASE_URL='https://semattice.agentcici.com'
export SEMATTICE_TOKEN='<short-lived-oact>'
```

发现线上能力：

```bash
python3 scripts/semattice_api.py \
  --capability system.capability.list \
  --input '{}'
```

写操作应先执行 dry-run：

```bash
python3 scripts/semattice_api.py \
  --capability runtime.record.create \
  --idempotency-key 'idem-contact-alice-v1' \
  --input '{"object_api_name":"contact","data":{"name":"Alice"}}' \
  --dry-run
```

不要把 Token 放入命令行参数、技能文件、Git 仓库或日志。

## 版本与升级

本项目使用 [Semantic Versioning](https://semver.org/)：

- `MAJOR`：不兼容的技能流程或公开约定变更。
- `MINOR`：向后兼容的能力、工作流或参考资料扩展。
- `PATCH`：向后兼容的修正和文档更新。

根目录 `VERSION` 是版本号的唯一事实源，每个发布版本必须具有完全一致的 Git 标签 `v<version>`。升级到指定版本：

```bash
cd ~/.codex/skills/semattice-customization-expert-universal
git fetch --tags
git checkout v0.1.1
```

发布新版本时，先更新技能内容和 `VERSION`，运行官方技能校验，再提交并创建对应版本标签。不要移动或覆盖已经发布的标签。

## 维护与发布流程

### 双目录边界

- **项目内开发副本**：`<semattice-project>/skill/semattice-customization-expert-universal`，是技能内容的开发源，不初始化 `.git`。
- **独立发布仓库**：项目外单独克隆的 `semattice-customization-expert-universal`，持有 `.git`，远程固定为 `https://github.com/CloudCCAI/semattice-customization-expert-universal`。
- 根目录 `VERSION` 是版本号的唯一事实源。发布仓库只承载从开发副本验证并同步后的发布内容，不在两个目录中并行编辑。

### 1. 准备版本

1. 在项目内开发副本修改技能内容。
2. 按 SemVer 更新 `VERSION`；同步更新 README 中当前版本、安装标签和升级示例。
3. 确认目标 `v<version>` 标签从未发布。已经发布的标签不可移动、覆盖或删除后重建。

### 2. 同步发布仓库

先确认发布仓库工作树干净、main 可快进到远程，再预览同步结果：

```bash
skill_source_dir='/absolute/path/to/cloudcc-semattice/skill/semattice-customization-expert-universal'
release_repo_dir='/absolute/path/to/semattice-customization-expert-universal'

git -C "$release_repo_dir" fetch origin --prune --tags
git -C "$release_repo_dir" switch main
git -C "$release_repo_dir" merge --ff-only origin/main
git -C "$release_repo_dir" status --short --branch

rsync -a --delete --exclude='.git/' --dry-run \
  "$skill_source_dir/" "$release_repo_dir/"
```

人工核对 dry-run 只涉及技能文件后，再执行相同同步并验证两目录一致。`--delete` 用于清除已从开发副本移除的旧文件，因此禁止省略 dry-run，也禁止交换源目录和目标目录：

```bash
rsync -a --delete --exclude='.git/' \
  "$skill_source_dir/" "$release_repo_dir/"

diff -qr --exclude='.git' "$skill_source_dir" "$release_repo_dir"
```

### 3. 发布前校验

在独立发布仓库执行：

```bash
release_version=$(tr -d '\n' < "$release_repo_dir/VERSION")

python3 /path/to/skill-creator/scripts/quick_validate.py "$release_repo_dir"
python3 "$release_repo_dir/scripts/semattice_api.py" --help >/dev/null
git -C "$release_repo_dir" diff --check
git -C "$release_repo_dir" diff --stat
git -C "$release_repo_dir" diff
```

同时确认：

- `VERSION` 是合法的 `MAJOR.MINOR.PATCH`，README 引用同一版本。
- `agents/openai.yaml` 是合法 YAML，默认提示仍使用正确的 `$skill` 名称。
- 辅助脚本语法与无 Token `--dry-run` 通过。
- 仓库不包含 Token、私钥、临时文件、`__pycache__` 或 `.pyc`。

### 4. 提交、打标签并推送

```bash
git -C "$release_repo_dir" add README.md SKILL.md VERSION agents references scripts
git -C "$release_repo_dir" commit -m "release: semattice customization skill v${release_version}"
git -C "$release_repo_dir" tag -a "v${release_version}" \
  -m "semattice-customization-expert-universal v${release_version}"
git -C "$release_repo_dir" push --atomic -u origin main "v${release_version}"
```

只允许普通快进和新标签推送。禁止 force push，禁止复用或移动历史版本标签。原子 push 保证 main 与版本标签同时成功或同时失败。

### 5. 发布后验证

1. 确认本地 HEAD、`origin/main` 和 `v<version>^{}` 指向同一提交。
2. 确认远程 HEAD 指向 main，仓库页面返回 HTTP 200。
3. 读取远程 raw `VERSION`，确认等于本次版本。
4. 读取远程 README，确认标题、安装标签和升级示例正确。
5. 确认本地发布仓库工作树 clean，再记录提交 SHA、标签和验证结果。

## 目录结构

```text
.
├── VERSION
├── SKILL.md
├── agents/openai.yaml
├── references/
│   ├── api-catalog.md
│   ├── api-contract.md
│   ├── capability-workflows.md
│   └── resource-model.md
└── scripts/semattice_api.py
```

详细执行规则见 [`SKILL.md`](SKILL.md)。
