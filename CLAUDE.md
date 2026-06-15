# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`wf` (WorkFlow) — a Go CLI (Cobra) that orchestrates git worktrees as isolated workspaces with live git status and optional tmux integration. Single module: `github.com/stack-bound/workflow`. Entry point: `cmd/wf/main.go`; logic lives under `internal/`. Architecture and roadmap: `@ai/specs/build-plan.md` and `@ai/specs/progress.md`.

## Build & test

```bash
go build -o wf ./cmd/wf   # build the binary
go test ./...             # run tests
golangci-lint run         # lint (config: .golangci.yml)
```

There is no Makefile; the release build is driven by `goreleaser` (`.goreleaser.yaml`).

## Changelog — use `clog`, never hand-edit CHANGELOG.md

This repo tracks changelog entries as YAML **fragments** in `changelog.d/<branch>.yaml`, managed by the `clog` tool (must be on PATH). At release time `clog release` merges all fragments into `CHANGELOG.md` and deletes them. **Do not edit `CHANGELOG.md` directly** — add a fragment instead.

When asked to record changelog changes for a change, use the `/clog` skill (e.g. `/clog add changelog changes for this change`). Entries must be feature-level (a feature added, a bug fixed) — not file-by-file edits.

## Release (tag-driven)

`.github/workflows/release.yaml` triggers **only on pushed `v*` tags** and never creates tags itself. To cut a release: run `clog release` (merges fragments, updates `CHANGELOG.md`), set `VERSION` to the new version, commit, then `git tag vX.Y.Z` and `git push origin vX.Y.Z`. CI verifies the `VERSION` file matches the tag and fails the release if they differ.

## Gotchas

- `VERSION` (repo root) is embedded into the binary via `//go:embed` and **must match** the release tag.
- Worktrees are created as siblings (e.g. `../<repo>_worktrees/<branch>`); per-repo config is `.workFlow.yaml`, global config/registry live under `$XDG_CONFIG_HOME/workFlow/`.
