# ADR 0001: TUI library for interactive mode

- Status: Proposed
- Date: 2026-11-18
- Issue: #33

## Context

Interactive mode is hand-rolled in `cmd/tf-slate/main.go` (`promptMenu`,
`promptStateSelection`, raw `x/term` key parsing, ANSI rendering): roughly 400
of its 938 lines. Tests select menu entries by position (`moveDown(n)`), so
adding an action shifts indices. Requirements: navigable lists, resource
detail, confirmation dialogs, resize handling, running `terraform` with its own
prompts, Windows support, no change to `--ni`/`-o` output, and subprocesses
staying in `cmd/tf-slate`.

## Spike

Throwaway prototypes of the state list (cursor, enter, quit) were built outside
the repo and cross-compiled. Version, size and dependency figures are from that
spike; star counts and activity are from the issue's research.

| | Hand-rolled (today) | Bubble Tea v1.3.10 | go-tui v0.24.0 | tview | tcell |
| --- | --- | --- | --- | --- | --- |
| Model | imperative loop | Elm (Model/Update/View) | `.gsx` templates + components | imperative widgets | low level |
| Prototype LOC (state list) | n/a | 58, plain Go | not measured (needs `.gsx` plus generated code) | not prototyped | not prototyped |
| Prototype binary | 4.3 MB (whole app) | 4.6 MB | 2.8 MB (empty program) | not measured | not measured |
| Dependency packages (prototype) | 2 | 19 | 7 | not measured | not measured |
| Windows build | partial | builds (`GOOS=windows`) | builds (`GOOS=windows`) | yes | yes |
| Codegen step | no | no | yes (`tui generate`, drift check in CI) | no | no |
| Run terraform interactively | n/a | `tea.ExecProcess` | suspend/resume support present (`app_suspend_*.go`), not exercised | `Application.Suspend` | `Screen.Suspend` |
| Testability | index-based | `Update` is a pure function | component tests | widget-bound | low |
| Maturity | n/a | ~45k stars, active | pre-1.0, ~450 stars | ~14k stars, mature | mature |

## Decision

Adopt **Bubble Tea** (with Bubbles and Lip Gloss as needed).

- `Update` is a pure function, so tests assert on model state instead of
  menu positions, which directly fixes the brittleness that motivated #33.
- Plain Go: no generator, no extra CLI, no CI drift check, no change to the
  `Taskfile.yml` build flow.
- Mature, widely used, and `tea.ExecProcess` fits the requirement that
  `terraform destroy` prompts work in the user's terminal.
- go-tui is attractive but pre-1.0 (API churn) and the codegen step adds
  permanent tooling cost. Revisit if it reaches 1.0.
- tview is imperative and harder to unit test; tcell is too low level.

Cost: more dependency packages (19 vs 7 in the prototypes) and a slightly larger binary.

## Consequences

- Add a `tui` layer in `cmd/tf-slate` (model, update, view); `internal/state`
  stays UI-free and terraform subprocesses stay in `cmd/tf-slate`.
- Keep behavior: exact `DESTROY` and `DELETE` confirmations, setup guard (#28),
  `visit`, deterministic ordering, graceful non-TTY fallback to the current
  prompt path.
- Replace index-based menu tests with model/`Update` tests plus golden views
  for key screens.
- Implementation is a follow-up PR; this ADR only records the decision.

## Open items for implementation

- Confirm Bubble Tea v1 vs v2 at implementation time.
- Verify `ExecProcess` with `terraform destroy` on macOS, Linux and Windows.
