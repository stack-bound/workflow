# WorkFlow (`wf`)

A local-first, CLI-first developer tool that orchestrates **git worktrees** — one
isolated workspace per piece of work — with live git status, review, and merge from
one cockpit. Written in Go.

## Install

```sh
go build -o wf ./cmd/wf
# then put ./wf on your PATH, e.g.
go install ./cmd/wf
```

Requires `git`. A clipboard helper (`xclip`/`xsel`/`wl-clipboard`/`pbcopy`) is
optional, used by `wf copy`.

## Quick start

```sh
cd ~/code/myrepo
wf project add                 # register the current repo as a project
wf add feature-x               # create a branch + worktree (+ run setup)
wf list                        # see every workspace with live git status
wf path feature-x              # print the worktree path (for shell cd)
wf open feature-x              # open the worktree in $EDITOR
wf merge feature-x             # merge into base, then remove worktree + branch
wf rm feature-x                # remove a workspace without merging
```

## Commands

| Command | Description |
|---|---|
| `wf project add [path]` | Register a git repo as a project (default: cwd) |
| `wf project ls` | List registered projects |
| `wf project rm <name>` | Unregister a project (leaves the repo on disk) |
| `wf add <branch>` | Create a branch + worktree workspace and run setup |
| `wf list` (`ls`) | List workspaces with live status (`--json` for scripts) |
| `wf path <branch>` | Print a workspace's filesystem path |
| `wf open <branch>` | Open a workspace in your editor |
| `wf copy <branch>` | Copy a workspace path to the clipboard |
| `wf merge <branch>` | Merge into base, then remove the worktree, branch, and registration |
| `wf rm <branch>` | Remove a workspace (worktree + branch + registration) without merging |
| `wf init` | Write an example `.workFlow.yaml` in the current repo |
| `wf config` | Manage global config (`path`, `show`, `edit`) |
| `wf completions <shell>` | Print a completion script; `wf completions install [shell]` installs it |

### Shell completions

```sh
wf completions install          # auto-detect $SHELL and install
wf completions install zsh      # or name the shell (bash, zsh, fish)
wf completions bash > _wf       # or just print the script to stdout
```

`install` writes to the standard per-user location (`bash-completion`/`fish`
auto-load theirs; for zsh it prints the one-time `fpath` line to add).

A workspace is referenced by its **branch name**. When the same branch exists in
two projects, disambiguate with `--project <name>`.

## Configuration

- **Global** — `$XDG_CONFIG_HOME/workFlow/config.yaml` (editor, clipboard command,
  default base branch, worktree directory). Manage with `wf config`.
- **Per-repo** — optional `.workFlow.yaml` at the repo root. Run `wf init` for a
  documented example:

```yaml
base: main                 # default base branch for new workspaces
worktree_dir: ../wt        # where worktrees go (default: ../<repo>_worktrees)
setup:                     # commands run (sh -c) in each new worktree
  - npm install
copy:                      # repo-root-relative files copied into new worktrees
  - .env.example
symlink:                   # repo-root-relative files symlinked into new worktrees
  - .env
```

The registry (registered projects + worktrees) is the only persisted state, stored
as JSON under `$XDG_CONFIG_HOME/workFlow/`. Git stats and status are derived live on
every command.

## Shell `cd` integration

Add a helper to your shell config to jump into a workspace:

```sh
wfd() { cd "$(wf path "$1")"; }
```
