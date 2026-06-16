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
wf config show     # print the effective config (with the resolved editor)
wf config edit     # open it in your editor (creating it if needed)
```

### Fields

```yaml
# ~/.config/workFlow/config.yaml — all fields optional
editor: code              # editor for `wf open` / open-in-editor
clipboard_cmd: "xclip -selection clipboard"  # custom copy command
default_base: development  # fallback base branch for new workspaces
worktree_dir: ~/worktrees  # default base directory for all worktrees
```

#### `editor`

The command used by `wf open` (outside tmux) and the dashboard's "open in
editor" action. When empty, WorkFlow resolves an editor in this order:

1. `editor` in this config,
2. `$VISUAL`,
3. `$EDITOR`,
4. the first of `code`, `vim`, `vi`, `nano` found on your `PATH`,
5. `vi` as a final fallback.

#### `clipboard_cmd`

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
