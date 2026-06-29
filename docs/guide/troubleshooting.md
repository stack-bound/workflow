# Troubleshooting

Short answers to the things that most often trip people up. Each is grounded in
how `wf` actually behaves.

## `wf: command not found` after install

The binary installs to `~/.local/bin` (or your `INSTALL_DIR`), which the install
script adds to your `PATH`. Open a **new shell** or `source` your rc file so the
change takes effect. Confirm with:

```sh
which wf && wf version
```

## "needs a tmux session (no $TMUX detected)"

`wf close`, `wf resurrect`, and `wf sidebar` require tmux — they operate on
windows. Run them **inside** a tmux session. WorkFlow detects tmux via the `$TMUX`
environment variable, so it must be set (i.e. you're attached to a session).

`wf open` does **not** require tmux: outside tmux it falls back to opening the
worktree in your editor. See [tmux Integration](/guide/tmux#without-tmux).

## "base branch … does not exist"

When `wf add` creates a *new* branch, it first checks the resolved base exists,
and stops with a message like:

```
base branch "main" does not exist in project "myrepo" (available: development, release-1.4); set 'base:' in .workFlow.yaml or pass --base=<branch>
```

Fix it by either passing the right base, or recording it so you don't have to
each time:

```sh
wf add feature-x --base development     # one-off
```

```yaml
# .workFlow.yaml — make it the repo default
base: development
```

See the full [base-branch resolution order](/guide/concepts#base-branch). Running
`wf init` writes a `base:` that matches your repo's detected default.

## "git repo … is not registered as a project"

A workspace must belong to a registered project. `wf add` and `wf init` will
**offer to register** the current repo (showing its name and path); accept the
prompt, pass `--yes` to skip it, or register up front:

```sh
wf project add        # register the current repo
```

This only happens when you didn't name a project explicitly with `--project`.

## `wf copy` doesn't copy anything

`wf copy` needs a way to reach your clipboard. Either install a helper
(`xclip`/`xsel`/`wl-clipboard` on Linux, `pbcopy` on macOS), or set
`clipboard_cmd` in your [global config](/guide/configuration#clipboard-cmd).

If you set `clipboard_cmd`, remember it's run as `sh -c <cmd>` with the path on
**stdin**, so the command must read stdin (e.g. `xclip -selection clipboard`, not
a command that expects the path as an argument).

## `wf open` opens the wrong editor

WorkFlow no longer takes a single `editor` command — it [discovers the editors
installed on this machine](/guide/editors) and launches your chosen default.
Outside tmux, `wf open` (and the dashboard's edit key) use the resolved default:
the repo's `default_ide` → the global `default_ide` → the first detected editor.
Pick the one you want and pin it:

```sh
wf edit             # opens the picker; press d to pin a default (a to autolaunch)
wf edit --list      # see every detected editor and its id
```

Or set it by hand — `default_ide: <id>` in a repo's `.workFlow.yaml`, or in your
[global config](/guide/configuration#default-ide). Check what's resolved with
`wf config show` (it prints `# config editor: …`). If your editor isn't detected,
add it under [`ides:`](/guide/editors#custom-editors).

## "workspace … is ambiguous"

The same branch name exists in two registered projects. Disambiguate with
`--project`:

```sh
wf open feature-x --project myrepo
```

This applies to every workspace command that takes a branch (`path`, `open`,
`close`, `copy`, `merge`, `rm`, `forget`).

## "worktree path already exists"

`wf add` won't clobber an existing directory at the target worktree path. Remove
or move the stale directory, or choose a different branch name. If the workspace
is still tracked, clean it up properly with `wf rm <branch>`.

## Removing a workspace warns about lost work

`wf rm` (and the dashboard's <kbd>x</kbd>) refuse to silently discard work: if the
branch has uncommitted changes or commits not yet merged into base, removal needs
`--force`. That's by design — merge it with `wf merge` to keep the work, or pass
`--force` to drop it deliberately.

## Removing a workspace fails with "Permission denied"

If `wf rm` stops with something like:

```
error: failed to delete '…/myrepo_worktrees/feature-x': Permission denied
```

the worktree holds files **your user account can't delete** — almost always
files written as `root` by a Docker container that bind-mounted the worktree
(generated `docs/`, `node_modules`, build artifacts, and so on). Neither git nor
`wf` can remove root-owned files, so the directory has to be cleared with
elevated privileges. `wf` now reports this with the exact remedy. You have two
options:

```sh
# 1. Delete the files, then let wf finish the cleanup (removes branch + registration):
sudo rm -rf /path/to/repo_worktrees/feature-x
wf rm feature-x --force

# 2. Or drop just the wf registration now and deal with the files later:
wf forget feature-x
```

`wf rm` is **self-healing** — once the directory is gone (or was removed
out-of-band) a retry reconciles git's metadata, deletes the branch, and clears
the registry entry. In the dashboard, pressing <kbd>x</kbd> then <kbd>f</kbd>
forgets a stuck workspace without touching its files. See
[`wf forget`](/reference/commands#wf-forget).

To avoid it in the first place, run the container as your host user (e.g. Docker
Compose `user: "${UID}:${GID}"`, or `docker run --user "$(id -u):$(id -g)"`) so
bind-mounted files stay owned by you.

## Still stuck?

Open an issue on
[GitHub](https://github.com/stack-bound/workflow/issues) with the command you
ran and its output.
