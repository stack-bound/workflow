# Architecture &amp; Design

WorkFlow is the synthesis of a long design interview that moved from
*browser app → TUI cockpit → tmux-native CLI* — each step removing a layer that
fought the developer's existing workflow. The result is the most "unix" version
of the tool: small, composable, and a guest in the environment you already use.

This page summarizes the design for the curious and for contributors. It's drawn
from the project's build plan.

## Core principles

These decide everything downstream:

1. **Guest, not owner.** WorkFlow never wraps, captures, or bootstraps its own
   tmux session. It runs as a guest inside *your* tmux (or your plain terminal)
   and creates *real* windows you navigate with *your own* keybindings.
2. **Structured data → it draws; live programs → real terminals.** Lists, diffs,
   status, the dashboard — WorkFlow renders these. Shells and agents run in real
   tmux windows or your real terminal, never reimplemented inside the app (no
   embedded terminal, no xterm.js).
3. **CLI-first.** The engine is a CLI where *everything* is a command. The
   dashboard and sidebar are optional surfaces over the same engine; future
   automated agents drive the exact same CLI.
4. **tmux is optional, and detected.** The core works in any terminal. tmux is an
   auto-detected power-up that adds window and jump commands.
5. **Cockpit-first; automation later over the same core.** Build the attended
   daily-driver first; autonomous agents come later as a headless layer over the
   same engine.
6. **Persist durable facts; derive the rest.** Store only the project/worktree
   registry. Compute git stats, "is a window open?", and (later) agent status
   live on every refresh.
7. **Least complexity; reuse the ecosystem.** Shell out to `git`, `tmux`, and
   `gh` rather than reimplement them.

## Object model

```
Project          a git repo at a path on this host
  └─ Workspace   a git worktree on a branch (base branch, optional PR, status)
       └─ Terminal   a tracked shell/agent in that workspace
                     (a real tmux window when in tmux)
   (later) Agent/Session: an agent engagement attached to a Workspace
```

- **Project** — a registered git repo, identified by path.
- **Workspace** — a worktree on its own branch. First-class and *agent-optional*:
  fully usable for hand-work. Carries path, branch, base, created-at, and an
  optional PR ref.
- **Terminal** — a tracked shell. In tmux it's a real window; outside tmux it's
  whatever the universal launcher opened.

See [Core Concepts](/guide/concepts) for the user-facing view of these.

## Surfaces

One engine, opt-in layers:

1. **CLI** — the engine and the power. Every capability is a subcommand;
   scriptable, and the foundation everything else sits on.
2. **tmux-native navigation** — when in tmux, a window per workspace; tmux's own
   window list and your keys *are* the navigation.
3. **Dashboard (TUI)** — the cross-project ledger and home: projects → workspaces
   with live stats, an active/clean flag, a diff viewer, and actions. A peer tmux
   window when in tmux; a standalone TUI otherwise — never a wrapper that captures
   navigation.
4. **Sidebar** — a thin, always-on strip of the terminals open *right now*.

**Ledger vs. live-now:** the dashboard is the full ledger of everything tracked
(even with nothing open); the sidebar is only what's open right now.

## Persistence

Only **projects + worktrees** are persisted — a small JSON registry under
`$XDG_CONFIG_HOME/workFlow/`, written atomically (temp file + rename) so
concurrent `wf` invocations are safe. Everything else is derived live:

- **git stats** — dirty?, ahead/behind base, `+/-` lines, current branch.
- **window-open?** — queried from tmux each refresh.
- **agent status** — read from a per-workspace status file written by hooks
  (a later stage; the slot exists from the start).

## Package layout

WorkFlow is a single Go module, `github.com/stack-bound/workflow`. Logic lives
under `internal/`:

```
cmd/wf/                main; CLI dispatch (cobra)
internal/config/       global + per-repo config; XDG paths
internal/registry/     JSON store of projects + worktrees (atomic writes)
internal/git/          git ops: worktree add/list/remove, diff/shortstat, ahead/behind
internal/launcher/     "open a workspace": tmux backend | universal (path/clipboard/editor)
internal/tmux/         thin tmux client (detect $TMUX, window ops, queries)
internal/workspace/    lifecycle orchestration: ties git + registry + launcher
internal/dashboard/    Bubble Tea TUI: ledger, diff viewer, actions → engine
internal/sidebar/      live "now" strip
```

**Stack:** Go; [cobra](https://github.com/spf13/cobra) for the CLI;
[Bubble Tea](https://github.com/charmbracelet/bubbletea) + Lipgloss + Bubbles for
the TUI; shell-outs to `git`/`tmux`/`gh`; a JSON registry. No Electron, no web
frontend, no embedded terminal.

## Where it's going

The cockpit you use today is Stage 1. GitHub integration, container isolation,
and AI agents are planned on top of the same engine.
