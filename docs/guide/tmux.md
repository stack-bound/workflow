# tmux Integration

WorkFlow works in any terminal. When you run it **inside tmux**, it lights up
with window integration — but always as a **guest**: it creates real windows in
*your current session* that you navigate with *your own* keybindings, and never
wraps, captures, or bootstraps a session of its own.

::: tip Guest, not owner
Splitting, resizing, and moving between panes and windows keep working exactly as
they do today. WorkFlow only ever *adds* windows to the session you're already
in. This is the single most important rule of the tmux integration.
:::

## How it's detected

WorkFlow checks for the `$TMUX` environment variable. Present → the tmux launcher
backend is used. Absent → the [universal launcher](#without-tmux) is used. There's
nothing to configure.

## The model

One flat session, **one window per workspace**, with the dashboard as a peer
window. You jump between them with native tmux keys (`prefix` + window number);
WorkFlow never remaps your keys.

| Command | In tmux |
| --- | --- |
| `wf add <branch>` | Creates the workspace **and** a real window at its path |
| `wf open <branch>` | **Jumps** to the workspace's window, creating it on demand |
| `wf close <branch>` | **Kills** the window (keeps the worktree and branch) |
| `wf merge <branch>` | Merges, then closes the window as it cleans up |
| `wf rm <branch>` | Removes the workspace, then closes the window |

In the [dashboard](/guide/dashboard), <kbd>t</kbd> jumps to the selected
workspace's window, and a **`▣`** marks any workspace whose window is open right
now.

## Resurrect

tmux windows are *derived*, not persisted — the [registry](/guide/concepts#the-registry)
is the source of truth. After a tmux server restart or a reboot, recreate the
windows for every tracked workspace in one command:

```sh
wf resurrect          # alias: wf restore
```

It reconciles live tmux against the registry, creating a window for any tracked
worktree that doesn't have one (and skipping those already open or missing from
disk). It reports what it did:

```ansi
  rebound acme-api/feature-login
Resurrect: 1 created, 2 already open
```

## Sidebar

`wf sidebar` is a thin, always-on strip of the workspace windows **open right
now** — meant to run in a split pane for an at-a-glance view while you work
across several features:

```sh
wf sidebar
```

Unlike the dashboard (the full ledger of everything tracked), the sidebar shows
only what's currently open. A status slot is reserved for future agent
integration.

## Without tmux

Outside tmux, the window-only commands fall back gracefully:

- `wf open` opens the worktree in your **editor** instead of jumping to a window.
- `wf close`, `wf resurrect`, and `wf sidebar` report that no tmux session was
  detected (they need `$TMUX`).
- `wf path`, `wf copy`, and the [shell `cd` helper](/guide/shell-integration)
  give you universal ways to reach a workspace.

You still get the full worktree manager and dashboard — tmux just adds the
window navigation on top.

::: info Screenshots pending the TUI redesign
Genuine tmux/sidebar captures will be added once the lipgloss redesign lands.
Regenerate them with the procedure in
[`docs/capture/`](https://github.com/stack-bound/workflow/tree/master/docs/capture)
— capture targets: **`sidebar`**, **`tmux-windows`**.
:::
