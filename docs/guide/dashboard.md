# The Dashboard

Run `wf` with no command to open the **dashboard** — an interactive TUI ledger of
every project and its workspaces, with live git status, a scrollable diff viewer,
and actions wired straight to the engine. It works in any terminal; when stdout
isn't a TTY (e.g. `wf | cat`), `wf` prints the plain [list](/reference/commands#list)
instead so it stays scriptable.

```sh
wf            # open the dashboard (on a TTY)
wf dashboard  # explicit; aliases: wf dash, wf ui
```

Here's the ledger layout — projects as headers, workspaces beneath, with a
legend and a context-sensitive help line:

<TerminalHero />

::: info Screenshots pending the TUI redesign
The panel above is a styled representation of the real layout. The dashboard is
being reworked in lipgloss, so genuine captures will be added once it's settled.
They can be regenerated any time with the procedure in
[`docs/capture/`](https://github.com/stack-bound/workflow/tree/master/docs/capture)
— capture target: **`dashboard-ledger`**.
:::

## Reading the ledger

Each project is a header (`name (count)  path`); its workspaces are listed
beneath, under a dim column heading row:

| Column | Meaning |
| --- | --- |
| `●` / `○` | **active** (green) — holds work not yet in base · **clean** (blue) — merged, nothing outstanding |
| `branch` | The workspace's branch name (its identity) |
| `state` | The word `active` or `clean` |
| `behind\|ahead` | Commit gap to base: `↓` commits on base this branch lacks, `↑` commits it has on top |
| `diff` | `+added -removed` lines vs base, with a trailing `*` when the working tree is dirty |
| `base` | The base branch this workspace merges into |

A leading **`▣`** (cyan) marks a workspace whose **tmux window is open right
now** — derived live on every refresh. The selected row is highlighted and
prefixed with `❯`.

### Glyph legend

```ansi
● active   ○ clean   ▣ tmux open   ↓behind|↑ahead vs base   +added -removed   * uncommitted
```

## Keybindings

### Ledger (default)

| Key | Action |
| --- | --- |
| <kbd>↑</kbd> / <kbd>k</kbd>, <kbd>↓</kbd> / <kbd>j</kbd> | Move the cursor |
| <kbd>g</kbd> / <kbd>Home</kbd>, <kbd>G</kbd> / <kbd>End</kbd> | Jump to top / bottom |
| <kbd>Enter</kbd> / <kbd>d</kbd> | View the selected workspace's **diff** |
| <kbd>a</kbd> | **Add** a workspace in the selected project (opens a branch-name prompt) |
| <kbd>o</kbd> | **Open** the workspace in your editor |
| <kbd>t</kbd> | Jump to the workspace's **tmux window** (tmux only) |
| <kbd>c</kbd> | **Copy** the workspace path to the clipboard |
| <kbd>m</kbd> | **Merge** the workspace (asks to confirm) |
| <kbd>x</kbd> | **Remove** the workspace (asks to confirm) |
| <kbd>r</kbd> | Refresh status now |
| <kbd>q</kbd> / <kbd>Ctrl</kbd>+<kbd>C</kbd> | Quit |

The help line at the bottom adapts to your environment — inside tmux it shows
`o edit · t term`; without tmux, a single `o open`.

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
