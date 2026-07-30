# AGENTS

<!-- agentic-project-guidelines:begin -->
## Required Skill

- This project follows the `agentic-project-guidelines` skill.
- Every AI agent must automatically load and use this skill before any project work.
- If the skill is not installed in the current environment, install it first from GitHub: https://github.com/CloudCCAI/agentic-project-guidelines
- After the skill is available, follow its greenfield or brownfield workflow to maintain `README.md`, `AGENTS.md`, `.claw/` or `.ai-dev/`, and `docs/specs/`.

## 必须遵守

- 本项目遵循 `agentic-project-guidelines` 技能。
- 所有 AI 智能体在开始任何项目工作前，都必须自动加载并使用此技能。
- 如果当前环境尚未安装该技能，必须先从 GitHub 安装：https://github.com/CloudCCAI/agentic-project-guidelines
- 技能可用后，必须按技能中的 Greenfield 或 Brownfield 流程维护 `README.md`、`AGENTS.md`、`.claw/` 或 `.ai-dev/` 以及 `docs/specs/`。
<!-- agentic-project-guidelines:end -->

## Terminology Boundary

- “数据平台”始终且仅指本仓库的 CloudCC Semattice（语义格）：`/Volumes/AISpace/codehouse/AI-Native-Platform`。
- Agent CC / AgentCiCi 及 CloudCC CRM 是外部应用或集成方，禁止将其称为“数据平台”。
- 历史/协议名称 `Native Platform`、`ai-native-platform` 和 `native_platform` 均仍指本系统；正式品牌与兼容边界以 ADR-012 为准。

## Semattice Skill Maintenance and Release

本节是 `cloudcc-semattice` 的唯一维护与发布流程。技能 `README.md` 只保留面向安装者的介绍、安装、使用和版本升级说明，禁止将本节的内部同步、校验或推送步骤写入技能 README。

### 双目录边界

- **项目内开发副本**：`skill/cloudcc-semattice`，是技能内容的开发源，不初始化 `.git`。
- **独立发布仓库**：项目外单独克隆的 `cloudcc-semattice-skill`，持有 `.git`，目标远程为 `https://github.com/CloudCCAI/cloudcc-semattice`。
- 根目录 `VERSION` 是版本号的唯一事实源。发布仓库只承载从开发副本验证并同步后的发布内容，不在两个目录中并行编辑。

### 1. 准备版本

1. 在项目内开发副本修改技能内容。
2. 按 SemVer 更新 `VERSION`；同步更新 README 中当前版本、安装标签和升级示例。
3. 确认目标 `v<version>` 标签从未发布。已经发布的标签不可移动、覆盖或删除后重建。

### 2. 同步发布仓库

先确认发布仓库工作树干净、main 可快进到远程，再预览同步结果：

```bash
semattice_skill_source_dir='/absolute/path/to/cloudcc-semattice/skill/cloudcc-semattice'
semattice_skill_release_repo_dir='/absolute/path/to/cloudcc-semattice-skill'

git -C "$semattice_skill_release_repo_dir" fetch origin --prune --tags
git -C "$semattice_skill_release_repo_dir" switch main
git -C "$semattice_skill_release_repo_dir" merge --ff-only origin/main
git -C "$semattice_skill_release_repo_dir" status --short --branch

rsync -a --delete --exclude='.git/' --dry-run \
  "$semattice_skill_source_dir/" "$semattice_skill_release_repo_dir/"
```

人工核对 dry-run 只涉及技能文件后，再执行相同同步并验证两目录一致。`--delete` 用于清除已从开发副本移除的旧文件，因此禁止省略 dry-run，也禁止交换源目录和目标目录：

```bash
rsync -a --delete --exclude='.git/' \
  "$semattice_skill_source_dir/" "$semattice_skill_release_repo_dir/"

diff -qr --exclude='.git' "$semattice_skill_source_dir" "$semattice_skill_release_repo_dir"
```

### 3. 发布前校验

在独立发布仓库执行：

```bash
semattice_skill_release_version=$(tr -d '\n' < "$semattice_skill_release_repo_dir/VERSION")

python3 /path/to/skill-creator/scripts/quick_validate.py "$semattice_skill_release_repo_dir"
python3 "$semattice_skill_release_repo_dir/scripts/semattice_api.py" --help >/dev/null
git -C "$semattice_skill_release_repo_dir" diff --check
git -C "$semattice_skill_release_repo_dir" diff --stat
git -C "$semattice_skill_release_repo_dir" diff
```

同时确认：

- `VERSION` 是合法的 `MAJOR.MINOR.PATCH`，README 引用同一版本。
- `agents/openai.yaml` 是合法 YAML，默认提示仍使用正确的 `$skill` 名称。
- 辅助脚本语法与无 Token `--dry-run` 通过。
- 仓库不包含 Token、私钥、临时文件、`__pycache__` 或 `.pyc`。

### 4. 提交、打标签并推送

```bash
git -C "$semattice_skill_release_repo_dir" add README.md SKILL.md VERSION agents references scripts
git -C "$semattice_skill_release_repo_dir" commit \
  -m "release: cloudcc-semattice v${semattice_skill_release_version}"
git -C "$semattice_skill_release_repo_dir" tag \
  -a "v${semattice_skill_release_version}" \
  -m "cloudcc-semattice v${semattice_skill_release_version}"
git -C "$semattice_skill_release_repo_dir" push --atomic -u origin main \
  "v${semattice_skill_release_version}"
```

只允许普通快进和新标签推送。禁止 force push，禁止复用或移动历史版本标签。原子 push 保证 main 与版本标签同时成功或同时失败。

### 5. 发布后验证

1. 确认本地 HEAD、`origin/main` 和 `v<version>^{}` 指向同一提交。
2. 确认远程 HEAD 指向 main，仓库页面返回 HTTP 200。
3. 读取远程 raw `VERSION`，确认等于本次版本。
4. 读取远程 README，确认标题、安装标签和升级示例正确。
5. 确认本地发布仓库工作树 clean，再记录提交 SHA、标签和验证结果。
