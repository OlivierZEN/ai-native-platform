# Archived ADR-005 — Phase 0 Capability Contract PoC Runtime Proposal

> **Status:** superseded on 2026-07-17 by ADR-007 (Go runtime and binary delivery). This historical document is preserved for decision audit only and must not be used as an implementation, dependency, CI, or delivery baseline.

## Original proposal record

- Status: `superseded`
- Date: 2026-07-17
- Replaced by: `ADR-007`
- Background: ADR-004 required one atomic capability to project from a common Capability Contract to functional API, MCP Tool, and non-interactive CLI. The project originally needed a small, verifiable Phase 0 runtime to prove that the three entry points shared invocation, validation, authorization, idempotency, error, and audit semantics.
- Evidence at the time: Node's release page listed v24 (Krypton) as LTS and v26 as Current. The MCP TypeScript SDK described `McpServer`, `StdioServerTransport`, Zod schemas, and JSON-RPC over standard input/output.
- Proposal at the time: use Node.js 24 LTS, TypeScript, ESM, `@modelcontextprotocol/sdk`, and Zod for the limited Phase 0 PoC; API, MCP, and CLI would be transport adapters around one invocation layer. MCP diagnostics would write to stderr and stdio stdout would remain protocol-only. CLI would accept flags or JSON stdin and emit JSON/JSON Lines without menus or prompts.
- Alternatives at the time: Node 26 Current was not selected as an LTS baseline; separate handlers per API/MCP/CLI were rejected because they could drift; a complete CRM production stack was deferred.
- Original acceptance conditions: TASK-009 approval, explicit L2 allowlist, an independent parity verifier, and real Node 24 build/test evidence.
- Historical references:
  - https://nodejs.org/en/about/previous-releases
  - https://ts.sdk.modelcontextprotocol.io/server
  - https://modelcontextprotocol.io/docs/develop/build-server

## Supersession rationale

The user selected Go for production delivery because Agent hosts in domestic, private, and restricted-network environments must not require Node.js, npm, or a `node_modules` tree. ADR-007 is the sole current runtime decision; its Go plan and verification requirements replace every unexecuted part of this proposal.
