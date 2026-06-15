# WorkFlow — Build Progress

> **Living document.** Update as the build progresses — tick items, move milestone status, append dated notes. Single source of truth for "where are we?". See [`build-plan.md`](./build-plan.md) for the full spec and rationale.

## How to use this file
- Update each task's status: `[ ]` todo → `[~]` in progress → `[x]` done. Use `[!]` for blocked.
- When a milestone's tasks are all `[x]`, set its **Status** to ✅ Done with the date.
- Append a dated **Changelog** entry whenever something meaningful lands or a decision changes.
- If a decision in `build-plan.md` changes, note it under **Decision changes** and update the build plan too.

## Status legend
`[ ]` todo · `[~]` in progress · `[x]` done · `[!]` blocked

## Milestone status at a glance
| Milestone | Status | Started | Completed |
|---|---|---|---|
| M1 — Engine: worktrees + registry + git (CLI) | ✅ Done | 2026-06-15 | 2026-06-15 |
| M2 — Dashboard TUI + review | ✅ Done | 2026-06-15 | 2026-06-15 |
| M3 — tmux integration + sidebar | ✅ Done | 2026-06-15 | 2026-06-15 |
| M4 — GitHub: branches, PRs, merge | ⬜ Not started | — | — |
| Stage 2 — Docker isolation | ⬜ Not started | — | — |
| Stage 3 — AI agents | ⬜ Not started | — | — |

---

## M1 — Engine: worktrees + registry + git (CLI only)
**Goal:** a composable, scriptable worktree manager usable in any terminal — no tmux, no AI.

- [x] `cmd/wf` skeleton + CLI dispatch (cobra)
- [x] `internal/config` — global config + per-repo `.workFlow.yaml`; XDG paths
- [x] `internal/registry` — JSON store of projects + worktrees (atomic writes + flock)
- [x] Project register (path-based) — `wf project add/ls/rm`
- [x] `internal/git` — worktree add/list/remove; diff/numstat; branch; ahead/behind
- [x] `internal/workspace` — `add` (branch + worktree + setup + file copy/symlink), `list`, `path`, `rm`, local `merge`
- [x] `internal/launcher` (universal) — `path`, copy-to-clipboard, open-in-editor
- [x] `completions` — generate + **install** shell completions (bash/zsh/fish/powershell)
- [x] **M1 verification** (see build-plan.md → M1)

## M2 — Dashboard TUI + review
**Goal:** run the app → cross-project ledger; review diffs; merge. Works without tmux.

- [x] `internal/dashboard` — Bubble Tea ledger: projects → worktrees tree, live git stats, active/done flag, refresh (manual `r` + auto every 4s while idle; selection preserved across refresh)
- [x] Diff viewer (scrollable, colorized; cumulative diff vs base + untracked-file list)
- [x] Dashboard actions wired to the engine: add, open-in-editor, copy-path, merge, rm (open universal; tmux jump arrives in M3)
- [x] **M2 verification** (see build-plan.md → M2)

## M3 — tmux integration (optional adapter) + sidebar
**Goal:** the full cockpit for tmux users, native navigation preserved. Guest, never owner.

- [x] `internal/tmux` — detect `$TMUX`; window ops (new/select/kill); queries (list, find-by-workspace, open-set)
- [x] `internal/launcher` (tmux backend) — `add` → real window (detached), `open` → create-or-jump, `close` → kill; `merge`/`rm` close the window
- [x] `resurrect` — recreate windows for tracked worktrees after a tmux/computer restart (matched by the `@wf_workspace` path tag)
- [x] Dashboard `open` → tmux `select-window` when in tmux (`t` key; `▣` marks workspaces with an open window)
- [x] `internal/sidebar` — live strip of open terminals (+ empty agent-status slot)
- [x] **M3 verification** (see build-plan.md → M3)

## M4 — GitHub: branches, PRs, merge
**Goal:** launch workspaces from branches/PRs; PR status; commit + merge from CLI and dashboard.

- [ ] `internal/vcs` Host interface + `internal/vcs/github` (gh CLI)
- [ ] `branches` / `prs` listing
- [ ] `add --branch <b>` / `add --pr <n>`
- [ ] PR state in dashboard; open PR; merge via PR or locally
- [ ] **M4 verification** (see build-plan.md → M4)

---

## Stage 2 — Docker isolation
**Goal:** npm + commands run inside a per-workspace container (supply-chain safety).
- [ ] `internal/executor` interface + Docker backend; per-workspace container; run setup/commands inside
- [ ] Auto-detect existing `Dockerfile` / `.devcontainer`
- [ ] **Stage 2 verification**

## Stage 3 — AI agents
**Goal:** run stackllm agents in workspaces; live status via hooks.
- [ ] `internal/agent` — stackllm agent runner in a workspace
- [ ] `internal/status` — per-workspace status files; hook installation (`setup`)
- [ ] Sidebar/dashboard show agent status (active / waiting-for-input / complete)
- [ ] `send` / `capture` / `wait` / `run`; verification hooks (Stop/SubagentStop feed-back-until-clean)
- [ ] **Stage 3 verification**

---

## Future / not now (tracked, not scheduled)
- [ ] Daemon + programmatic job API (automation layer)
- [ ] ArkPM integrator (separate, deterministic, no-AI)
- [ ] Outbound notifiers (webhook/email/Slack/web-push)
- [ ] Multi-host beyond SSH+tmux; auto-clone from GitHub URL
- [ ] Additional VCS adapters (GitLab/Gitea); spawn-new-OS-terminal launcher; native Anthropic provider

---

## Decision changes
_Record any deviation from `build-plan.md` here, with date and reason._
- **2026-06-14** — **Full redesign.** Replaced the browser/Docker/daemon-first plan with a **CLI-first, tmux-native (optional), Go TUI** design: tool is a *guest* in tmux (never owns it), dashboard ledger + live sidebar, persist-worktrees/derive-the-rest, B-scoping (flat session, window-per-workspace), Docker → Stage 2, AI → Stage 3, daemon deferred to the automation layer. Reason: a design interview + throwaway spike showed the owned-cockpit model fought native tmux; the guest/CLI model is simpler, composable, and team-friendly (works without tmux).

## Changelog
- **2026-06-15** — **M3 landed.** tmux integration as a **guest**, never an owner. New `internal/tmux` thin client (detect `$TMUX` + binary; list/new/select/kill windows in the *current* session) and a `launcher.Tmux` backend. Windows are bound to workspaces by a per-window `@wf_workspace` user option holding the worktree path — derived live, nothing extra persisted — so open/close/resurrect rediscover them without storing ephemeral ids. Wiring: `wf add` gives the new workspace a detached window; `wf open` jumps to it (creating on demand) and outside tmux still opens the editor (`--editor` forces the editor even in tmux); new `wf close` kills the window but keeps the worktree+branch; `merge`/`rm` close the window as they clean up; new `wf resurrect` recreates windows for every tracked workspace after a tmux/computer restart (skips ones already open or missing on disk). Dashboard: `t` jumps to a workspace's window (a plain `select-window`, no suspend — the dashboard keeps running in its own window), a `▣` marks workspaces whose window is open right now (derived each refresh alongside git status), and the footer help adapts in/out of tmux. New `internal/sidebar` + `wf sidebar`: a thin, live (~1.5s) strip of the windows open right now with an empty agent-status slot (Stage 3 fills it) — `enter` jumps, meant to run in a split pane. Tests: pure `tmux` window parsing + a full window-lifecycle integration test on an **isolated** tmux server (`-L`, kill only that socket — per CLAUDE.md), `sidebar` entry building/sorting, dashboard `t`/indicator/footer logic, and CLI guards (tmux-only commands refuse cleanly without `$TMUX`; `open` falls back to the editor). The CLI test harness now clears `$TMUX` so the suite never touches a real server. End-to-end verified against an isolated server: `add`→window, `open` idempotent jump, kill→`resurrect` rebind, `close`, `merge` window cleanup, and the no-tmux fallback — default server confirmed untouched.
- **2026-06-15** — **M2 landed.** Built the Bubble Tea dashboard (`internal/dashboard`) — the default surface when running `wf` on a TTY (`wf` piped still prints the plain list; `wf dashboard`/`dash`/`ui` forces it). Cross-project ledger: projects → worktrees tree with live git stats and an active/done flag, manual (`r`) + auto-refresh (4s while idle on the ledger), selection preserved across refresh. Scrollable, colorized diff viewer (`enter`/`d`) showing the cumulative diff vs base plus an untracked-file list. Actions wired straight to the engine: add (`a`, inline branch prompt), open-in-editor (`o`), copy-path (`c`), merge (`m`) and rm (`x`), both behind a y/n confirm. Engine additions: `Manager.Ledger()` (projects grouped with workspaces, incl. empty projects), `Manager.Diff()`, `git.Diff()`, `launcher.EditorCommand()`. Operations that stream git/setup output (add/merge/rm) and the editor run via `tea.ExecProcess` (suspend → run → resume) so output and terminal editors display cleanly. **`merge` is now non-interactive** — a default `Merge branch 'X' into base` message is supplied, so the dashboard never has to host the merge-message editor (also smooths the CLI). Verified end-to-end in an isolated sandbox: ledger across 2 projects, active/done + stats, diff scroll, merge → ledger updates + branch/worktree cleaned, add → new workspace, persistence across a restart, and the no-tmux list fallback. TUI smoke-tested through a PTY on a **dedicated tmux server** (`-L`), never the default one (see CLAUDE.md). New tests: `dashboard` model logic (row flattening, selection preservation, cursor clamp, diff colorize) and a `workspace` Ledger/Diff integration test.
- **2026-06-15** — **M1 landed.** Built the scriptable engine from the empty repo: `cmd/wf` (cobra), `internal/{config,registry,git,launcher,workspace,cli}`. Commands: `project add/ls/rm`, `add`, `list`/`ls` (+`--json`), `path`, `open`, `copy`, `merge`, `rm`, `init`, `config`, `completions` (generate + install for bash/zsh/fish/powershell, replacing cobra's default `completion` command while keeping the hidden `__complete` runtime intact). Registry is atomic JSON + `flock`; git stats (dirty / ahead-behind / ±lines vs base) derived live; universal launcher (path/clipboard/editor); per-repo `.workFlow.yaml` (base, setup, copy, symlink). Unit tests for registry + slug/active logic; full M1 end-to-end verification passed in an isolated `XDG_CONFIG_HOME` sandbox (register → add ×2 → stats → dirty-merge refusal → clean merge → rm → empty). One design refinement: `merge` now refuses up front when the workspace is dirty, so it is all-or-nothing rather than merging history and then failing to remove a dirty worktree. Note: in M1 a brand-new, clean workspace shows as `done` (no commits, not dirty) — the "live terminal/agent keeps it active" dimension arrives with M3/Stage 3.
- **2026-06-13** — Original build plan (browser/Docker/daemon-first) authored. No code.
- **2026-06-14** — Redesigned after a design interview and a throwaway `spike/` prototype (proved: real tmux > embedded terminal; guest > owner). Build plan + progress rewritten for the new architecture. Prototype `spike/` is disposable — the plan is written to build fresh from an empty repo.
