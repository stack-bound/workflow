# Agent Status

WorkFlow can show, live, what a coding agent is doing in each workspace — a
**working**, **waiting**, or **idle** icon prefixed to the tmux window name (with
the tab recoloured to match), and the same icon in the [dashboard](/guide/dashboard)
and [sidebar](/guide/tmux#sidebar). At a glance you can see which workspace is
busy, which is blocked on you, and which is idle — without switching to it.

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

This adds four lifecycle hooks to `~/.claude/settings.json` (creating it if
needed). The install is **idempotent** and only ever touches its own entries, so
re-running it is safe and your other hooks and settings are left untouched.

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
| **waiting** | The agent is blocked on you (a permission prompt or dialog) | ⏳ hourglass |
| **idle** | No active turn — the turn finished, or status went stale | 🌿 branch |

By default the icons are **Nerd Font** glyphs (a robot, an hourglass, and a
branch); the emoji above stand in for them here. Switch presets or override the
glyphs in [config](#customising-the-icons) if your font lacks them. A working or
waiting status is treated as **idle** once it goes stale (see `ttl` below), so a
crashed or detached agent doesn't leave a window looking busy forever.

## Where it shows

- **tmux tab** — the icon is prefixed to the workspace window's name and, by
  default, the whole tab is recoloured for the state (yellow working, red
  waiting), reverting on idle.
- **[Dashboard](/guide/dashboard)** — an agent-status column on each workspace
  row, with the working/waiting glyphs called out in the legend. Idle rows leave
  the cell blank.
- **[Sidebar](/guide/tmux#sidebar)** — the status of each open window, live.

All three update on their own as the agent works — no manual refresh.

## How it works

Your agent's hooks call `wf set-status working|waiting|done` at the right
moments. For Claude Code the mapping `wf hooks install` writes is:

| Claude Code event | Status |
| --- | --- |
| `UserPromptSubmit`, `PostToolUse` | working |
| `Notification` (`permission_prompt` / `elicitation_dialog`) | waiting |
| `Stop` | done (idle) |

`wf set-status` infers the calling workspace from the current directory, records
the state to a small per-workspace status file, and — inside tmux — updates that
window's tab. It's a silent no-op outside a registered worktree, so it's safe to
run globally.

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
  ttl: 5m                 # how long working/waiting stays "live" before idle (default 5m)
  glyphs:                 # override individual state glyphs
    working: "🤖"
    waiting: "⏳"
    idle: "🌿"
  colors:                 # override individual state colours (ANSI-256 numbers)
    working: "11"         # bright yellow
    waiting: "9"          # bright red
```

| Field | Default | What it does |
| --- | --- | --- |
| `preset` | `nerdfont` | Built-in glyph set: `nerdfont`, `emoji` (🤖 ⏳ 🌿), or `ascii` (`*` `?` `-`) |
| `color_mode` | `tab` | tmux tab colouring: `tab` (whole tab), `glyph` (icon only), or `none` |
| `ttl` | `5m` | How long a working/waiting status stays live before it reads as idle (any Go duration, e.g. `90s`, `10m`) |
| `glyphs` | preset | Per-state glyph overrides (`working` / `waiting` / `idle`) |
| `colors` | yellow / red / none | Per-state colour overrides as ANSI-256 numbers |

::: tip Nerdfont glyphs need a patched font
The default preset uses [Nerd Font](https://www.nerdfonts.com/) glyphs. If your
terminal font isn't patched, switch to `preset: emoji` or `preset: ascii` so the
icons render.
:::
