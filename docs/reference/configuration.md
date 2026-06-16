# Configuration Reference

Field-by-field lookup for both config files. For explanations and examples, see
the [Configuration guide](/guide/configuration).

## Global config

**Path:** `$XDG_CONFIG_HOME/workFlow/config.yaml` (typically
`~/.config/workFlow/config.yaml`). All fields optional. Manage with
`wf config path | show | edit`.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| [`editor`](/guide/configuration#editor) | string | `$VISUAL` → `$EDITOR` → autodetect → `vi` | Editor for `wf open` / open-in-editor |
| [`clipboard_cmd`](/guide/configuration#clipboard-cmd) | string | built-in clipboard | Copy command, run as `sh -c <cmd>` with the path on stdin |
| [`default_base`](/guide/configuration#default-base) | string | detected default branch | Fallback base branch for new workspaces |
| [`worktree_dir`](/guide/configuration#worktree-dir-global) | string | sibling `<repo>_worktrees` | Default base directory for all worktrees |

```yaml
# ~/.config/workFlow/config.yaml
editor: code
clipboard_cmd: "xclip -selection clipboard"
default_base: development
worktree_dir: ~/worktrees
```

## Per-repo `.workFlow.yaml`

**Path:** `.workFlow.yaml` at the repository root. All fields optional. Generate a
documented starter with `wf init`.

| Field | Type | Default | Description |
| --- | --- | --- | --- |
| [`base`](/guide/configuration#base) | string | global/detected default | Default base branch for new workspaces in this repo |
| [`worktree_dir`](/guide/configuration#worktree-dir) | string | global setting, else sibling dir | Where this repo's worktrees are created (overrides global) |
| [`setup`](/guide/configuration#setup) | list of strings | none | Commands run via `sh -c` in each new worktree after creation |
| [`copy`](/guide/configuration#copy-and-symlink) | list of strings | none | Repo-root-relative files copied into each new worktree |
| [`symlink`](/guide/configuration#copy-and-symlink) | list of strings | none | Repo-root-relative files symlinked into each new worktree |

```yaml
# .workFlow.yaml
base: development
# worktree_dir: ../myrepo_worktrees
setup:
  - npm install
copy:
  - .env.example
symlink:
  - .env
```

## Base-branch resolution

The base for a new workspace is the first of these that's set
([details](/guide/concepts#base-branch)):

1. `wf add --base <branch>`
2. `.workFlow.yaml` → `base`
3. global config → `default_base`
4. the repo's detected default (`origin/HEAD`, else `development` → `main` → `master`)
5. the current branch

## State &amp; paths

| Path | What |
| --- | --- |
| `$XDG_CONFIG_HOME/workFlow/config.yaml` | Global config |
| `$XDG_CONFIG_HOME/workFlow/registry.json` | The registry — registered projects + tracked worktrees (the only persisted state) |
| `<repo>_worktrees/<branch-slug>` | Default location of a workspace's worktree (a sibling of the repo) |

`$XDG_CONFIG_HOME` defaults to `~/.config`.
