# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

<!-- New release entries are prepended here automatically by `task release:prepare`. -->

## [v0.3.0] - 2026-11-18

- Merge pull request #41 from abuxton/feature/39-readme-badges (aa2d028)
- Merge pull request #40 from abuxton/feature/36-prebuilt-packages (8365a66)
- fix: use patched Go toolchain for security scan (5e88877)
- docs(readme): add Homebrew, Ruby formula and Copilot badges (2dddae0)
- ci(release): harden release workflow and add owner-only release policy (8725b04)
- docs: document prebuilt archives and formula generation (c4f1474)
- feat(release): publish darwin/linux amd64/arm64 archives and checksums (9c53d1c)
- Merge pull request #38 from abuxton/feature/33-bubbletea-tui (1b83777)
- update test (15fbebd)
- update quit message (06191a9)
- docs: re-record TUI demo GIF with the empty-state toggle (1cf8051)
- feat(tui): default interactive view to non-zero and weighted (397fe3c)
- fix(tui): address review feedback (177ede3)
- docs: add TUI demo GIF and task docs:demo (a86b301)
- feat(tui): make a Bubble Tea interface the default interactive mode (2017c90)
- Merge pull request #37 from abuxton/docs/33-tui-spike (b6a51ff)
- docs(adr): evaluate TUI libraries and recommend Bubble Tea (3ff2cb1)
- Merge pull request #35 from abuxton/feature/31-homebrew-tap (49cd94c)
- feat(brew): make the repo a self-contained Homebrew tap (a6ee648)


## [v0.2.0] - 2026-11-18

- Merge pull request #32 from abuxton/feature/29-delete-state-gomod (0f184a0)
- Improve confirmation check for delete action (aadcc80)
- feat(tui): add delete state file action (74b10b3)
- chore(deps): update go directive to 1.26 and dependencies (ddd4cf1)
- Merge pull request #25 from abuxton/dependabot/github_actions/actions/checkout-7 (fbad180)
- Merge branch 'main' into dependabot/github_actions/actions/checkout-7 (64520d6)
- Merge pull request #30 from abuxton/fix/28-destroy-working-dir (500b7c2)
- fix(tui): run terraform in the state file directory (2a46955)
- Merge pull request #24 from abuxton/fix/interactive-client-menus (cc2c93d)
- Merge pull request #27 from abuxton:feature/agents-md (8da39bd)
- chore: update project skills and skills-lock.json (ded1d03)
- docs: add AGENTS.md as governance source and slim copilot-instructions (13d0413)
- build(deps): bump actions/checkout from 6 to 7 (18a6dd5)
- fix: strip ANSI codes for non-terminal output and handle lone ESC without blocking (db44a2a)
- Merge pull request #23 from abuxton/dependabot/github_actions/actions/setup-go-6 (825ff12)
- fix: improve interactive client menus (c504086)
- Merge pull request #22 from abuxton/dependabot/github_actions/actions/checkout-6 (88ababc)
- build(deps): bump actions/setup-go from 5 to 6 (20cabe7)
- build(deps): bump actions/checkout from 4 to 6 (775c56d)
- Merge pull request #21 from abuxton/patch/abc-readme (8291b0f)
- Enhance README with installation and usage instructions (001a93f)
- fix: clean release temp files (34b148f)


## [v0.1.0] - 2026-04-20

- fix: support release changelog insertion on macOS (3ed8f5f)
- style: apply gofmt drift on main (8f798b5)
- fix: make release prepare dry runs safe (75c35bf)
- Merge pull request #18 from abuxton/feature/15-default-help-root (c590617)
- Merge origin/main into feature/15-default-help-root, resolve conflicts (25fe5f5)
- Merge pull request #19 from abuxton/copilot/add-release-task-and-github-action (9f22e51)
- fix: improve awk insertion logic in release:prepare and fix whitespace in release.yml (0725c32)
- Merge origin/main into feature/15-default-help-root, resolve conflicts (422c18d)
- feat: add release:prepare task, release GH Action, CHANGELOG.md, and release workflow docs (8f45ef8)
- Merge pull request #17 from abuxton/feature/14-interactive-visit (ed5d5eb)
- Merge remote-tracking branch 'origin/main' into feature/14-interactive-visit (b9bfbde)
- Merge pull request #16 from abuxton/feature/4-output-formats (a95b517)
- feat(cli): improve default root handling (6a5c22e)
- feat(tui): add visit and nested list navigation (feee9f8)
- feat(cli): add non-interactive output formats (aa31cd3)
- Merge pull request #13 from abuxton/feature/7-version-flags (7548f9b)
- feat(cli): add version flags (072de40)
- fix go.sum ci tests (5ebd005)
- Merge pull request #12 from abuxton/feature/6-taskfile (f348032)
- Merge pull request #11 from abuxton/dependabot/github_actions/actions/setup-go-6 (5363719)
- build(deps): bump actions/setup-go from 5 to 6 (7814969)
- Merge pull request #10 from abuxton/dependabot/github_actions/actions/checkout-6 (1cc08b4)
- feat(build): add Taskfile automation (c8e1a64)
- build(deps): bump actions/checkout from 4 to 6 (601205b)
- Merge pull request #9 from abuxton/copilot/add-go-workflows-to-ci-cd (142170b)
- feat: add Go CI workflows and Dependabot configuration (fdf71af)
- Merge pull request #5 from abuxton/feature/3-state-summary-flags (4fe768e)
- Initial plan (8dfb542)
- update (6ef9f08)
- add binary to .ignore (f600a7e)
- feat(cli): add state summary flags (c1e41fe)
- Merge pull request #2 from abuxton/feature/add-agentic-material (9576fd5)
- additional material (81b2a7c)
- feat(agentic): add agentic skills and workspace metadata (6752ca9)

