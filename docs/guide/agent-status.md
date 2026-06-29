# Agent Status

WorkFlow can show, live, what a coding agent is doing in **every** tmux tab — a
**working**, **waiting**, **ready**, or **idle** icon prefixed to the window name
(with the tab recoloured to match), and the same icon in the
[dashboard](/guide/dashboard) and [sidebar](/guide/tmux#sidebar). At a glance you
can see which tab is busy, which is blocked on you, which finished and wants your
reply, and which is idle — without switching to it.

It decorates **whatever tab the agent is in** — a worktree window WorkFlow
opened, the project's base checkout, or a tab you started by hand in some
unrelated directory. A tab WorkFlow doesn't own is left **exactly as it found
it** once the agent's session ends: its original name and `automatic-rename` are
restored. WorkFlow stays a polite guest.

The status is driven by your agent's lifecycle events, so it tracks the agent
rather than git. It's built for [Claude Code](https://claude.com/claude-code)
out of the box but works with **any** agent that can run a command on its
lifecycle events.

## Turn it on

WorkFlow reports status from a small command, `wf set-status`, that it asks your
agent to call as it works. For Claude Code, install the hooks once:

```sh
wf hooks install
```

This adds five lifecycle hooks to `~/.claude/settings.json` (creating it if
needed). The install is **idempotent** and only ever touches its own entries, so
re-running it is safe and your other hooks and settings are left untouched.

::: tip Upgrading is automatic
If you installed the hooks before the `ready`/`SessionEnd` states existed, you
don't have to do anything: **opening the dashboard brings already-installed hooks
up to date in the background** and shows a one-line notice when it does. The
dashboard opens instantly either way — the check never holds it up. It only ever
updates hooks you've already opted into — it never installs from scratch or undoes
a `wf hooks uninstall`. You can still run `wf hooks install` by hand any time.
:::

```sh
wf hooks print        # print the hook JSON without installing (for manual setup)
wf hooks uninstall    # remove WorkFlow's hooks again
```

Because `wf set-status` figures out the workspace from its working directory, a
single global install covers **every** current and future workspace — there's
nothing to wire up per repo.

## The states

| State | When | Icon |
| --- | --- | --- |
| **working** | The agent is mid-turn (a prompt was sent, or a tool just ran) | 🤖 robot |
| **waiting** | The agent is blocked on you mid-turn (a permission prompt or dialog) | ⏳ hourglass |
| **ready** | The agent finished its turn and is parked for your reply ("your turn") | 🔔 bell |
| **idle** | No agent session here at all | 🌿 branch |

By default the icons are **Nerd Font** glyphs (a robot, an hourglass, a bell, and
a branch); the emoji above stand in for them here. Switch presets or override the
glyphs in [config](#customising-the-icons) if your font lacks them.

**ready** is distinct from **idle** on purpose: a finished-but-parked agent
("your turn") should not look identical to a tab with no agent at all.

Only **working** ages out: it restamps on every tool call, so a stale one means
the agent died mid-tool (see `ttl` below) — it's then shown as idle so a crashed
or detached agent doesn't look busy forever. **waiting** and **ready** never
restamp by nature, so they **persist** until the next state change or the session
ends — a "needs you" tab stays lit even while you're away for half an hour.

## Where it shows

- **tmux tab** — the icon is prefixed to the current window's name and, by
  default, the whole tab is recoloured for the state (yellow working, red
  waiting, green ready). A tab WorkFlow opened keeps its icon; a borrowed tab is
  reverted to exactly its prior state when the session ends.
- **[Dashboard](/guide/dashboard)** — an agent-status cell on each workspace row
  **and on the base/root checkout row**, with the working/waiting/ready glyphs
  called out in the legend. Idle rows leave the cell blank. (Unrelated tabs are
  tmux-only — the dashboard is WorkFlow-scoped.)
- **[Sidebar](/guide/tmux#sidebar)** — the status of each open window, live.

All three update on their own as the agent works — no manual refresh.

::: tip A tab was left decorated?
If an agent's session is killed outright (so its `SessionEnd` hook never fires)
and the tab is never reused, run `wf status reset` to revert any borrowed tabs
WorkFlow decorated but left behind. There's no background janitor by design — the
next agent in a stale tab also fixes it automatically.
:::

## How it works

Your agent's hooks call `wf set-status working|waiting|ready|done` at the right
moments. For Claude Code the mapping `wf hooks install` writes is:

| Claude Code event | Status |
| --- | --- |
| `UserPromptSubmit`, `PostToolUse` | working |
| `Notification` (`permission_prompt` / `elicitation_dialog`) | waiting |
| `Stop` (end of every turn) | ready ("your turn") |
| `SessionEnd` (teardown) | done |

A turn runs `working` → … → `ready`; you reply and it's `working` again;
`waiting` interleaves whenever the agent blocks on a prompt. `done` (on session
teardown) reverts a borrowed tab and drops an owned tab back to the idle glyph.

`wf set-status` reads the agent's working directory from the hook's stdin (the
JSON payload Claude Code pipes in, falling back to its own working directory),
records the state to a small status file, and — inside tmux — decorates the
agent's **current window**. For a registered worktree or project base it writes a
status file the dashboard reads; for an unrelated tab it decorates tmux only.

To drive the icons from a **different** agent, point its lifecycle hooks at the
same command — anything that can run `wf set-status <state>` on its events works;
`wf hooks print` shows the shape.

## Customising the icons {#customising-the-icons}

Add a `status:` block to your [global config](/reference/configuration#global-config)
to change the glyphs, colours, tab-colouring mode, or staleness window. Every
field is optional.

```yaml
# ~/.config/workFlow/config.yaml
status:
  preset: nerdfont        # glyph set: nerdfont (default), emoji, or ascii
  color_mode: tab         # how tmux colours the tab: tab (default), glyph, or none
  scope: all              # which tabs to decorate: all (default) or wf
  ttl: 30m                # how long a working status stays "live" before idle (default 30m)
  glyphs:                 # override individual state glyphs
    working: "🤖"
    waiting: "⏳"
    ready: "🔔"
    idle: "🌿"
  colors:                 # override individual state colours (ANSI-256 numbers)
    working: "11"         # bright yellow
    waiting: "9"          # bright red
    ready: "10"           # bright green
```

| Field | Default | What it does |
| --- | --- | --- |
| `preset` | `nerdfont` | Built-in glyph set: `nerdfont`, `emoji` (🤖 ⏳ 🔔 🌿), or `ascii` (`*` `?` `!` `-`) |
| `color_mode` | `tab` | tmux tab colouring: `tab` (whole tab), `glyph` (icon only), or `none` |
| `scope` | `all` | Which tabs to decorate: `all` (any current window, anywhere) or `wf` (only registered worktrees and project bases — leave unrelated tabs untouched) |
| `ttl` | `30m` | How long a **working** status stays live before it reads as idle (any Go duration, e.g. `90s`, `10m`; `≤0` disables). `waiting`/`ready` ignore it and persist. |
| `glyphs` | preset | Per-state glyph overrides (`working` / `waiting` / `ready` / `idle`) |
| `colors` | yellow / red / green / none | Per-state colour overrides as ANSI-256 numbers |

::: tip Nerdfont glyphs need a patched font
The default preset uses [Nerd Font](https://www.nerdfonts.com/) glyphs. If your
terminal font isn't patched, switch to `preset: emoji` or `preset: ascii` so the
icons render.
:::
