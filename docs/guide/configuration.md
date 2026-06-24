# Configuration

WorkFlow reads two optional config files: a **global** one for your personal
defaults, and a **per-repo** `.workFlow.yaml` for project-specific behavior.
Both are optional — sensible defaults apply when they're absent. For a terse
field-by-field lookup, see the [Configuration Reference](/reference/configuration).

## Global config

Your user-wide settings live at:

```
$XDG_CONFIG_HOME/workFlow/config.yaml
```

(usually `~/.config/workFlow/config.yaml`). Manage it with the `wf config`
subcommands:

```sh
wf config path     # print the config file path
wf config show     # print the effective config (and the resolved editor command)
wf config edit     # open it in your editor (creating it if needed)
```

### Fields

```yaml
# ~/.config/workFlow/config.yaml — all fields optional
clipboard_cmd: "xclip -selection clipboard"  # custom copy command
default_base: development  # fallback base branch for new workspaces
worktree_dir: ~/worktrees  # default base directory for all worktrees
default_ide: code          # fallback editor when a repo pins no default_ide
ides:                      # custom editors the picker can launch
  - id: lapce
    name: Lapce
    cmd: lapce
    gui: true
status: {}                 # agent-status icons/colours (see Agent Status guide)
```

#### `clipboard_cmd` {#clipboard-cmd}

The command `wf copy` uses to put a path on your clipboard. When set, it's run as
`sh -c <cmd>` with the path on **stdin** — so it must read stdin:

```yaml
clipboard_cmd: "xclip -selection clipboard"   # X11
# clipboard_cmd: "wl-copy"                      # Wayland
# clipboard_cmd: "pbcopy"                       # macOS
```

When empty, WorkFlow uses a built-in clipboard. (A clipboard helper is only
needed for `wf copy`.)

#### `default_base` {#default-base}

The base branch used for new workspaces when neither `wf add --base` nor the
repo's `.workFlow.yaml` specifies one. See the full
[base-branch resolution order](/guide/concepts#base-branch).

#### `worktree_dir` {#worktree-dir-global}

A default base directory for **all** worktrees. When empty, each repo's
worktrees go in a sibling directory, `<repo>_worktrees`. A per-repo
`worktree_dir` overrides this.

#### `default_ide` {#default-ide}

The fallback editor id for [`wf edit`](/guide/editors) (and `wf open` outside
tmux) when the repo you're in pins no `default_ide` of its own. Use an id from
`wf edit --list`. Pin it from the picker (press <kbd>d</kbd>) or set it by hand.
See the [Editors guide](/guide/editors#default-ide).

#### `ides`

Custom editors merged into WorkFlow's built-in catalog, so the picker can launch
an editor it doesn't ship with. Each entry takes an `id`, an optional `name`, the
launch `cmd` (the target directory is appended), and `gui: true` for a windowed
app that should launch detached. A custom `id` matching a built-in one overrides
it. See [Custom editors](/guide/editors#custom-editors).

#### `status`

Tunes the live [agent-status](/guide/agent-status) icons and colours shown in
tmux tabs, the dashboard, and the sidebar — the glyph preset, per-state glyph and
colour overrides, the tmux tab-colour mode, and the staleness window. Absent
means sensible defaults; see [Customising the icons](/guide/agent-status#customising-the-icons).

## Per-repo `.workFlow.yaml` {#per-repo-workflow-yaml}

Drop a `.workFlow.yaml` at a repository's root to control how *that repo's*
workspaces are created. Generate a documented starter with `wf init`:

```sh
wf init            # write an example .workFlow.yaml (and offer to register the repo)
wf init --force    # overwrite an existing one
wf init --yes      # register the repo without prompting
```

`wf init` detects the repo's default base branch (git's tracked default,
otherwise preferring `development` over `main`/`master`) and, on an interactive
terminal with several candidates, asks you to confirm or pick it — so it writes
the right `base:` instead of a hardcoded guess.

### Fields

```yaml
# .workFlow.yaml — per-repository WorkFlow settings. All fields optional.

# Default base branch for new workspaces in this repo.
base: development

# Default editor for `wf edit` in this repo (an id from `wf edit --list`).
# default_ide: goland
# autolaunch: true        # open it without showing the picker

# Where worktrees for this repo are created.
# Default: a sibling directory "<repo>_worktrees".
# worktree_dir: ../myrepo_worktrees

# Commands run (via sh -c) inside each new worktree after it is created.
setup:
  - npm install
  - npm run build

# Repo-root-relative files copied into each new worktree.
copy:
  - .env.example

# Repo-root-relative files symlinked into each new worktree.
symlink:
  - .env
```

#### `base`

The default base branch for new workspaces in this repo — second in the
[resolution order](/guide/concepts#base-branch), after the `--base` flag.

#### `default_ide` and `autolaunch` {#default-ide-and-autolaunch}

The editor [`wf edit`](/guide/editors) prefers for this repo's workspaces, and
whether to open it without the picker. `default_ide` is an editor id (from
`wf edit --list`); with `autolaunch: true`, `wf edit` launches it straight away
(use `--pick` to choose anyway). Pin both from the picker — <kbd>d</kbd> sets the
default, <kbd>a</kbd> also enables autolaunch — or set them by hand. See the
[Editors guide](/guide/editors#autolaunch).

#### `worktree_dir` {#worktree-dir}

Overrides where this repo's worktrees are created (taking precedence over the
global `worktree_dir`). Relative paths are resolved from the repo.

#### `setup`

Commands run with `sh -c` **inside each new worktree** right after it's created —
the place to install dependencies or build. Skip them for a single `wf add` with
`--no-setup`.

#### `copy` and `symlink`

Repo-root-relative files to **copy** (`copy`) or **symlink** (`symlink`) into
each new worktree. This is how per-workspace files that aren't committed — like
`.env` — come along automatically. Use `copy` for files each workspace should own
independently, and `symlink` for files that should track a single shared source.

::: warning Setup runs real commands
`setup` entries execute on your machine via `sh -c` when a workspace is created.
Treat `.workFlow.yaml` like any other executable project config and only run
`wf add` in repos you trust. (Container isolation is a planned feature.)
:::

## Where state lives

The [registry](/guide/concepts#the-registry) (registered projects + tracked
worktrees) is the only persisted state, stored as JSON alongside the global
config:

```
$XDG_CONFIG_HOME/workFlow/config.yaml     # global config (this page)
$XDG_CONFIG_HOME/workFlow/registry.json   # projects + worktrees
```

Everything else — git stats, open-window markers, active/clean status — is
derived live on every command.
