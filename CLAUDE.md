# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`wf` (WorkFlow) — a Go CLI (Cobra) that orchestrates git worktrees as isolated workspaces with live git status and optional tmux integration. Single module: `github.com/stack-bound/workflow`. Entry point: `cmd/wf/main.go`; logic lives under `internal/`. Architecture and roadmap: `@ai/specs/build-plan.md` and `@ai/specs/progress.md`.

## Playwright MCP server

If I ask you to use the Playwright MCP server and you can't find it among the
available tools, **stop immediately and tell me you can't see it.** Do not
substitute another browser tool or try to drive Playwright yourself (e.g. via
`npx`, a script, or a cached browser) — just report that the server isn't
connected and wait for me.

## Build & test

```bash
go build -o wf ./cmd/wf   # build the binary
go test ./...             # run tests
golangci-lint run         # lint (config: .golangci.yml)
```

A `Makefile` wraps the common tasks: `make build`, `make test`, `make test-coverage` (runs all tests and prints per-function + total coverage), `make lint`, and `make clean`. The release build is driven by `goreleaser` (`.goreleaser.yaml`).

## Tests — write them for any code you add

**Every change ships with tests.** Do not leave new code untested — it drops
coverage and lets regressions through.

- **Cover what you write.** New function or branch → a test exercises it. Run
  `make test-coverage` after a change and confirm the **total did not drop**; if
  it did, you left something untested. Match the existing per-package style
  (`internal/*/*_test.go`).
- **Unit-test pure logic directly** (parsers, model `Update`/`View` via the
  `step(m, msg)` pattern, slug/sort/format helpers) — this is where most coverage
  comes from and needs no real git/tmux.
- **Integration-test shell-out paths** against an isolated sandbox: a private
  `XDG_CONFIG_HOME` for the registry, a throwaway `git init` repo, and a private
  tmux server (see the tmux section below). Guard with a skip when the binary
  (`git`/`tmux`) is absent.
- **Assert on engine/registry state, not captured stdout, for CLI commands.**
  Most commands print with `fmt.Println`/`Printf` to `os.Stdout`, which the cobra
  test buffer does **not** capture — asserting on it silently passes (and
  `strings.Contains(x, "")` is always true). Read the registry or query the
  sandbox instead. (Commands that use `cmd.OutOrStdout()` — e.g. `list`,
  `resurrect`, `close` — are safe to assert on.)
- It's fine to leave the Bubble Tea program loop (`Run`) and `tea.Cmd` closures
  that only shell out uncovered — that's the established untested surface — but
  cover everything around them.

## Changelog — use `clog`, never hand-edit CHANGELOG.md

This repo tracks changelog entries as YAML **fragments** in `changelog.d/<branch>.yaml`, managed by the `clog` tool (must be on PATH). At release time `clog release` merges all fragments into `CHANGELOG.md` and deletes them. **Do not edit `CHANGELOG.md` directly** — add a fragment instead.

When asked to record changelog changes for a change, use the `/clog` skill (e.g. `/clog add changelog changes for this change`). Entries must be feature-level (a feature added, a bug fixed) — not file-by-file edits.

## Release (tag-driven)

`.github/workflows/release.yaml` triggers **only on pushed `v*` tags** and never creates tags itself. To cut a release: run `clog release` (merges fragments, updates `CHANGELOG.md`), set `VERSION` to the new version, commit, then `git tag vX.Y.Z` and `git push origin vX.Y.Z`. CI verifies the `VERSION` file matches the tag and fails the release if they differ.

## Testing the TUI / driving tmux — use an isolated server, never the default one

The dashboard (`internal/dashboard`) is a Bubble Tea TUI, so end-to-end testing
needs a real PTY. tmux can provide one, **but**:

- **Always use a private tmux server via a dedicated socket: `tmux -L wf_test …`**
  (or `-S /tmp/wf_test.sock`). Commands here can inherit the developer's `$TMUX`,
  so an unsocketed `tmux new-session` lands on the *default server* — the one
  running the interactive session — and disrupts it.
- **Only ever `tmux -L wf_test kill-server`** to clean up. **Never** run
  `tmux kill-server` (kills every server, including the user's) and never
  `kill-session` on the default server.
- Unset/override `$TMUX` for the test command if in doubt. Capture rendered
  output with `tmux -L wf_test capture-pane -p`.
- Prefer unit-testing the model's pure logic (see `internal/dashboard/*_test.go`)
  and the engine via the CLI; reserve the PTY harness for a final smoke check.

## Gotchas

- `VERSION` (repo root) is embedded into the binary via `//go:embed` and **must match** the release tag.
- Worktrees are created as siblings (e.g. `../<repo>_worktrees/<branch>`); per-repo config is `.workFlow.yaml`, global config/registry live under `$XDG_CONFIG_HOME/workFlow/`.
