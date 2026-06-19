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
otherwise opens the worktree in your editor.

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Scope to a project when the branch is ambiguous |
| `--editor` | Open in the editor even inside tmux |

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

| Flag | Description |
| --- | --- |
| `-p, --project <name>` | Scope to a project when the branch is ambiguous |
| `--force` | Remove even with uncommitted changes or an unmerged branch |

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
wf config show     # print the effective config (with the resolved editor)
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
