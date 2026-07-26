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

## 唯一工作目录与提交边界

- AgentCiCi 的唯一仓库目录是 `/Volumes/AISpace/codehouse/cc-codeup-agentcici_PM`；Semattice 的唯一仓库目录是 `/Volumes/AISpace/codehouse/AI-Native-Platform`。
- AI agent 仅可操作这两个仓库，禁止在其外创建、克隆、扩展或使用任何项目仓库、worktree、临时项目目录或派生工作区。
- 每次修改 Semattice 代码或项目文档后，必须在本仓库内完成相应 Git 提交；不得把改动留在未提交工作区或其他目录。

## Terminology Boundary

- “数据平台”始终且仅指本仓库的 CloudCC Semattice（语义格）：`/Volumes/AISpace/codehouse/AI-Native-Platform`。
- Agent CC / AgentCiCi 及 CloudCC CRM 是外部应用或集成方，禁止将其称为“数据平台”。
- 历史/协议名称 `Native Platform`、`ai-native-platform` 和 `native_platform` 均仍指本系统；正式品牌与兼容边界以 ADR-012 为准。
