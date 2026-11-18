# tf-slate

[![Go Version](https://img.shields.io/github/go-mod/go-version/abuxton/tf-slate?style=flat-square&logo=go)](https://github.com/abuxton/tf-slate/blob/main/go.mod)
[![Top Language](https://img.shields.io/github/languages/top/abuxton/tf-slate?style=flat-square&logo=go)](https://github.com/abuxton/tf-slate)
[![License](https://img.shields.io/github/license/abuxton/tf-slate?style=flat-square)](https://github.com/abuxton/tf-slate/blob/main/LICENSE)
[![Last Commit](https://img.shields.io/github/last-commit/abuxton/tf-slate?style=flat-square&logo=github)](https://github.com/abuxton/tf-slate/commits/main)
[![Contributors](https://img.shields.io/github/contributors/abuxton/tf-slate?style=flat-square&logo=github)](https://github.com/abuxton/tf-slate/graphs/contributors)

`tf-slate` is a Go utility that discovers local Terraform state files (`*.tfstate`), summarizes each file, and provides an interactive terminal UI for follow-up operations.
`tf` as in [Terraform](https://developer.hashicorp.com/terraform/cli/commands/state) and `slate` as in getting a ["clean slate"](https://dictionary.cambridge.org/dictionary/english/clean-slate).
A terraform helper utility to find and manage your terraform state files on your local file system.

## Features

![tf-slate interactive interface](docs/media/tui-demo.gif)

- Recursively scans a filesystem path for Terraform state files (Linux and Windows path compatible via Go `filepath` APIs)
- Optional summary output that groups discovered state files into `0 resources` and `> 0 resources`
- Optional filtering to show only state files with resources
- Optional weighted ordering to list the most resource-heavy state files first
- Shows:
  - file path
  - resource count (managed resources)
  - provider/platform names
  - terraform version + serial metadata
- Full-screen interactive interface (Bubble Tea) that is the default on a terminal; `--classic` or a non-TTY falls back to line-based prompts. It suggests and can run:
  - `terraform state list` with a nested picker so you can inspect a listed resource or go `back`
  - `terraform state show`
  - `terraform destroy` (with explicit confirmation)
  - Terraform commands run from the state file's directory, so `show` and `destroy` need the configuration (`*.tf`) and an initialized `.terraform/` there; otherwise tf-slate explains what is missing. `-state` is only passed for state files not named `terraform.tfstate`.
  - `delete`, which permanently removes the selected state file after typing `DELETE` (warns if it still tracks resources and optionally removes the `.backup` file); it does not destroy infrastructure
  - `visit`, which opens a shell in the selected state file directory and exits the interactive client

##  Installation

### Homebrew

This repository is its own Homebrew tap (`Formula/tf-slate.rb`), so the tap
name points at the repo with an explicit URL:

```
brew tap abuxton/tf-slate https://github.com/abuxton/tf-slate
brew trust --formula abuxton/tf-slate/tf-slate
brew install tf-slate
tf-slate -v
```

Upgrade with `brew upgrade tf-slate`; remove with `brew uninstall tf-slate`
and `brew untap abuxton/tf-slate`. Terraform is optional and only needed for the
interactive list/show/destroy follow-ups.

Starting with the first release published after the prebuilt archives were
introduced, the formula installs the prebuilt binary for your platform (macOS or
Linux, `amd64` or `arm64`) and needs no Go toolchain. Until then, and for
`brew install --HEAD tf-slate`, it builds from source (Go required at build time).

### Prebuilt archives

Each release attaches `tf-slate_<version>_<os>_<arch>.tar.gz` for `darwin` and
`linux` on `amd64` and `arm64`, plus a `checksums.txt` (SHA256):

```
curl -LO https://github.com/abuxton/tf-slate/releases/download/vX.Y.Z/tf-slate_X.Y.Z_linux_amd64.tar.gz
curl -LO https://github.com/abuxton/tf-slate/releases/download/vX.Y.Z/checksums.txt
shasum -a 256 -c --ignore-missing checksums.txt
tar -xzf tf-slate_X.Y.Z_linux_amd64.tar.gz tf-slate
```

macOS binaries are not yet signed or notarized, so Gatekeeper may quarantine a
manually downloaded binary (`xattr -d com.apple.quarantine tf-slate`); Homebrew
installs are unaffected. `.deb`/`.rpm` packages are planned.

Maintainers: `task brew:audit` lints the formula, `task brew:test` installs it
from the checkout, and `task brew:update VERSION=x.y.z` bumps it. The Release
workflow opens a formula-bump PR automatically after each release, generating the
formula from the release's `checksums.txt`. `task package:snapshot` builds the
archives locally (requires `goreleaser`).

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

Use `-non-interactive` or `--ni` to print summaries without prompts. On a terminal the interactive mode is a full-screen interface (arrow keys or `j`/`k`, `enter`, `esc` to go back, `z` to show or hide zero-resource state files, `q` to quit); use `--classic` for the line-based prompts.

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
- Interactive mode defaults to `-nz -w` (non-zero, weighted); pass `-nz=false` or `-w=false` to change that, or press `z` in the full-screen interface to reveal empty state files. Non-interactive output is unchanged.
- `--classic` to use the line-based prompts instead of the full-screen interface
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

# Re-record the README demo GIF (needs vhs, ttyd and ffmpeg)
task docs:demo
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
2. Creates an annotated git tag (`v0.1.0`) on the merge commit.
3. Builds per-OS/arch archives with GoReleaser (`darwin`/`linux` on `amd64`/`arm64`,
   version string embedded) plus `checksums.txt`.
4. Publishes a GitHub Release with auto-generated release notes, the archives
   and the checksums.
5. Opens a PR bumping `Formula/tf-slate.rb` to the new tag (requires
   "Allow GitHub Actions to create and approve pull requests").

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
| Actions PRs | Settings → Actions → General | Allow GitHub Actions to create pull requests (formula bump PR) |
| Branch protection (recommended) | Settings → Branches → Add rule | Require CI to pass before merging `main` |

## Test

```bash
go test ./...
```
