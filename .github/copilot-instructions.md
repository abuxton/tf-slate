# Copilot Instructions

## Build and test commands

```bash
# Run the CLI against a directory of local Terraform state files
go run ./cmd/tf-slate -root /path/to/search

# Print summaries only, without interactive prompts
go run ./cmd/tf-slate -root /path/to/search -non-interactive

# Build the executable
go build ./cmd/tf-slate

# Run the full test suite
go test ./...

# Run a single test
go test ./internal/state -run TestSummarizeStateFile
```

There is no repository-specific lint command checked into this repo today.

## High-level architecture

- `cmd/tf-slate/main.go` is the entire CLI flow. It parses `-root` and `-non-interactive`, resolves the scan root to an absolute path, discovers state files, summarizes each file, prints a table, and optionally enters the interactive prompt loop.
- `internal/state/state.go` contains the domain logic for Terraform state handling:
  - `FindStateFiles` recursively walks the requested root and returns sorted `*.tfstate` paths.
  - `SummarizeStateFile` parses one state file into a `Summary` with resource count, providers, Terraform metadata, serial, and lineage.
- The parser supports two Terraform state layouts:
  - top-level `resources`
  - nested `values.root_module` / `child_modules`
- Interactive actions are intentionally thin wrappers around the local `terraform` binary. The UI suggests and can run `terraform state list`, `terraform state show`, and `terraform destroy`, but only after a state file has already been summarized.

## Key conventions

- Keep filesystem scanning and Terraform-state parsing in `internal/state`; keep user interaction, printing, flag parsing, and `terraform` subprocess execution in `cmd/tf-slate/main.go`.
- Preserve deterministic output. `FindStateFiles` sorts paths, and `SummarizeStateFile` sorts provider names before returning them.
- Resource counting is codebase-specific:
  - ignore resources with `mode == "data"`
  - count managed resources by `len(instances)`
  - if a managed resource has no `instances`, count it as `1`
- Provider names are normalized down to the short provider name (for example `aws` from `registry.terraform.io/hashicorp/aws`) and may come from either `provider` or `provider_name`.
- Parsing failures are non-fatal at the CLI level. `main.go` skips unreadable or invalid state files, reports the skip on stderr, and continues with remaining files.
- The interactive destroy path requires the exact confirmation string `DESTROY`; keep that explicit guard in place if you change the TUI flow.
- Use Go `filepath` APIs for paths. The repo is intentionally written to stay portable across operating systems rather than assuming Unix-style separators.
- When adding parser behavior, keep support for both Terraform state representations already covered by the current code and tests.
