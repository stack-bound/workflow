# Command Reference

Every `wf` command, argument, and flag. For task-oriented guides, start with
[Getting Started](/guide/getting-started).

`-h, --help` is available on every command. A workspace is referenced by its
**branch name**; when a branch exists in two projects, disambiguate with
`-p, --project <name>`.

[[toc]]

## `wf`

```
wf [command]
```

With no command, opens the [dashboard](/guide/dashboard) on an interactive
terminal, or prints the plain workspace list when stdout isn't a TTY (so
`wf | cat` stays scriptable).

| Flag | Description |
| --- | --- |
| `--version` | Print the version and exit |
| `-h, --help` | Help for any command |

## Projects

### `wf project add`

```
wf project add [path]
```

Register a git repo as a project (default: the current directory). Records the
name and path in the registry; never modifies the repo on disk. **Aliases:**
`wf projects`, `wf proj`.

| Flag | Description |
| --- | --- |
| `--name <name>` | Project name (default: the repo directory name; deduplicated with `-2`, `-3`, …) |

### `wf project ls`

```
wf project ls
```

List registered projects with their workspace counts and paths. **Alias:**
`wf project list`.

### `wf project rename`

```
wf project rename <old> <new>
```

Rename a registered project and retarget its worktrees to the new name.
**Alias:** `wf project mv`. (Also available from the dashboard — press
<kbd>Enter</kbd> on a project header.)

### `wf project rm`

```
wf project rm <name>
```

Unregister a project. Leaves the repository on disk untouched.

| Flag | Description |
| --- | --- |
| `--force` | Remove even if it still has worktrees (drops them from the registry) |

## Workspaces

### `wf add`

```
wf add <branch>
```

Create a branch and a worktree workspace, then run the repo's setup commands and
file [copy/symlink](/guide/configuration#copy-and-symlink) operations. Inside
tmux, also creates a window for it. If the repo isn't registered, offers to
register it first (unless `--project` was given).

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Project to create the workspace in (default: infer from cwd) |
| `-b, --base <branch>` | Base branch (default: repo/global config or the [detected default](/guide/concepts#base-branch)) |
| `--no-setup` | Skip setup commands and file copy/symlink |
| `-y, --yes` | Register the current repo without prompting if it isn't yet |

### `wf list`

```
wf list
```

List all workspaces with live git status. **Alias:** `wf ls`.

| Flag | Description |
| --- | --- |
| `--json` | Output as JSON instead of a table |

Columns: `PROJECT`, `BRANCH`, `STATE` (`active`/`done`), `BASE`, `A/B`
(`+ahead/-behind`), `CHANGES` (`+added -removed`, trailing `*` when dirty),
`PATH`. The `--json` fields are:

```json
{
  "project": "myrepo",
  "branch": "feature-x",
  "base": "development",
  "path": "/home/you/code/myrepo_worktrees/feature-x",
  "active": true,
  "dirty": true,
  "ahead": 2,
  "behind": 0,
  "added": 84,
  "deleted": 12
}
```

(`error` is added per entry when status couldn't be derived.)

### `wf path`

```
wf path <branch>
```

Print a workspace's filesystem path — for shell `cd` integration (see the
[`cd` helper](/guide/shell-integration#cd-helper)).

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Scope to a project when the branch is ambiguous |

### `wf open`

```
wf open <branch>
```

Open a workspace. Inside tmux, jumps to its window (creating it on demand);
otherwise launches the workspace's default editor (repo `default_ide` → global
`default_ide` → first detected). Use [`wf edit`](#wf-edit) to choose interactively.

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Scope to a project when the branch is ambiguous |
| `--editor` | Open in the editor even inside tmux |

### `wf edit`

```
wf edit [branch]
```

Open a workspace in an editor or IDE. With no argument it opens the current
directory; pass a branch to target another workspace. A picker lists the editors
[detected](/guide/editors) on this machine (the repo's default first); pick one
with the arrow keys and <kbd>Enter</kbd>. In the picker, <kbd>d</kbd> pins the
highlighted editor as the repo default and <kbd>a</kbd> also enables autolaunch.
When the repo has autolaunch set, its default opens straight away.

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Scope to a project when the branch is ambiguous |
| `-i, --pick` | Always show the picker, even when autolaunch is set |
| `-l, --list` | List the editors detected on this machine (with their ids) and exit |

### `wf close`

```
wf close <branch>
```

Close a workspace's tmux window, keeping the worktree and branch. **Requires
tmux.**

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Scope to a project when the branch is ambiguous |

### `wf copy`

```
wf copy <branch>
```

Copy a workspace's path to the clipboard (see
[`clipboard_cmd`](/guide/configuration#clipboard-cmd)).

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Scope to a project when the branch is ambiguous |

### `wf merge`

```
wf merge <branch>
```

Merge the branch into its base, then remove the worktree, delete the branch, and
deregister the workspace (closing its tmux window in tmux). Uses a default merge
commit message — no editor opens.

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Scope to a project when the branch is ambiguous |

### `wf rm`

```
wf rm <branch>
```

Remove a workspace — worktree, branch, and registration — **without** merging.

Removal is **self-healing**: a worktree left half-removed by an earlier failure
(its directory orphaned or already deleted) is reconciled to a clean state on a
retry. If the directory can't be deleted from here — for example it holds
root-owned files written by a Docker bind mount — `wf rm` stops with an
actionable error pointing at `sudo rm -rf <path>` (delete the files, then retry)
or [`wf forget`](#wf-forget) (drop the registration and keep the files).

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Scope to a project when the branch is ambiguous |
| `--force` | Remove even with uncommitted changes or an unmerged branch |

### `wf forget`

```
wf forget <branch>
```

Drop a workspace from `wf` — its registry entry and agent-status file — **without**
deleting the worktree directory or its branch. It's the escape hatch for a stuck
or orphaned workspace whose files can't be removed from here (e.g. root-owned
files left by a Docker container) or that you've already cleaned up out-of-band:
it always clears `wf`'s view, leaving the files for you to remove separately
(e.g. with `sudo`). It still prunes git's stale worktree metadata so re-adding the
same path later isn't blocked.

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Scope to a project when the branch is ambiguous |

## Dashboard

### `wf dashboard`

```
wf dashboard
```

Open the interactive TUI ledger. **Aliases:** `wf dash`, `wf ui`. (Bare `wf` does
the same on a TTY.) See [The Dashboard](/guide/dashboard) for the keymap.

## tmux

### `wf resurrect`

```
wf resurrect
```

Recreate tmux windows for all tracked workspaces after a tmux or machine restart.
**Alias:** `wf restore`. **Requires tmux.**

### `wf sidebar`

```
wf sidebar
```

Show a live strip of the workspace windows open right now — run it in a split
pane. **Requires tmux.**

## Agent status

See [Agent Status](/guide/agent-status) for the full picture.

### `wf hooks`

```
wf hooks install      # add the status hooks to ~/.claude/settings.json (idempotent)
wf hooks uninstall    # remove WorkFlow's status hooks again
wf hooks print        # print the hook JSON (for manual setup)
```

Manage the Claude Code lifecycle hooks that drive WorkFlow's live agent-status
icons. The hooks call `wf set-status`; one global install covers every workspace.
`install` and `uninstall` only ever touch WorkFlow's own entries, leaving your
other hooks and settings untouched.

### `wf set-status`

```
wf set-status <working|waiting|done>
```

Record the current workspace's agent status (inferred from the working
directory), shown live in the tmux tab, dashboard, and sidebar. This is the
target of the hooks installed by `wf hooks install` — you rarely run it by hand,
but any agent that can run a command on its lifecycle events can call it. A no-op
outside a registered worktree.

## Setup &amp; config

### `wf init`

```
wf init
```

Write an example `.workFlow.yaml` in the current repo, detecting the base branch,
and offer to register the repo.

| Flag | Description |
| --- | --- |
| `--force` | Overwrite an existing `.workFlow.yaml` |
| `-y, --yes` | Register the repo without prompting |

### `wf config`

```
wf config path     # print the global config file path
wf config show     # print the effective config (and the resolved editor command)
wf config edit     # open the global config in your editor (creating it if needed)
```

Manage the [global config](/guide/configuration#global-config).

### `wf completions`

```
wf completions <bash|zsh|fish|powershell>      # print a script to stdout
wf completions install [bash|zsh|fish]          # install for your shell
```

Generate or install shell completions. `install` auto-detects your shell from
`$SHELL` when none is given (and supports bash, zsh, fish — use
`wf completions powershell` to print the PowerShell script). See
[Shell Integration](/guide/shell-integration#completions).

| Flag (on `install`) | Description |
| --- | --- |
| `--force` | Overwrite an existing completion file |

### `wf version`

```
wf version
```

Print the `wf` version.
