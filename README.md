# tf-slate

[![Go Version](https://img.shields.io/github/go-mod/go-version/abuxton/tf-slate?style=flat-square&logo=go)](https://github.com/abuxton/tf-slate/blob/main/go.mod)
[![Top Language](https://img.shields.io/github/languages/top/abuxton/tf-slate?style=flat-square&logo=go)](https://github.com/abuxton/tf-slate)
[![License](https://img.shields.io/github/license/abuxton/tf-slate?style=flat-square)](https://github.com/abuxton/tf-slate/blob/main/LICENSE)
[![Last Commit](https://img.shields.io/github/last-commit/abuxton/tf-slate?style=flat-square&logo=github)](https://github.com/abuxton/tf-slate/commits/main)
[![Contributors](https://img.shields.io/github/contributors/abuxton/tf-slate?style=flat-square&logo=github)](https://github.com/abuxton/tf-slate/graphs/contributors)

`tf-slate` is a Go utility that discovers local Terraform state files (`*.tfstate`), summarizes each file, and provides an interactive terminal UI for follow-up operations.

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
  - `terraform state list`
  - `terraform state show`
  - `terraform destroy` (with explicit confirmation)

## Run

```bash
go run ./cmd/tf-slate -root /path/to/search
```

Use `-non-interactive` to only print summaries.

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
```

Short aliases are also available for the new flags:

- `-s` for `-summarize`
- `-nz` for `-non-zero`
- `-w` for `-weighted`

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

# Manage Go modules
task mod:download
task mod:tidy
task mod:update

# Preview a release tag without creating it
task release VERSION=0.0.2 DRY_RUN=true

# Create and push a semver release tag
task release VERSION=0.0.2

# Run the skills installer helper
task skills:install
```

## Test

```bash
go test ./...
```
