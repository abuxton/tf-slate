# tf-slate

[![Go Version](https://img.shields.io/github/go-mod/go-version/abuxton/tf-slate?style=flat-square&logo=go)](https://github.com/abuxton/tf-slate/blob/main/go.mod)
[![Top Language](https://img.shields.io/github/languages/top/abuxton/tf-slate?style=flat-square&logo=go)](https://github.com/abuxton/tf-slate)
[![License](https://img.shields.io/github/license/abuxton/tf-slate?style=flat-square)](https://github.com/abuxton/tf-slate/blob/main/LICENSE)
[![Last Commit](https://img.shields.io/github/last-commit/abuxton/tf-slate?style=flat-square&logo=github)](https://github.com/abuxton/tf-slate/commits/main)
[![Contributors](https://img.shields.io/github/contributors/abuxton/tf-slate?style=flat-square&logo=github)](https://github.com/abuxton/tf-slate/graphs/contributors)

`tf-slate` is a Go utility that discovers local Terraform state files (`*.tfstate`), summarizes each file, and provides an interactive terminal UI for follow-up operations.
`tf` as in [Terraform](https://developer.hashicorp.com/terraform/cli/commands/state) and `slate` as in getting a ["clean slate"](https://dictionary.cambridge.org/dictionary/english/clean-slate). 
A terarform helper utility to find and manage your terraform state files on your local file system.

## Features

- Recursively scans a filesystem path for Terraform state files (Linux and Windows path compatible via Go `filepath` APIs)
- Optional summary output that groups discovered state files into `0 resources` and `> 0 resources`
- Optional filtering to show only state files with resources
- Optional weighted ordering to list the most resource-heavy state files first
- Shows:
  - file path
  - resource count (managed resources)
  - provider/platform names
  - terraform version + serial metadata
- Interactive TUI prompts that suggest and can run:
  - `terraform state list` with a nested picker so you can inspect a listed resource or go `back`
  - `terraform state show`
  - `terraform destroy` (with explicit confirmation)
  - `visit`, which opens a shell in the selected state file directory and exits the interactive client

##  Installation

### Brew

```
brew tap abuxton/tap
brew install abuxton/tap/tf-slate
tf-slate -v
tf-slate #same as tf-slate -h
tf-slate -v # returns version
tf-slate . simply usage, search directory tree from $CWD.
```

### Build locally

```bash
git clone https://github.com/abuxton/tf-slate && cd tf-slate
# task https://taskfile.dev/docs/installation
task build
./dist/tf-slate -v # returns `dev` on local build
./dist/tf-slate # same as tf-slate -h
.dist/tf-slate . minimal usage search directory tress from $CWD.
 ```

 
## Usage


```bash
tf-slate
```

```bash
go run ./cmd/tf-slate -root /path/to/search
go run ./cmd/tf-slate /path/to/search
```

Use `-non-interactive` or `--ni` to print summaries without prompts.

Running `tf-slate` with no arguments now prints the CLI help, and a single positional path is treated the same as `-root /path`.

Print the client version:

```bash
go run ./cmd/tf-slate --version
go run ./cmd/tf-slate -v
```

Useful flags:

```bash
# Show the zero-resource vs non-zero-resource summary table
go run ./cmd/tf-slate -root /path/to/search -summarize

# Show only state files that contain managed resources
go run ./cmd/tf-slate -root /path/to/search -non-zero

# Sort by greatest managed resource count first
go run ./cmd/tf-slate -root /path/to/search -weighted

# Combine the summary, non-zero filter, and weighted sort
go run ./cmd/tf-slate -root /path/to/search -summarize -non-zero -weighted

# Emit JSON in non-interactive mode
go run ./cmd/tf-slate --ni --output json -root /path/to/search

# Emit YAML or CSV in non-interactive mode
go run ./cmd/tf-slate --ni -o yaml -root /path/to/search
go run ./cmd/tf-slate --ni -o csv -root /path/to/search
```

In interactive mode, selecting `list` now shows the resources in the chosen state file and lets you either inspect one directly or go `back` to the state file list. Selecting `visit` opens a shell in the selected state file directory so you can work there immediately.

Short aliases are also available for the new flags:

- `--ni` for `-non-interactive`
- `-o` for `-output`
- `-s` for `-summarize`
- `-nz` for `-non-zero`
- `-w` for `-weighted`

Supported non-interactive output formats are `string` (default), `json`, `yaml`, and `csv`. Structured outputs do not include the footer summary table.

## Taskfile automation

This repository now includes a `Taskfile.yml` for common Go development workflows.

```bash
# Show available tasks
task

# Run the local development loop
task dev

# Run format, vet, and tests
task validate

# Build the CLI into dist/
task build

# Build the CLI with an explicit version string embedded
task build VERSION=v0.0.2

# Manage Go modules
task mod:download
task mod:tidy
task mod:update

# Run the skills installer helper
task skills:install
```

## Release workflow

Releases follow a two-step process that keeps tagging and publishing fully
automated while still allowing a peer-review window before anything ships.

### Step 1 — prepare the release (local)

```bash
# Preview what the release branch and CHANGELOG entry would look like
task release:prepare VERSION=0.1.0 DRY_RUN=true

# Create the release branch, update CHANGELOG.md, and open a GitHub PR
task release:prepare VERSION=0.1.0
```

`release:prepare` will:

1. Run `validate` (fmt + vet + tests) to confirm the tree is green.
2. Create a `release/v0.1.0` branch from `origin/main`.
3. Collect all commits since the last tag and prepend a new entry to `CHANGELOG.md`.
4. Push the branch and open a pull request against `main` with the title
   **"Release v0.1.0"** and the changelog diff in the body.

With `DRY_RUN=true`, `release:prepare` now performs the validation and preview
steps only: it fetches `origin/main`, prints the generated changelog entry and
PR body, and does **not** modify the current branch, `CHANGELOG.md`, commits,
or the remote.

### Step 2 — merge the PR (automated)

Once the PR is reviewed and merged, the **Release** GitHub Actions workflow
(`.github/workflows/release.yml`) fires automatically and:

1. Extracts the version from the branch name (`release/v0.1.0` → `v0.1.0`).
2. Builds the binary with the version string embedded.
3. Creates an annotated git tag (`v0.1.0`) on the merge commit.
4. Publishes a GitHub Release with auto-generated release notes and the
   compiled binary as an asset.

### Escape hatch — tag manually

If you ever need to tag a release without the branch/PR flow (e.g., hotfixes):

```bash
# Preview
task release:tag VERSION=0.1.1 DRY_RUN=true

# Tag and push
task release:tag VERSION=0.1.1
```

### One-time repository setup

The following settings are required before the Release workflow can run:

| Setting | Location | Value |
|---------|----------|-------|
| Workflow permissions | Settings → Actions → General → Workflow permissions | **Read and write permissions** |
| `release` label | Settings → Labels → New label | Name: `release` (required for `task release:prepare`) |
| Branch protection (recommended) | Settings → Branches → Add rule | Require CI to pass before merging `main` |

## Test

```bash
go test ./...
```
