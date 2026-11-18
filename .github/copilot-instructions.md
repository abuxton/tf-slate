# Copilot Instructions

All repository governance (project overview, commands, architecture rules, behavioral invariants, testing, git/PR workflow, security) lives in [AGENTS.md](../AGENTS.md). Read and follow it first; it takes precedence over anything else here.

Path-scoped guidance applied automatically by Copilot via `applyTo` front matter:

- [go.instructions.md](./instructions/go.instructions.md): Go source, `go.mod`, `go.sum`
- [terraform.instructions.md](./instructions/terraform.instructions.md): `*.tf` files, if any are added
- [agent-skills.instructions.md](./instructions/agent-skills.instructions.md): `**/skills/**/SKILL.md`
- [agent-safety.instructions.md](./instructions/agent-safety.instructions.md): agent or tool-calling logic

Do not restate project rules in this file; add or change them in `AGENTS.md`.
