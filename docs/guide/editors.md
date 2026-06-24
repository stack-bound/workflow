# Editors

WorkFlow opens a workspace in an editor or IDE **inline** — it discovers the
editors installed on this machine and launches one for you. There's no editor
command to configure up front: `wf edit` probes for what you have, shows a
picker, and remembers your choice per repo.

```sh
wf edit                 # open the current directory in an editor (picker)
wf edit feature-x       # open the feature-x workspace instead
```

GUI apps (VS Code, the JetBrains IDEs, Zed, …) launch **detached** so your
shell returns immediately; terminal editors (Vim, Neovim, Helix, …) run
**attached** so they take over the terminal.

## The picker

With no default set, `wf edit` opens a picker listing every editor detected on
this machine — move with the arrow keys, launch with <kbd>Enter</kbd>:

| Key | Action |
| --- | --- |
| <kbd>↑</kbd> / <kbd>k</kbd>, <kbd>↓</kbd> / <kbd>j</kbd> | Move the cursor |
| <kbd>Enter</kbd> | **Launch** the highlighted editor |
| <kbd>d</kbd> | Pin it as the **default** for this repo |
| <kbd>a</kbd> | Pin it as the default **and** enable [autolaunch](#autolaunch) |
| <kbd>Esc</kbd> / <kbd>q</kbd> | Cancel |

The current default is moved to the top of the list, pre-selected, and marked
with a `★`, so a quick <kbd>Enter</kbd> always launches it.

## The per-project default {#default-ide}

Press <kbd>d</kbd> in the picker to pin the highlighted editor as the **default**
for the current repo. WorkFlow records it as `default_ide` in that repo's
[`.workFlow.yaml`](/guide/configuration#per-repo-workflow-yaml):

```yaml
# .workFlow.yaml
default_ide: goland
```

From then on the default is listed first and pre-selected, so opening it is one
keystroke. Each repo can prefer its own IDE. You can also set it by hand — use
[`wf edit --list`](#listing-detected-editors) to find the id.

When the directory you're opening doesn't belong to a registered project,
WorkFlow falls back to a **global** default instead (`default_ide` in the
[global config](/reference/configuration#global-config)); pressing <kbd>d</kbd>
there pins the global default.

## Autolaunch {#autolaunch}

Pin a default **and skip the picker** with autolaunch. Press <kbd>a</kbd> in the
picker (or set it by hand), and `wf edit` opens that editor straight away:

```yaml
# .workFlow.yaml
default_ide: goland
autolaunch: true
```

Need the picker anyway — say, to open a one-off in a different editor? Force it
with `--pick`:

```sh
wf edit --pick          # always show the picker, even with autolaunch on
```

Pressing <kbd>a</kbd> on an editor that's already the autolaunching default
toggles autolaunch back off.

## Custom editors {#custom-editors}

WorkFlow ships a catalog of well-known editors (VS Code, VSCodium, Cursor, Zed,
Sublime Text, the JetBrains IDEs, Neovim, Vim, Emacs, Helix, Nano, and more). To
launch one it doesn't know about, add it under `ides:` in the
[global config](/reference/configuration#global-config):

```yaml
# ~/.config/workFlow/config.yaml
ides:
  - id: lapce
    name: Lapce
    cmd: lapce            # the target directory is appended to this command
    gui: true             # windowed app → launch detached (omit for a terminal editor)
```

A custom entry whose `id` matches a built-in one **overrides** it. Custom
editors appear in the picker and `wf edit --list` alongside the detected ones.

## Listing detected editors {#listing-detected-editors}

To see what WorkFlow found on this machine — and the ids you'd use for
`default_ide` or a config — pass `--list`:

```sh
wf edit --list
```

```ansi
ID        NAME           KIND
code      VS Code        gui
goland    GoLand         gui
nvim      Neovim         terminal
```

## In the dashboard

The [dashboard](/guide/dashboard) wires the same model to two keys:

- <kbd>e</kbd> — **edit**: autolaunch the project default when one is set,
  otherwise open the picker.
- <kbd>o</kbd> — **configure**: always open the picker, so you can set the
  default or toggle autolaunch.

Both act on the selected workspace, or on the base checkout (at the project
root) when the cursor is on a project's [base row](/guide/dashboard#the-base-row).

## Relationship to `wf open`

[`wf open`](/reference/commands#wf-open) is about *reaching* a workspace: inside
tmux it jumps to the workspace's window, and only **outside** tmux does it fall
back to launching the resolved default editor (repo default → global default →
first detected). Reach for `wf edit` whenever you specifically want an editor —
or want to choose one.
