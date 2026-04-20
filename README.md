# tf-slate

[![Go Version](https://img.shields.io/github/go-mod/go-version/abuxton/tf-slate?style=flat-square&logo=go)](https://github.com/abuxton/tf-slate/blob/main/go.mod)
[![Top Language](https://img.shields.io/github/languages/top/abuxton/tf-slate?style=flat-square&logo=go)](https://github.com/abuxton/tf-slate)
[![License](https://img.shields.io/github/license/abuxton/tf-slate?style=flat-square)](https://github.com/abuxton/tf-slate/blob/main/LICENSE)
[![Last Commit](https://img.shields.io/github/last-commit/abuxton/tf-slate?style=flat-square&logo=github)](https://github.com/abuxton/tf-slate/commits/main)
[![Contributors](https://img.shields.io/github/contributors/abuxton/tf-slate?style=flat-square&logo=github)](https://github.com/abuxton/tf-slate/graphs/contributors)

`tf-slate` is a Go utility that discovers local Terraform state files (`*.tfstate`), summarizes each file, and provides an interactive terminal UI for follow-up operations.

## Features

- Recursively scans a filesystem path for Terraform state files (Linux and Windows path compatible via Go `filepath` APIs)
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

## Test

```bash
go test ./...
```
