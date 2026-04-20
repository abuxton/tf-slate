# Copilot Instructions

## Scope and instruction layering

- This file is the repository-wide baseline for `tf-slate`.
- Follow the path-specific instruction files in `.github/instructions/` when they apply:
  - `go.instructions.md` for Go source, `go.mod`, and `go.sum`
  - `terraform.instructions.md` for Terraform configuration files if any are added
  - `agent-skills.instructions.md` for `**/skills/**/SKILL.md`
  - `agent-safety.instructions.md` when working on agent or tool-calling logic
- Keep this file repo-specific. Put language- or path-specific detail in `.github/instructions/` rather than duplicating it here.

## Project overview

`tf-slate` is a Go CLI that scans a filesystem tree for local Terraform state files (`*.tfstate`), summarizes each file, prints a table, and optionally offers an interactive prompt for Terraform follow-up commands.

The repository is small and intentionally simple:

- `cmd/tf-slate/main.go` owns CLI flags, printing, prompt flow, and `terraform` subprocess execution.
- `internal/state/state.go` owns filesystem scanning and Terraform state parsing.
- `internal/state/state_test.go` covers the current parser and scanning behavior.
- `go.mod` currently targets Go `1.22`; keep changes compatible with that version unless the module is explicitly updated.

## Build and test commands

```bash
# Show available automation tasks
task

# Run the local Task-based development loop
task dev

# Run the local CI-style checks
task ci

# Build the executable
task build

# Run format, vet, and tests
task validate

# Run the CLI against a directory of local Terraform state files
go run ./cmd/tf-slate -root /path/to/search

# Print summaries only, without interactive prompts
go run ./cmd/tf-slate -root /path/to/search -non-interactive

# Run the full test suite
go test ./...

# Run a focused parser test
go test ./internal/state -run TestSummarizeStateFile
```

Prefer `task` commands for the standard development workflow now that the repository has a `Taskfile.yml`.

## Architecture and behavioral boundaries

- Keep the split between packages sharp:
  - `internal/state` for state discovery, parsing, normalization, and summary data
  - `cmd/tf-slate` for flags, UX, terminal output, prompting, and `terraform` execution
- Do not move Terraform subprocess calls into `internal/state`.
- Keep the CLI flow lightweight. This project uses a line-oriented prompt loop, not a framework-driven TUI.
- Interactive actions are thin wrappers around the local `terraform` binary:
  - `terraform state list`
  - `terraform state show`
  - `terraform destroy`
- If `terraform` is not installed, preserve the current behavior of printing a manual fallback message instead of failing with a stack trace.

## Repository-specific code conventions

- Preserve deterministic output:
  - `FindStateFiles` must return sorted paths.
  - `SummarizeStateFile` must return sorted provider names.
- Preserve cross-platform path handling with `filepath` APIs. Do not hardcode Unix-style separators.
- `FindStateFiles` currently skips `.git` directories during traversal; keep that exclusion unless the repo requirement changes.
- Parsing failures are non-fatal at the CLI layer. The CLI should skip unreadable or invalid state files, report the skip on stderr, and continue processing the rest.
- Preserve the exact destructive-action guard: the destroy flow requires the exact confirmation string `DESTROY`.
- Preserve the current summary contract unless intentionally changing user-facing behavior:
  - include file path
  - include managed resource count
  - include normalized provider names
  - include Terraform version and serial metadata

## Terraform state parsing rules

These behaviors are codebase-specific and should not be changed casually:

- Support both Terraform state layouts already implemented:
  - top-level `resources`
  - nested `values.root_module` / `child_modules`
- Ignore resources where `mode == "data"` when computing `ResourceCount`.
- Count managed resources by `len(instances)`.
- If a managed resource has zero `instances`, count it as `1`.
- Normalize providers to the short provider name such as `aws` from `registry.terraform.io/hashicorp/aws`.
- Provider names may come from either `provider` or `provider_name`.

## Testing expectations

- Update or add tests in `internal/state/state_test.go` when changing filesystem discovery or parsing behavior.
- Keep tests fast and local: use `t.TempDir()`, temporary files, and inline fixture content rather than external dependencies.
- When parser behavior changes, cover both happy-path parsing and the specific edge case being introduced.
- Prefer preserving behavior through tests before refactoring parsing logic.

## Local skills in this repository

The repo also contains local skill packs under `.agents/skills/` and a subset under `.claude/skills/`.

When work overlaps those areas, prefer the local skill guidance over generic advice:

- `golang-pro` and `golang-testing` for Go implementation and tests
- `terraform-engineer` for Terraform and state-management workflows
- `bash-defensive-patterns` if shell scripts are added
- `test-master` for broader testing strategy
- `code-reviewer` for review-oriented tasks
- `git-workflow` for branch, commit, and PR conventions

If you edit a `SKILL.md`, also follow `.github/instructions/agent-skills.instructions.md`. If a corresponding skill exists in both `.agents/skills/` and `.claude/skills/`, keep them aligned unless the repository clearly intends them to diverge.
