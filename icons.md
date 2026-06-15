# Session Status Icons (workmux Claude Code integration)

How workmux drives the per-tab status indicators (the "robot head" icons that
appear while Claude is working) by hooking into Claude Code's lifecycle events.

This is captured from a working install so the `workflow` project can implement
the same mechanism itself.

## The mechanism

Claude Code fires **hooks** at defined points in a session's lifecycle. workmux
registers a `command`-type hook on four of those events. Each hook simply shells
out to `workmux set-window-status <state>`, and workmux updates the icon on the
current tmux/workmux window/tab accordingly.

The state flows like this over a normal turn:

```
UserPromptSubmit ──▶ working
   PostToolUse   ──▶ working   (fires after every tool call)
   Notification  ──▶ waiting   (only on permission_prompt / elicitation_dialog)
      Stop       ──▶ done       (turn finished)
```

| Hook event         | Matcher                                | Command                                 | Meaning                          |
|--------------------|----------------------------------------|-----------------------------------------|----------------------------------|
| `UserPromptSubmit` | (none — all)                           | `workmux set-window-status working`     | User sent a prompt → working     |
| `PostToolUse`      | (none — all tools)                     | `workmux set-window-status working`     | A tool just ran → still working  |
| `Notification`     | `permission_prompt\|elicitation_dialog`| `workmux set-window-status waiting`     | Needs user input → waiting       |
| `Stop`             | (none — all)                           | `workmux set-window-status done`        | Turn finished → done             |

Notes:
- `PostToolUse` keeps the window in `working` after each tool result, so a long
  multi-tool turn stays "working" throughout rather than flickering.
- The `Notification` hook is matched only against `permission_prompt` and
  `elicitation_dialog` notifications, so the "waiting" state specifically means
  "Claude is blocked on you," not just any notification.
- `Stop` fires when the turn ends, flipping the tab to `done`.
- There is no `SubagentStop` / `PreToolUse` wiring — only the four events above.

## Exact configuration

This is the verbatim `"hooks"` block from `~/.claude/settings.json` that produces
the behaviour. Drop this into a Claude Code `settings.json` (user-level
`~/.claude/settings.json`, or project-level `.claude/settings.json`) to replicate
it.

```json
{
  "hooks": {
    "Notification": [
      {
        "matcher": "permission_prompt|elicitation_dialog",
        "hooks": [
          {
            "type": "command",
            "command": "workmux set-window-status waiting"
          }
        ]
      }
    ],
    "PostToolUse": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "workmux set-window-status working"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "workmux set-window-status done"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "workmux set-window-status working"
          }
        ]
      }
    ]
  }
}
```

## Implementing this in `workflow`

To reproduce the icon behaviour without depending on the `workmux` binary,
replace each `command` with your own status-setting script. The contract is
minimal:

- A small CLI (here `workmux set-window-status <state>`) that maps a state string
  to a visual change on the current terminal tab/window.
- Three states are used: `working`, `waiting`, `done`. (You can add more, e.g.
  an `idle`/reset state, but these three are what the hooks emit.)
- The hook command runs in the session's environment, so it can read context
  from env vars / stdin. Claude Code passes hook event JSON on **stdin**, which
  your script can parse if it needs session id, tool name, cwd, etc.

Suggested replacement command shape:

```json
{ "type": "command", "command": "workflow set-status working" }
```

Then `workflow set-status` does whatever your project wants — set a tmux window
name/icon, write to a status file, emit an OSC escape, etc.

### State → icon mapping (reference)

| State     | Emitted by hook(s)               | Typical icon idea |
|-----------|----------------------------------|-------------------|
| `working` | `UserPromptSubmit`, `PostToolUse`| 🤖 (robot, busy)  |
| `waiting` | `Notification`                   | ⏳ / 🔴 (blocked)  |
| `done`    | `Stop`                           | ✅ / 💤 (finished) |

## Reference

- Source of truth captured from: `~/.claude/settings.json` (user-level settings).
- Claude Code hooks fire as `command` type and receive event JSON on stdin.
- Events used: `UserPromptSubmit`, `PostToolUse`, `Notification`, `Stop`.
