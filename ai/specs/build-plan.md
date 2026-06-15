# WorkFlow — Build Plan

A local-first, CLI-first developer tool that orchestrates **git worktrees** and (optionally) **tmux**, with a **TUI dashboard**, written in **Go**. It lets you spin up an isolated workspace per piece of work, see all of them at a glance with live git status, jump into a terminal in any of them, review the diff, and commit/merge — and it grows toward running your own `stackllm`-based agents in those workspaces.

> Companion file: [`progress.md`](./progress.md) — the living build tracker. Update it as work lands.
> This plan is **self-contained** and written to be built from an **empty repository**.
> The installed command is **`wf`** (WorkFlow → **w**ork **f**low): the flow of workspaces, branches, and (later) agents you launch, watch, and steer from one cockpit.

---

## What we're building

Think Paseo's value — projects mapped to git repos, one-click **worktree workspaces**, a live diff, AI commit messages, commit→PR→merge without leaving the app — but delivered as a **tmux-native CLI + TUI** instead of an Electron app, so it composes with a developer's real terminal and is trivial to automate later.

The headline daily-driver loop: **see every workspace across your projects in one dashboard → open a terminal in any of them (run Claude Code / Codex / later your own agent) → review the diff → commit and merge → clean up.**

This design is the synthesis of a long design interview. The short history matters because it explains the shape: we moved **browser app → TUI cockpit → tmux-native CLI**, and every step *removed a layer that fought the developer's existing workflow*. The result is the most "unix" version of the tool: small, composable, and a guest in the environment you already use.

---

## Core principles (read these first — they decide everything downstream)

1. **Guest, not owner.** The tool **never wraps, captures, or bootstraps its own tmux session.** It runs as a guest inside *your* tmux (or your plain terminal) and creates *real* tmux windows you navigate with *your own* keybindings. Splitting, resizing, and moving between panes/windows must keep working exactly as they do today. *(This is the single biggest lesson from the prototype, which wrongly owned tmux and broke native navigation.)*
2. **Structured data → we draw it. Live programs → real terminals.** Lists, diffs, status, the dashboard: the tool renders these (Go TUI). Shells and agents: they run in **real tmux windows / your real terminal**, never reimplemented inside the app (no embedded terminal, no xterm.js, no libghostty).
3. **CLI-first.** The engine is a CLI where *everything* is a command. The TUI dashboard and sidebar are **optional surfaces over the same engine**. Future automated agents drive the exact same CLI.
4. **tmux is optional, and detected.** The core (worktree management + dashboard + git/PR/merge) works in any terminal. tmux is an auto-detected *power-up* that adds window/jump commands. Teammates who don't use tmux still get a worktree manager with a dashboard.
5. **Cockpit-first; automation later over the same core.** Build the attended daily-driver tool first. Autonomous agents, a job API, and external integrations (e.g. ArkPM) come later as a **headless layer over the same engine** — not in the critical path now.
6. **Persist durable facts; derive the rest.** Store only what can't be recomputed (the project/worktree registry). Compute git stats, "is a window open?", and agent status live on every refresh.
7. **Least complexity; reuse the ecosystem.** Shell out to `git`, `tmux`, and `gh` rather than reimplement them. Prefer the smallest thing that delivers the daily loop.

---

## Decisions (resolved during the design interview)

| Area | Decision                                                                                                                                                                                                    |
|---|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Form factor** | A **Go CLI + Bubble Tea TUI**. Not Electron, not a browser app, not an embedded terminal.                                                                                                                   |
| **Terminal strategy** | Use **real terminals** — tmux windows when in tmux, the user's terminal otherwise. Never embed/reimplement a terminal.                                                                                      |
| **tmux relationship** | **Guest, never owner.** Create real windows in the user's *current* session; never bootstrap a captive session.                                                                                             |
| **tmux requirement** | **Optional, auto-detected** via `$TMUX`. Full window integration when present; universal actions (copy-path, `path`+shell-fn, open-in-editor) when absent.                                                  |
| **Scoping** | **B: one flat tmux session, one window per workspace, dashboard as window 0.** (Most users juggle ≤2 projects; flat window management is enough. Session-per-project rejected as overkill.)                 |
| **Surfaces** | **CLI** (engine) · **tmux-native navigation** · **Dashboard** (cross-project ledger / home) · **Sidebar** (live "now": open terminals + agent status). The last two are optional.                           |
| **Object model** | **Project → Workspace (worktree) → Terminal.** Workspace is first-class and agent-optional. (Later: Agent/Session attached to a workspace.)                                                                 |
| **Persistence** | Persist **projects + worktrees** only (small JSON store, atomic writes; SQLite later if needed). Derive git stats / window-open / agent status live.                                                        |
| **"Active vs done"** | A worktree is **active** if: uncommitted changes, *or* ahead of base / open PR, *or* a live agent/terminal. **Done/cleanable** when merged and clean. Drives the dashboard's key column and cleanup nudges. |
| **Agent status** | Provided later by **hooks** (Claude Code Stop/Notification etc.) that write a per-workspace **status file**; sidebar/dashboard read it. **No daemon** needed locally.                                       |
| **Worktree lifecycle** | `add` / `open` / `close` / `merge` / `rm` / `list` / `path` — the workmux-proven shape. `merge` = merge branch → remove worktree → close window → delete branch. `resurrect` rebinds windows after a crash. |
| **VCS** | **GitHub first** via the `gh` CLI, behind a small host interface (GitLab/Gitea later). List remote branches + PRs; create a workspace from a branch or PR.                                                  |
| **Isolation (Docker)** | **Stage 2.** Per-workspace container (npm + commands run inside) for supply-chain safety, behind an Executor interface. Not in v1.                                                                          |
| **AI agents** | **Stage 3.** Run `stackllm`-based agents and claude code and codex in workspaces; status via hooks; `send`/`capture`/`wait`; verification hooks. Not in v1.                                                 |
| **Daemon** | **Deferred.** Not needed for the cockpit. Multi-host = `ssh host` + the same CLI + remote tmux. A daemon returns only for the Stage-3+ automation API.                                                      |
| **Language/stack** | Go; Bubble Tea + Lipgloss + Bubbles (TUI); shell out to git/tmux/gh; JSON registry; a clipboard lib.                                                                                                        |

---

## Object model

```
Project          a git repo at a path on this host
  └─ Workspace   a git worktree on a branch (base branch, optional PR, status)
       └─ Terminal   a tracked shell/agent in that workspace
                     (a real tmux window when in tmux)
   (later) Agent/Session: an agent engagement attached to a Workspace
```

- **Project** — registered by path (Stage 2: clone from a GitHub URL). Optional per-repo `.workFlow.yaml` (worktree base dir, setup commands, files to copy/symlink into new worktrees, default base branch).
- **Workspace** — a worktree on its own branch. First-class and **agent-optional** (usable for hand-work). Carries: path, branch, base branch, created-at, optional PR ref, status slot.
- **Terminal** — a tracked shell. In tmux it's a real **window** (tracked by tmux window/pane id + name). Outside tmux it's whatever the universal launcher opened.

---

## Surfaces (one engine, opt-in layers)

1. **CLI — the engine and the power.** Every capability is a subcommand. Scriptable; future agents call it. This is the foundation everything else sits on.
2. **tmux-native navigation.** When in tmux, the tool adds a window per workspace; **tmux's own window list + your keys are the navigation.** You already have a cockpit — it's tmux.
3. **Dashboard (TUI) — the ledger / home.** Run `wf` (or `wf dashboard`) → a cross-project overview: projects → worktrees, live git stats (the +/− icons, ahead/behind, dirty), **active/done** status, and (Stage 3) agent status. It's where you land. Actions: open, review diff, commit, merge, remove, create-from-branch/PR. It is a **peer tmux window** when in tmux, or a standalone TUI otherwise — never a wrapper that captures navigation. Its left-hand list *is* "click into a workspace" — implemented as `select-window` to a real window you can also reach by hand.
4. **Sidebar (optional, thin) — the live "now".** Only the **currently-open** terminal windows and their **hook-driven** progress (working / waiting-for-input / done). Always-on, glanceable; the most useful thing when running several features at once. (Stage 3 fills the status; the slot exists from the start.)

**Ledger vs. live-now:** the dashboard is the full ledger of everything tracked (even with no terminal open); the sidebar is only what's open right now.

---

## tmux integration (optional, detected)

- **Detect `$TMUX`.** Present → tmux launcher backend. Absent → universal launcher.
- **Scoping = B:** one flat tmux session; **one window per workspace**; the dashboard is window 0. Jump with native `prefix+<n>`; the tool never remaps your keys.
- **tmux launcher backend:** `add`/`open` create or jump to a real window at the worktree path; `close` kills the window (keeps worktree+branch); `merge`/`rm` also close the window; **`resurrect`** re-creates/rebinds windows for tracked worktrees after a tmux or machine restart (match by window name).
- **Universal launcher (no tmux):** `path` (print the worktree path), **copy-path-to-clipboard**, **open-in-editor** (`$EDITOR`/`code <path>`), and a shell function the user installs once for "jump": `wfd <ws>` → `cd "$(wf path <ws>)"` (the zoxide pattern). Spawning a new OS terminal is OS-specific and deferred.
- **Never** bootstrap a captive session or take over the user's layout. The prototype's mistake — owning tmux — is explicitly out of bounds.

---

## Data model & persistence

- **Persisted (durable truth):** projects + worktrees. Fields per worktree: `project`, `path`, `branch`, `base`, `created_at`, `pr_ref?`, `status_slot?`. Small **JSON store** under `$XDG_CONFIG_HOME/workFlow/` (atomic write: temp file + rename; tolerate concurrent CLI invocations). SQLite is a later option if querying grows.
- **Derived live (each dashboard refresh / CLI call):**
  - **git stats** — dirty?, ahead/behind base, `+/-` from `git diff --shortstat` / `--numstat`, current branch.
  - **window-open?** — query tmux for a window bound to this worktree.
  - **agent status** — read the per-workspace **status file** written by hooks (Stage 3).
- **"Active vs. done":** active if dirty *or* ahead-of-base/open-PR *or* a live agent/terminal; done/cleanable when merged + clean. The dashboard highlights active worktrees and nudges `merge`/`rm` on done ones.
- **Resurrect:** the registry is the source of truth; on launch, reconcile it against live tmux windows and re-bind/offer to reopen. Survives tmux crashes and reboots.
- **Config files:** global `~/.config/workFlow/config.yaml` (editor, clipboard cmd, defaults); optional per-repo `.workFlow.yaml` (worktree base dir, setup commands, file copy/symlink ops, default base branch).
- **Worktree placement:** configurable; default a **sibling** dir `…/<repo>_worktrees/<branch-slug>` (never nested inside the main working tree). Run the project's setup commands on creation (install deps), and copy/symlink configured files (e.g. `.env`).

---

## Command surface (CLI)

Mirror the proven workmux shape; everything the dashboard does is also a command.

```
Worktree lifecycle
  add        create branch + worktree (+ setup) (+ tmux window if present)
  open       open / jump to a workspace (tmux window, or universal action)
  close      close the tmux window; keep worktree + branch
  merge      merge branch → remove worktree → close window → delete branch
  rm         remove worktree + window + branch without merging
  list (ls)  list workspaces with status
  path       print a workspace's filesystem path (for shell `cd` integration)

Review & VCS
  diff       show a workspace's diff (CLI)
  branches   list remote branches (gh)
  prs        list pull requests (gh)
  add --branch <b> / --pr <n>   create a workspace from a branch or PR

UI
  dashboard  open the TUI ledger (default when run with no command)
  sidebar    toggle the live status sidebar (tmux)

Setup
  init       write an example .workFlow.yaml
  config     manage global config
  completions  shell completions

(Stage 3) Agents
  status / send / capture / wait / run
```

---

## Architecture (package layout for an empty repo)

```
cmd/wf/                  main; CLI dispatch (cobra) — subcommands above
internal/config/         global + per-repo config; XDG paths
internal/registry/       persistent JSON store of projects + worktrees (atomic writes)
internal/git/            git ops: worktree add/list/remove, diff/shortstat, branch, ahead/behind
internal/vcs/            Host interface; internal/vcs/github (gh CLI): branches, PRs, open/merge PR
internal/launcher/       "open a workspace" adapter:
                           tmux backend (new/select/kill window, resurrect) | universal (path/clipboard/editor)
internal/tmux/           thin tmux client (detect $TMUX, window ops, queries) — used by launcher
internal/workspace/      lifecycle orchestration: ties git + registry + launcher (add/merge/rm/close)
internal/status/         per-workspace agent status files (stub in v1; used in Stage 3)
internal/dashboard/      Bubble Tea TUI: ledger tree, diff viewer, actions → engine
internal/sidebar/        live "now" strip (optional)
internal/executor/       (Stage 2) Executor interface + Docker backend
internal/agent/          (Stage 3) stackllm agent runner + hooks
```

**Stack:** Go; `cobra` (CLI) ; Bubble Tea + Lipgloss + Bubbles (TUI); shell out to `git`/`tmux`/`gh`; a clipboard library; JSON registry. No Electron, no web frontend, no embedded terminal.

---

## Milestones

Each milestone is independently useful and verifiable. Stage 1 is **M1–M4** (the cockpit).

### M1 — Engine: worktrees + registry + git (CLI only)
The scriptable core, usable in any terminal, no tmux/AI.
- `config` + `registry` (projects + worktrees, atomic JSON), XDG paths.
- Project register (path-based); per-repo `.workFlow.yaml` (worktree base, setup cmds, file copy/symlink).
- Workspace: `add` (branch + worktree + setup + file ops), `list`, `path`, `rm`, local `merge` (merge to base → remove worktree → delete branch).
- `git` derivations: dirty, ahead/behind, `+/-` shortstat, branch.
- Universal launcher: `path`, copy-to-clipboard, open-in-editor.
- **Deliverable:** a composable worktree manager.
- **Verify:** register a project; `add` two workspaces (confirm worktrees + branches + setup ran); `list` shows them with stats; edit a file → stats update; `merge` one → worktree/branch gone, changes on base; `rm` the other → clean.

### M2 — Dashboard TUI (the ledger) + review
- Bubble Tea dashboard: projects → worktrees tree; live git stats; **active/done** flag; manual + auto refresh.
- Diff viewer: select a workspace → scrollable, colorized diff (review code quickly).
- Actions wired to the M1 engine: `add`, `open` (universal), copy-path, open-in-editor, `merge`, `rm`.
- **Deliverable:** run `wf` → see everything, review diffs, merge — **works without tmux**.
- **Verify:** launch with several workspaces across 2 projects; confirm stats + active/done; scroll a diff; merge from the dashboard; confirm the ledger updates and the registry persists across restarts.

### M3 — tmux integration (optional adapter) + sidebar
- Detect `$TMUX`; tmux launcher backend: `add`/`open` → real window (flat session, window-per-workspace, dashboard = window 0); `close`; **`resurrect`**.
- Dashboard "open" becomes a tmux jump (`select-window`) when in tmux; native split/resize/navigation untouched.
- Sidebar: live strip of open terminals (+ empty agent-status slot).
- **Guest principle enforced**: no captive session; operate on the user's current session.
- **Deliverable:** the full cockpit for tmux users, with native navigation preserved.
- **Verify:** inside tmux, `add`/`open` create real windows you reach with `prefix+<n>`; split/resize a window manually (still works); kill tmux and `resurrect` to rebind; confirm the no-tmux path still works unchanged.

### M4 — GitHub: branches, PRs, merge
- `vcs/github` via `gh` behind the Host interface.
- `branches` / `prs`; `add --branch` / `add --pr` to create a workspace from a branch or PR.
- PR state in the dashboard; open PR; merge via PR or locally. (AI commit/PR text deferred to Stage 3.)
- **Deliverable:** launch workspaces from branches/PRs; see PR status; commit + merge from CLI and dashboard.
- **Verify:** list PRs; `add --pr <n>` creates a workspace on that branch; the dashboard shows PR status; merge cleans up worktree + window + branch.

### Stage 2 — Docker isolation
- `executor` interface + Docker backend; per-workspace container; npm/setup/commands run **inside** the container (supply-chain safety). Auto-detect an existing `Dockerfile`/`.devcontainer`.
- **Verify:** a workspace's commands run in a container; nothing untrusted touches the host.

### Stage 3 — AI agents
- Run `stackllm`-based agents in workspaces; **agent status via hooks** (status files) shown in sidebar/dashboard (active / waiting-for-input / complete); `send`/`capture`/`wait`/`run`; verification hooks (Stop/SubagentStop feed-back-until-clean).
- **Verify:** launch an agent in a workspace; watch its status flow through the sidebar; an ambiguous task surfaces a waiting-for-input state.

---

## Future / explicitly not now
- **Daemon + programmatic job API** (the automation layer) and an **ArkPM integrator** (separate, deterministic, no-AI) that feeds work in.
- **Outbound notifiers** (webhook/email/Slack/web-push) on agent status changes.
- **Multi-host beyond SSH+tmux** (a real remote protocol), if/when the daemon lands.
- **Auto-clone a project from a GitHub URL.**
- Additional VCS adapters (GitLab/Gitea); spawn-a-new-OS-terminal launcher; native Anthropic provider in `stackllm`.

---

## Open questions to confirm at implementation
- Registry format: start JSON (recommended) vs SQLite from day one.
- Exact worktree base-dir convention and branch→slug rule.
- `.workFlow.yaml` schema (setup commands, file copy/symlink, base branch, editor).
- Sidebar transport for agent status (status-file polling vs fsnotify) — decide in Stage 3.
- Whether the dashboard should also be available as a **tmux popup** (summonable from anywhere) in addition to window 0.
