# Change Log
All notable changes to this project will be documented in this file.
This project adheres to [Semantic Versioning](http://semver.org/).

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
