# The Dashboard

Run `wf` with no command to open the **dashboard** — an interactive TUI ledger of
every project and its workspaces, with live git status, a scrollable diff viewer,
and actions wired straight to the engine. It works in any terminal; when stdout
isn't a TTY (e.g. `wf | cat`), `wf` prints the plain [list](/reference/commands#wf-list)
instead so it stays scriptable.

```sh
wf            # open the dashboard (on a TTY)
wf dashboard  # explicit; aliases: wf dash, wf ui
```

Here's the ledger layout — projects as headers, workspaces beneath, with a
legend and a context-sensitive help line:

<TerminalHero />

::: info About the panel above
The panel is a styled, hand-built stand-in for the real layout — the project
header, the `◆` base row, a workspace with its agent-status icon, and the legend
and help line. Genuine captures of the live lipgloss dashboard can be generated
with the procedure in
[`docs/capture/`](https://github.com/stack-bound/workflow/tree/master/docs/capture)
— capture target: **`dashboard-ledger`**.
:::

## Reading the ledger

Each project is a block: a header line (`name (count)  path`), then the
project's [base checkout](#the-base-row) on its own row, then its workspaces as
aligned tree children — all under one dim column heading:

| Column | Meaning |
| --- | --- |
| *(agent)* | The live [agent-status](/guide/agent-status) icon — **working** / **waiting** — when an agent is active in the workspace; blank when idle |
| `●` / `○` | **active** (green) — holds work not yet in base · **clean** (blue) — merged, nothing outstanding |
| `branch` | The workspace's branch name (its identity) |
| `state` | The word `active` or `clean` |
| `behind\|ahead` | Commit gap to base: `↓` commits on base this branch lacks, `↑` commits it has on top |
| `diff` | `+added -removed` lines vs base, with a trailing `*` when the working tree is dirty |
| `base` | The base branch this workspace merges into |

A trailing **`▣`** marks a row whose **tmux window is open right now** — derived
live on every refresh. The selected row is highlighted and prefixed with `❯`.

### The base row {#the-base-row}

Every project shows its **base checkout** — the repository root — on its own row,
marked with a **`◆`**, listing the branch the root is currently on and its
clean/dirty state. (The ahead/behind, diff, and base columns are blank for it.)
It makes the trunk a launch target without first creating a worktree: the
launch keys act on it just like a workspace —

- <kbd>t</kbd> opens a tmux window on the base branch **at the project root**,
- <kbd>e</kbd> / <kbd>o</kbd> open it in an [editor](/guide/editors#in-the-dashboard),
- <kbd>c</kbd> copies the root path,
- <kbd>Enter</kbd> shows the root's **uncommitted** diff.

Merge and remove don't apply to the base row (it isn't a workspace).

### Glyph legend

The legend beneath the ledger keys every glyph, including the active
[agent-status](/guide/agent-status) icons (shown here with the emoji preset; the
default is Nerd Font):

```ansi
🤖 working   ⏳ waiting   ● active   ○ clean   ▣ tmux open   ↓behind|↑ahead vs base   +added -removed   * uncommitted
```

## Keybindings

### Ledger (default)

| Key | Action |
| --- | --- |
| <kbd>↑</kbd> / <kbd>k</kbd>, <kbd>↓</kbd> / <kbd>j</kbd> | Move the cursor |
| <kbd>g</kbd> / <kbd>Home</kbd>, <kbd>G</kbd> / <kbd>End</kbd> | Jump to top / bottom |
| <kbd>Enter</kbd> | On a workspace: its **diff**. On a [base row](#the-base-row): the root's uncommitted diff. On a project header: the [project menu](#managing-projects) |
| <kbd>d</kbd> | View the selected workspace's **diff** (same as Enter, but no menu on a header) |
| <kbd>a</kbd> | **Add** a workspace in the selected project (opens a branch-name prompt) |
| <kbd>e</kbd> | **Edit** — open the [editor](/guide/editors): autolaunch the project default if set, else a picker |
| <kbd>o</kbd> | **Configure** the editor — always opens the picker (set the default / autolaunch) |
| <kbd>t</kbd> | Jump to the workspace's **tmux window** (tmux only) |
| <kbd>c</kbd> | **Copy** the workspace path to the clipboard |
| <kbd>m</kbd> | **Merge** the workspace (asks to confirm) |
| <kbd>x</kbd> | **Remove** the workspace (asks to confirm; <kbd>f</kbd> on the prompt forgets instead) |
| <kbd>r</kbd> | Refresh status now |
| <kbd>q</kbd> / <kbd>Ctrl</kbd>+<kbd>C</kbd> | Quit |

The launch keys (<kbd>e</kbd> / <kbd>o</kbd> / <kbd>t</kbd> / <kbd>c</kbd> /
<kbd>Enter</kbd>) act on the [base row](#the-base-row) too, targeting the project
root. The help line at the bottom adapts to your environment — the `t term` hint
only appears inside tmux.

### Diff viewer

Selecting a workspace opens a scrollable, syntax-colored diff against its base
(additions green, deletions red, hunks cyan, metadata dim):

| Key | Action |
| --- | --- |
| <kbd>↑</kbd> / <kbd>↓</kbd> | Scroll |
| <kbd>r</kbd> | Reload the diff |
| <kbd>q</kbd> / <kbd>Esc</kbd> | Back to the ledger |

When a workspace has no changes against base, the viewer shows
`(no changes against base)`.

### Add prompt

Pressing <kbd>a</kbd> opens an inline branch-name input: type a name and press
<kbd>Enter</kbd> to create the workspace, or <kbd>Esc</kbd> to cancel.

### Confirmations

Merge and remove ask for a `y`/`n` confirmation. **Remove** is risk-aware — it
inspects the workspace first and warns in red when removal would discard
uncommitted changes or unmerged commits, and reassures in green when the branch
is safe to drop:

```ansi
Remove acme-api/feature-login? This discards uncommitted changes and 3 unmerged commits — work will be lost. Are you sure? [y/n]
```

The remove prompt also offers <kbd>f</kbd> to **forget** the workspace instead:
unregister it but leave its files and branch on disk. It's the way out when a
removal is blocked because the worktree holds files `wf` can't delete (for
example root-owned files left by a Docker container) — see
[Troubleshooting](/guide/troubleshooting#removing-a-workspace-fails-with-permission-denied).

## Managing projects

Press <kbd>Enter</kbd> on a **project header** row to open a popup action menu
for that project:

| Key | Action |
| --- | --- |
| <kbd>↑</kbd> / <kbd>↓</kbd> | Move between options |
| <kbd>Enter</kbd> | Choose the highlighted option |
| <kbd>Esc</kbd> / <kbd>q</kbd> | Dismiss the menu |

The two options are:

- **Rename** — opens a text input pre-filled with the current name; edit it and
  press <kbd>Enter</kbd> to rename the project (its worktrees are retargeted to
  the new name). Same as [`wf project rename`](/reference/commands#wf-project-rename).
- **Delete** — unregisters the project from WorkFlow (the repository on disk is
  untouched). It asks to confirm, warning in red when the project still has
  registered workspaces that would be dropped.

## Live refresh

While you're idle on the ledger, the dashboard re-derives git status (and the
open-window markers) **every 4 seconds**, so edits in a workspace show up
without pressing <kbd>r</kbd>. The cursor stays on the same workspace across
refreshes.

## Actions run the real CLI

The dashboard isn't a separate code path — its actions invoke the same engine as
the CLI. Creating, merging, and removing briefly suspend the TUI to stream git
and setup output, then return you to an updated ledger. Anything you can do here,
you can also script with [the commands](/reference/commands).
