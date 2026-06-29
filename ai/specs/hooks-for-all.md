# WorkFlow — Hooks for All Tabs

> Feature spec for `feature/hooks-for-all`. Companion to [`build-plan.md`](./build-plan.md) (overall design) and [`progress.md`](./progress.md) (tracker). This is the synthesis of a design interview (2026-06-29); build from it, and record landing notes in `progress.md`.

## Why

Today the agent-status hooks (`wf set-status`, driven by Claude Code lifecycle events) only decorate tmux tabs **that `wf` itself opened for a registered worktree**. `set-status` resolves the workspace from cwd via `ResolveByCwd` (`internal/status/resolve.go`) and **bails the moment that returns nil** (`internal/cli/setstatus.go:56-59`). The consequences:

- The **base / project-root** checkout — recently promoted to a first-class row in the dashboard — is not a registered *worktree*, so an agent working there is invisible: no status file, no tab icon, no dashboard cell (the base row hardcodes a blank agent cell, `internal/dashboard/view.go:347`).
- Any tab **unrelated to `wf`** (an agent started by hand in an arbitrary directory) gets nothing.

Goal: **see agents working in every tmux tab**, regardless of whether `wf` owns it — worktree, base, or unrelated — while staying a polite guest in tabs `wf` doesn't own.

A second, related defect surfaced during the interview: the status model has no real "your turn" signal. `Stop` (end of every turn) maps to `done → idle`, so a finished-but-parked agent looks identical to one with no agent at all, and the 5-minute TTL actively *hides* a genuinely-waiting agent (e.g. while you're on a 30-minute call). This spec fixes both.

## Principles (extend the build-plan's guest model)

1. **Decorate where the agent actually is.** The hook fires in the agent's tmux pane, so the **current window** is the agent's window. Target it directly; stop using a registry lookup to *find* a window.
2. **Be a polite guest in tabs we don't own.** A tab `wf` didn't open must be left **exactly as we found it** once the agent goes away — original name and tmux `automatic-rename` restored. No permanent decoration on borrowed tabs.
3. **Err toward over-showing, never under-showing.** A stale "it's waiting for you" is acceptable; a missed one is not. Persist "needs you" states until the session ends or you act.
4. **Persist durable facts, derive the rest** (unchanged): status files remain the dashboard's source; tmux tabs are decorated live and never re-rendered on a loop.

## Decisions (resolved during the interview)

| Area | Decision |
|---|---|
| **Window targeting** | Always decorate the **current** tmux window (the pane the hook fired in). The `@wf_workspace` tag is no longer a targeting mechanism. |
| **Ownership discriminator** | The current window's **`@wf_workspace` tag**: present ⇒ **owned** (worktree/base opened via `wf`); absent ⇒ **borrowed**. |
| **Owned tabs** | Decorate `<glyph> <branch>` (label from the existing window name via strip-and-prepend — no git call). On `done`, keep the **idle glyph** (`wf` owns the window permanently). |
| **Borrowed tabs** | **Adopt-and-revert.** First touch snapshots `@wf_prev_name` + `@wf_prev_autorename`; decorate `<glyph> <existing-name>`. On `done`, **fully revert** (restore name + `automatic-rename`, clear markers). |
| **New state** | `ready` ("your turn") — distinct from `waiting`. Two "needs you" states kept separate so a multi-worktree glance distinguishes *blocked mid-turn* from *finished, your move*. |
| **Hook mapping** | `UserPromptSubmit`/`PostToolUse`→`working`; `Notification`(permission/elicitation)→`waiting`; `Stop`→`ready` (was `done`); **new** `SessionEnd`→`done`. |
| **"Your turn" driver** | `Stop` (documented to fire every turn, deterministically), **not** the `idle_prompt` notification (throttle/focus-gated, less reliable as a state driver). |
| **TTL** | Per-state: only `working` ages out (self-refreshes via `PostToolUse`); `waiting`/`ready` **persist** until the next state change or `SessionEnd`. Default raised **5m → 30m**, configurable, `≤0` disables. |
| **Dashboard base row** | Now shows agent status: write a base status file (keyed by path, **empty branch component**, both sides), include `Main` in `readStatuses`, render `agentCell` instead of the blank. |
| **Scope config** | `status.scope: all` (default — decorate any current window) \| `wf` (only when cwd is in a registered worktree or project base). |
| **Stale-tab cleanup** | Lean on `SessionEnd`; self-heals on reuse (markers prevent re-snapshot). Add a manual `wf status reset` sweep. **No** polling janitor. |
| **cwd source** | Read `cwd` from hook **stdin JSON**, falling back to `os.Getwd()`. |

## State model & hook mapping

| Hook | `set-status` arg | State | Glyph (nerdfont / emoji / ascii) | Colour | Staleness |
|---|---|---|---|---|---|
| `UserPromptSubmit`, `PostToolUse` | `working` | `working` | 🤖 / 🤖 / `*` | yellow `11` | TTL 30m (self-refreshes) |
| `Notification` (permission_prompt \| elicitation_dialog) | `waiting` | `waiting` | `` / ⏳ / `?` | red `9` | **persists** |
| `Stop` | `ready` | `ready` *(new)* | bell / 🔔 / `!` | green `10` | **persists** |
| `SessionEnd` *(new hook)* | `done` | `done`→revert/idle | idle glyph on owned; none on borrowed | — | terminal |

Per-turn cycle: `UserPromptSubmit`(working) → `PostToolUse`…(working) → `Stop`(ready) → [you reply] → `UserPromptSubmit`(working). Session teardown: `SessionEnd`(done). `waiting` interleaves whenever the agent blocks on a permission/elicitation prompt mid-turn.

Exact nerdfont codepoints to be verified against the existing preset style during implementation; the bell / `!` / 🔔 *concept* and green `10` are the agreed defaults. All configurable via the `status` block + overrides.

## Behaviour by ownership

**Owned** (current window has `@wf_workspace`): unchanged shape — `<glyph> <branch>`, whole-tab recolour per `color_mode`. On `done`, keep the idle glyph. The only change vs today: between turns the tab now shows the green `ready` glyph ("agent parked, your turn"), falling back to the plain idle glyph only on `SessionEnd` ("no session here") — previously indistinguishable.

**Borrowed** (no `@wf_workspace`): adopt-and-revert.
1. On first decoration (no `@wf_prev_*` markers yet): snapshot the current window name into `@wf_prev_name` and the window's `automatic-rename` into `@wf_prev_autorename`. (Idempotent: if markers already exist, skip — preserves the *true* original across a re-used stale tab.)
2. Decorate: `<glyph> <existing-name>` (strip any leading resolved glyph first, so repeated `PostToolUse` calls don't stack `🤖 🤖 …`), recolour per `color_mode`.
3. On `done`: restore `@wf_prev_name`, restore `automatic-rename` from `@wf_prev_autorename`, clear the markers. The tab returns to exactly its prior state.

Borrowed windows are **never tagged** with `@wf_workspace`, so `merge`/`rm` cleanup (which closes windows by tag) and the dashboard `▣` open-indicator never touch them — that falls out for free.

## TTL

`Effective()` (`internal/status/status.go:197`) becomes per-state: it downgrades **only `working`** to idle past the TTL; `waiting` and `ready` are returned unchanged (persist). Rationale: `working` restamps on every `PostToolUse`, so a live agent never ages out — the TTL only ever catches a process that **died mid-tool** (or a single tool call longer than the TTL, hence the 30m default vs 5m). `waiting`/`ready` never restamp by nature, so any TTL on them is a false-clear — exactly the 30-minute-call bug.

Note the pre-existing asymmetry this also tidies: the TTL is applied at **read** time, so it self-heals the dashboard (re-reads every 4s) but never the tmux tab (only changed on a hook event). With `waiting`/`ready` now persisting, both surfaces agree.

## Dashboard

- `set-status`, when cwd resolves to a registered **project base** (new `ResolveProjectByCwd`, mirrors `ResolveByCwd` over `Projects`) but not a worktree, writes a status file keyed by the **project root path with an empty branch component** (same key on the read side so they match without a git call).
- `readStatuses` (`internal/dashboard/dashboard.go:259`) additionally reads each project's `Main` checkout and stores it under the project-root path.
- `renderMainRow` (`internal/dashboard/view.go:352`) renders `agentCell(...)` from that status instead of the hardcoded blank.
- `ready` is an active state in `agentLook` (`view.go:277`) and appears in the legend, on worktree rows, and on base rows.
- Unrelated tabs have **no** dashboard presence (the dashboard is `wf`-scoped) — they are tmux-only, by design.

## Scope config

New `status.scope` enum on the existing `StatusConfig` (`internal/config/status.go`):
- `all` *(default)* — decorate the current window for any agent, anywhere.
- `wf` — only when cwd resolves to a registered worktree **or** project base; leave truly-unrelated tabs untouched.

## Cleanup

`SessionEnd` is the primary revert signal and covers the common exits (`/exit`, `/clear`, logout, normal quit). Its reliability on SIGKILL / killed terminal is undocumented, so for the residual "decorated, session dead, window never reused, never closed" case:
- **Self-heal on reuse:** the next agent in a stale tab re-decorates correctly, and the idempotent snapshot keeps the true original.
- **Manual `wf status reset`:** sweep all windows carrying `@wf_prev_name` (and/or our decoration), revert them, clear markers. One-key escape hatch.
- **No polling janitor** — cross-process session-liveness checking is fragile and not worth the machinery for a key-press-fixable orphan.

## Implementation plan (by package)

- **`internal/status`**
  - `State`: add `Ready`. `Normalize`: map `"ready"`→`Ready`; keep `"done"`→ the terminal/idle path (used by `SessionEnd`).
  - `Effective`: per-state — age out `working` only.
  - `resolve.go`: add `ResolveProjectByCwd(store, cwd)` (longest-prefix over `Projects`).
  - Base status keying helper (empty-branch component) shared by write (set-status) and read (dashboard).
- **`internal/config/status.go`**
  - Add `ready` glyphs to all presets + default colour `10`; add `Scope` field (`all`|`wf`, default `all`); bump `defaultTTL` to 30m; thread both through `Resolve`/`ResolvedStatus`.
- **`internal/tmux/tmux.go`**
  - Helpers: get/set a window user option (`@wf_prev_name`, `@wf_prev_autorename`), get/set `automatic-rename`, read current window name. Reuse `CurrentWindowID`.
  - Idempotent glyph-strip helper for `WindowName` rebuilds.
- **`internal/cli/setstatus.go`**
  - Read cwd from stdin JSON (fallback `os.Getwd()`).
  - Resolve: worktree → base → unrelated; gate on `scope`.
  - Write status file for worktree (as today) and base (new key); none for unrelated.
  - Decorate the **current** window; branch on owned vs borrowed per the ownership section; handle `done` revert/idle.
- **`internal/cli/hooks.go`**
  - Install/upsert the **`SessionEnd`** hook (`set-status done`); remap **`Stop`** to `set-status ready`; keep the rest. Idempotent merge as today.
  - New `wf status reset` command (sweep + revert).
- **`internal/dashboard`**
  - `readStatuses` includes `Main`; `renderMainRow` renders `agentCell`; `agentLook`/legend include `ready`.

## Testing (per CLAUDE.md — every change ships with tests)

- **Unit (pure logic):** `Normalize` incl. `ready`; per-state `Effective` (working ages, waiting/ready persist); `ResolveProjectByCwd` (root, nested, none); idempotent glyph-strip (incl. the ascii `!`/`?`/`*`/`-` mis-strip edge); `WindowName`/`TabStyleOps` for the new state; scope gating; base status-key round-trip (write key == read key).
- **Integration (isolated tmux only — `-L wf_test`, never the default server; `unset TMUX` for non-tmux paths):** borrowed-tab adopt-and-revert (snapshot → decorate → `done` restores name + `automatic-rename`); owned-tab unchanged; `hooks install` idempotency incl. `SessionEnd`/`Stop` remap and non-clobber of user hooks; `wf status reset` sweep.
- **Dashboard:** base-row agent cell renders from a base status file; `ready` glyph in legend/rows.
- Run `make test-coverage`; **total must not drop**. Add a changelog fragment via `/clog`. Leave the Bubble Tea `Run` loop / `tea.Cmd` shell-out closures uncovered as established.

## Verification (end-to-end, isolated server)

On `tmux -L wf_test`: (1) borrowed tab — start an agent in a non-`wf` dir, confirm it decorates the current window while working/waiting/ready and **fully reverts** on session end (name + `automatic-rename`); (2) base — agent at the project root lights up both the tab and the dashboard base row; (3) worktree — unchanged, plus the new `ready` green between turns; (4) 30-min persistence — a `waiting`/`ready` tab stays lit well past 30m; (5) `wf status reset` clears an orphan; (6) `status.scope: wf` leaves an unrelated tab untouched. Confirm the **default** server is untouched afterward.

## Risks / notes

- **ascii preset glyph-strip:** stripping a leading `*`/`?`/`!`/`-` from an arbitrary borrowed window name (e.g. `*scratch*`) can mis-strip. Acceptable (ascii is the fallback preset); a decoration marker option could harden it later if needed.
- **`SessionEnd` reason codes** (`clear`/`resume`/…): v1 treats all as "revert"; `/clear` reverts then re-decorates on the next prompt (brief flicker). Smarter per-reason handling is a possible follow-up.
- **Per-call tmux cost:** `set-status` now issues a few extra tmux queries per `PostToolUse` (current window, option get/set). Local-socket calls are cheap; revisit only if it shows up.
- **Bigger follow-up banked, not built:** richer `SessionEnd`-reason handling and any session-liveness reconciliation are explicitly out of scope here.
