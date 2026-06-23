# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

## [0.5.0] - 2026-06-23 (80.5)
### Added
- Dashboard now shows a launchable base-checkout row for every project: it displays the branch the project root is on and its clean/dirty state, and `t` opens a tmux window on the base branch at the project root, `e`/`o` open it in an editor, and `enter` shows the root's uncommitted diff — so the trunk can be launched without first creating a worktree
- Manage projects from the dashboard: press enter on a project row to open a centered popup menu and rename the project or delete (unregister) it, with arrow-key navigation and a colour-coded confirmation card that warns when a delete would drop registered workspaces
- New `wf project rename <old> <new>` command renames a registered project and retargets its worktrees
### Changed
- Redesigned the dashboard with a polished lipgloss look: a full-width header bar with a project/worktree summary, a Catppuccin colour theme, a highlighted selected row, and a full-width footer help bar
- Reworked the dashboard ledger into a clean per-project block: a header line with the project name and home-shortened path, the base checkout on its own row (marked ◆, showing the root branch and clean/dirty state with the ahead/behind, diff, and base columns left blank), and worktrees as aligned tree children — the base row and worktree rows now line up under one set of column headings, with a blank line spacing each project apart. Per-worktree directory paths were dropped from the rows to cut clutter (the path stays on the project header), and the base-launch keys (`t`/`e`/`o`/`enter`/`c`) act on the base row
- Dashboard prompts (add-workspace input, and the merge/remove/delete confirmations) now render as centered, themed popup cards over the ledger — red for destructive actions, green when safe — instead of a plain bottom-line prompt

## [0.4.0] - 2026-06-19 (79.2%)
### Added
- tmux integration (auto-detected, guest-only): `wf add` gives each new workspace a real tmux window in your current session, one window per workspace
- `wf close` closes a workspace's tmux window while keeping the worktree and branch
- `wf resurrect` recreates tmux windows for every tracked workspace after a tmux or computer restart
- `wf sidebar` shows a live strip of the workspace windows open right now (run it in a split pane)
- Dashboard: press `t` to jump to a workspace's tmux window, and a `▣` marks workspaces whose window is open right now
- Added backlink to repo in yaml file in project created by init
- New documentation website (VitePress) covering the guide and command/configuration reference; its Changelog link points to the repository's `CHANGELOG.md` on GitHub
- `wf edit` opens a workspace in an editor: it discovers the IDEs and editors installed on this machine (VS Code, JetBrains IDEs, Sublime, Zed, Vim, and more) and shows a picker to choose one — arrow keys to move, Enter to launch
- Per-project default editor: set `default_ide` in a repo's `.workFlow.yaml` (or pin it from the picker) so each project can prefer its own IDE, with optional `autolaunch` to open it without the picker
- Custom editors can be added under `ides:` in the global config so the picker can launch an editor wf doesn't ship in its catalog
- Dashboard: `e` opens the editor (autolaunching the project default when set, else a picker) and `o` configures it; the picker appears as a popup over the ledger
- `wf edit --list` prints the editors detected on this machine and their ids
- Editor discovery finds snap-packaged IDEs via their Ubuntu menu entries and skips stale JetBrains Toolbox launcher scripts that point at a removed snap revision, so the picker resolves a launcher that actually works
- Launching a GUI editor reports a real error when the launcher fails to start (e.g. a broken wrapper script) instead of falsely reporting that it launched
- Live agent status for each workspace, shown inside the tmux tab (a working/waiting/idle icon prefixed to the window name, with the tab recolored by state), in the dashboard, and in the sidebar
- `wf hooks install` / `uninstall` / `print` to manage the Claude Code hooks that report agent status (works with any agent that can run a command on its lifecycle events)
- `status:` config block to customize the status icons and colors (presets nerdfont/emoji/ascii, per-state glyph/color overrides, tmux color mode, and a staleness TTL)
### Changed
- `wf open` now jumps to the workspace's tmux window when run inside tmux (creating it on demand); use `--editor` to open the editor instead. Outside tmux it opens the editor as before
- `wf merge` and `wf rm` now also close the workspace's tmux window as they clean up
- Dashboard ledger now shows a labelled column header and a legend explaining every status glyph and column
- Dashboard workspace rows are accent-coloured per field (status, commit gap, diff counts, dirty and tmux markers) rather than colouring the whole line, so colour flags a specific status instead of the entire row
- Dashboard shows each branch's commit gap to base as a single labelled "behind|ahead" column, replacing the unlabelled arrow pair
- Renamed the clean workspace state in the dashboard from "done" to "clean" and gave it its own colour, so it reads as a normal state alongside "active" rather than a finished one
- Dashboard: the `x` (remove) confirmation now checks the workspace first — it warns in red and spells out what would be lost (uncommitted changes and/or unmerged commits) when there is work at risk, and reassures in green that the branch is safe to drop when there isn't
- Outside tmux, `wf open` now launches the workspace's default editor instead of a single configured editor command
- The dashboard now updates agent status live (no manual refresh) and the sidebar's agent-status slot is filled instead of showing a placeholder
### Removed
- Removed the global `editor` config field (superseded by the discovered-editor model and per-project `default_ide`); a leftover `editor:` key is ignored and dropped on the next config save
### Fixed
- Dashboard: long branch names no longer break the table — the branch column is wider, and any name still too long is truncated with a `…`, so the state, ahead/behind, diff, and base columns stay aligned

## [0.3.1] - 2026-06-15 (79.5%)
### Added
- `wf init` and `wf add` now offer to register the current repo as a project so it appears in the dashboard, prompting interactively (showing the project name and path) or registering straight away with `--yes`
### Changed
- `wf init` now detects the repo's default base branch (git's tracked default, otherwise preferring `development` over `main`/`master`) and, on an interactive terminal, prompts to confirm or pick it when several branches could be the base — instead of always writing `base: main`
### Fixed
- `wf add` no longer fails with `fatal: invalid reference: main` in repositories without a `main` branch, where `wf init` had hardcoded `base: main`
- `wf add` now verifies the base branch exists first and, when it doesn't, reports a clear message listing the available branches and how to set a base — instead of surfacing git's raw `fatal: invalid reference` error

## [0.3.0] - 2026-06-15 (79.3)
### Added
- Locally built binaries now report a `-dev` version with the short commit hash (and a `.dirty` marker when the working tree has uncommitted changes), so they are distinguishable from official releases
- `make install` target builds and installs `wf` to the same location as the installer (`~/.local/bin`, overridable via `INSTALL_DIR`)
- Tests added and cilint issues fixed
- Interactive dashboard TUI: a cross-project ledger of projects and their worktree workspaces with live git status (ahead/behind, ±lines, dirty) and an active/done flag, plus manual and automatic refresh
- Dashboard diff viewer: review a workspace's scrollable, colorized diff against its base branch
- Dashboard actions wired to the engine: create a workspace, open it in your editor, copy its path, merge it, and remove it
### Changed
- Running `wf` with no arguments now opens the dashboard in an interactive terminal, and still prints the plain workspace list when piped or redirected
- `wf merge` now uses a default merge commit message instead of opening an editor, so merges complete non-interactively

## [0.2.2] - 2026-06-15
### Changed
- Release workflow is now tag-driven: it triggers only on pushed version tags and no longer creates tags itself

## [0.2.1] - 2026-06-15
### Fixed
- corrected the github location and repo name

## [0.2.0] - 2026-06-15
### Added
- Added version number and github actions code for releaseing

## [0.0.1] - 2026-06-15
### Added
- Initial build adding the basic git-worktree features
- Updated completions to auto install based on the current terminal being used

# Notes
[Deployment] Notes for deployment
[Added] for new features.
[Changed] for changes in existing functionality.
[Deprecated] for once-stable features removed in upcoming releases.
[Removed] for deprecated features removed in this release.
[Fixed] for any bug fixes.
[Security] to invite users to upgrade in case of vulnerabilities.
[YANKED] Note the emphasis, used for Hotfixes
