# WorkFlow (`wf`)

A local-first, CLI-first developer tool that orchestrates **git worktrees** — one
isolated workspace per piece of work — with live git status, review, and merge from
one cockpit. Written in Go.

## Install

### One-line install

```sh
curl -sSfL https://raw.githubusercontent.com/stack-bound/workflow/master/install.sh | sh
```

This detects your OS and architecture, downloads the latest release, installs the
`wf` binary to `~/.local/bin` (no sudo required), adds that directory to your
`PATH` if it isn't already, and installs shell completions for bash, zsh, or fish.

To install somewhere else, set `INSTALL_DIR`:

```sh
INSTALL_DIR=~/bin curl -sSfL https://raw.githubusercontent.com/stack-bound/workflow/master/install.sh | sh
```

### From source

```sh
go install github.com/stack-bound/workflow/cmd/wf@latest
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
wf open feature-x              # jump to its tmux window (or open it in $EDITOR)
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
| `wf open <branch>` | Jump to the workspace's tmux window (or open it in your editor outside tmux; `--editor` forces the editor) |
| `wf close <branch>` | Close the workspace's tmux window (keeps the worktree and branch) |
| `wf copy <branch>` | Copy a workspace path to the clipboard |
| `wf merge <branch>` | Merge into base, then remove the worktree, branch, and registration |
| `wf rm <branch>` | Remove a workspace (worktree + branch + registration) without merging |
| `wf resurrect` | Open a tmux window for every tracked workspace (after a tmux/computer restart) |
| `wf sidebar` | Live strip of the workspace windows open right now (tmux) |
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

## Dashboard & tmux

Run `wf` with no command to open the **dashboard** — a cross-project ledger of
projects → workspaces with live git status, an active/done flag, a scrollable
diff viewer, and actions (add, open, copy, merge, rm). It works in any terminal;
when stdout is not a TTY, `wf` prints the plain list instead.

When you run inside **tmux**, WorkFlow lights up as a *guest* — it creates real
windows in your current session (one per workspace) that you navigate with your
own keys; it never wraps or owns your session.

- `wf add` gives the new workspace a tmux window; `wf open` jumps to it (creating
  it on demand); `wf close` kills it; `merge`/`rm` close it as they clean up.
- In the dashboard, `t` jumps to a workspace's window and a `▣` marks workspaces
  whose window is open right now.
- After a tmux or machine restart, `wf resurrect` recreates the windows for every
  tracked workspace.
- `wf sidebar` is a thin, always-on strip of the windows open right now — run it
  in a split pane.

Outside tmux these commands fall back gracefully: `open` uses your editor, and
the window-only commands report that no tmux session was detected.

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
