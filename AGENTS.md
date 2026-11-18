# AGENTS.md

Single source of truth for agent governance in this repository. Human-facing docs live in [README.md](./README.md). [.github/copilot-instructions.md](./.github/copilot-instructions.md) is only a thin pointer to this file; path-scoped, generic style guides live in [.github/instructions/](./.github/instructions/). If rules conflict, this file wins. Update this file, not the pointer, when project rules change.

## Project overview

`tf-slate` is a small Go CLI (module `github.com/abuxton/tf-slate`, Go `1.26`, direct dependencies: `golang.org/x/term`, Bubble Tea, Lip Gloss) that recursively discovers local Terraform state files (`*.tfstate`), summarizes them, and optionally offers an interactive interface (Bubble Tea, with a line-oriented `--classic`/non-TTY fallback) wrapping the local `terraform` binary.

| Path | Responsibility |
| --- | --- |
| `cmd/tf-slate/main.go` | Flags, positional root, summary table, classic prompt flow, `terraform` subprocess execution, `visit` shell |
| `internal/state/` | Filesystem scan (`FindStateFiles`) and state parsing (`SummarizeStateFile`) |
| `internal/output/` | Non-interactive formats: `string`, `json`, `yaml`, `csv` |
| `Taskfile.yml` | Canonical dev, validate, build and release automation |
| `.github/workflows/` | `go-ci.yml` (fmt, vet, mod tidy, build, test) and `release.yml` |
| `.agents/skills/`, `.claude/skills/` | Local skill packs (`skills-lock.json` tracks them) |

## Setup and commands

Prefer `task` over raw `go` commands.

```bash
task                 # list tasks
task mod:download    # fetch modules
task dev             # test + build
task validate        # go fmt, go vet, go test
task ci              # validate + build (mirrors GitHub CI)
task build VERSION=v0.0.2   # builds dist/tf-slate with embedded version
task run CLI_ARGS="--ni -o json -root /path"

go test ./...                                   # full suite
go test ./internal/state -run TestSummarizeStateFile   # focused test
```

## Architecture rules

- Keep the package split sharp: parsing/discovery in `internal/state`, rendering in `internal/output`, UX and subprocesses in `cmd/tf-slate`. Never put `terraform` subprocess calls in `internal/state`.
- The default interactive UI is Bubble Tea in `cmd/tf-slate/tui.go` (ADR `docs/adr/0001-tui-library.md`); side effects go through `tuiDeps` so tests drive `Update` directly. Keep the classic prompt loop working as the `--classic`/non-TTY fallback. Interactive mode defaults to `-nz -w` (explicit flags win; `z` toggles empty states in the TUI); `--ni` defaults are unchanged. Regenerate the README GIF with `task docs:demo` when the UI changes.
- Interactive actions are thin wrappers over `terraform state list`, `terraform state show` and `terraform destroy`; `visit` opens a shell in the state file directory and exits the client.
- If `terraform` is missing, print the manual fallback message; do not crash.
- Keep dependencies minimal. Any `go.mod` change must be followed by `task mod:tidy` (CI fails on an untidy `go.mod`/`go.sum`). Keep the `go` directive on a supported Go release.

## Behavioral invariants (do not change casually)

- Output is deterministic: `FindStateFiles` returns sorted paths; providers are sorted.
- Use `filepath` APIs only (Linux and Windows compatible); skip `.git` during traversal.
- Unreadable or invalid state files are skipped with a stderr note; processing continues.
- Destroy requires the exact confirmation string `DESTROY`; deleting a state file requires the exact string `DELETE`, warns when the state still tracks resources, and removes a sibling `.backup` file only if the user agrees. Deletion goes through the injectable `session.deleteFile`.
- Terraform subprocesses must run with `cmd.Dir` set to the state file's directory (providers and config load from the working directory). `show`/`destroy` are guarded by `checkTerraformSetup` (needs `*.tf` and `.terraform/`). Pass `-state` only for non-default state file names (`stateArgs`).
- State parsing: support top-level `resources` and nested `values.root_module`/`child_modules`; ignore `mode == "data"`; count managed resources by `len(instances)` (zero instances counts as `1`); normalize providers to short names (`aws` from `registry.terraform.io/hashicorp/aws`), from `provider` or `provider_name`.
- Summary contract: path, managed resource count, providers, Terraform version, serial.
- CLI flag aliases: `--ni`/`-non-interactive`, `-o`/`-output`, `-s`/`-summarize`, `-nz`/`-non-zero`, `-w`/`-weighted`, `--classic`, `-v`/`--version`. No args prints help; a single positional path equals `-root`. Structured outputs omit the footer summary table. Update README when changing any of this.

## Code style

- Standard Go formatting (`go fmt`), `go vet` clean; follow [.github/instructions/go.instructions.md](./.github/instructions/go.instructions.md) and the `golang-pro` / `golang-patterns` skills.
- Wrap errors with context; return errors rather than panicking; write errors via the injected `stderr` writer (see `run(args, stdout, stderr)`) to keep the CLI testable.
- Comment only what needs clarification.

## Testing

- Table-driven tests, colocated `*_test.go` (`cmd/tf-slate/main_test.go`, `internal/state/state_test.go`, `internal/output/output_test.go`).
- Keep tests fast and hermetic: `t.TempDir()`, inline state fixtures, no real `terraform` or network.
- Cover the happy path plus the specific edge case you introduce. Add tests before refactoring parsing logic.
- Never commit real `*.tfstate` files (they are git-ignored and may contain secrets).

## Git workflow and PRs

- Branch from `main` as `feature/<issue#>-<slug>`, `fix/<slug>`, or `patch/<slug>`; use git worktrees for parallel work (`git worktree add -b feature/x ../tf-slate-x main`).
- Conventional Commits (`feat(cli):`, `fix:`, `chore:`, `docs:`, `style:`); include trailer `Co-authored-by: Copilot <223556219+Copilot@users.noreply.github.com>` on agent-authored commits.
- Before opening a PR run `task ci`; PRs target `main` and must pass the Go CI workflow.
- Do not edit `CHANGELOG.md` manually; `task release:prepare` prepends entries.
- Releases: `task release:prepare VERSION=x.y.z DRY_RUN=true` to preview; merging a `release/vX.Y.Z` PR triggers tagging and the GitHub Release. Avoid `release:tag` unless necessary.
- Homebrew: this repo is its own tap (`Formula/tf-slate.rb`). Releases build darwin/linux amd64/arm64 archives plus `checksums.txt` with GoReleaser (`.goreleaser.yaml`, `task package:snapshot`). `scripts/update-formula.sh` regenerates the whole formula from `checksums.txt`; never hand-edit urls/checksums, use `task brew:update VERSION=x.y.z` (the Release workflow opens this PR). Run `task brew:audit` after formula changes. Keep archive names (`tf-slate_<version>_<os>_<arch>.tar.gz`) in sync between GoReleaser and the script. macOS signing and `.deb`/`.rpm` are planned.

## Security and safety

- Treat state files as sensitive: do not log or print attribute values; only summaries are displayed.
- Destructive operations (`terraform destroy`) must stay behind explicit confirmation. Follow [agent-safety.instructions.md](./.github/instructions/agent-safety.instructions.md).
- Never commit secrets, `.tfvars`, or state files.

## Skills

Prefer local skills over generic advice: `golang-pro`, `golang-testing`, `terraform-engineer`, `test-master`, `code-reviewer`, `git-workflow`, `bash-defensive-patterns` (for shell). When editing a `SKILL.md`, follow [agent-skills.instructions.md](./.github/instructions/agent-skills.instructions.md) and keep `.agents/skills/` and `.claude/skills/` copies aligned.
