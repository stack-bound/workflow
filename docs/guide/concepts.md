# Core Concepts

WorkFlow has a small object model. Understanding these five ideas explains every
command.

```
Project          a git repo at a path on this host
  └─ Workspace   a git worktree on a branch (with a base branch, status)
       └─ Terminal   a tracked shell — a real tmux window when in tmux
```

## Projects

A **project** is a registered git repository. Projects are the top-level unit:
every workspace belongs to one. Registering a repo just records its name and
path in the [registry](#the-registry) — it never modifies the repo on disk.

```sh
wf project add [path]     # register (default: current directory)
wf project ls            # list projects with their workspace counts
wf project rm <name>     # unregister (leaves the repo on disk)
```

A project's **name** defaults to the repo's directory name; if that's taken,
WorkFlow appends `-2`, `-3`, and so on. Override it with `--name`.

## Workspaces

A **workspace** is a git **worktree** on its own branch — an isolated working
directory you can build in without touching your main checkout. Each workspace
records its project, path, branch, base branch, and creation time.

```sh
wf add <branch>      # create branch + worktree (+ setup)
wf list              # list workspaces with live status
wf merge <branch>    # merge into base, then clean up
wf rm <branch>       # remove without merging
```

A workspace is referenced by its **branch name**. If the same branch exists in
two projects, disambiguate with `--project <name>`.

### Worktree placement

By default, worktrees are created as **siblings** of the repo, never nested
inside it:

```
~/code/myrepo                      # the project
~/code/myrepo_worktrees/feature-x  # a workspace (worktree) on branch feature-x
```

Branch names are slugified for the directory (e.g. `feature/login` →
`feature-login`). Change the location with `worktree_dir` in
[config](/guide/configuration#worktree-dir).

## Active vs. clean

Every workspace is either **active** or **clean** — this is the dashboard's key
column and drives cleanup nudges. A workspace is **active** when it still holds
work that isn't in its base branch:

> **active** = uncommitted changes **or** commits ahead of base **or** an open PR

When none of those hold — the branch is clean and merged — the workspace is
**clean** (shown as `done` in `wf list`) and safe to remove.

::: info Why "behind" isn't a risk signal
A workspace that is merely *behind* its base still shows a non-zero diff, but it
holds no work of its own. WorkFlow only treats uncommitted changes and *unmerged
commits* (ahead of base) as work that removal would discard.
:::

## Base branch

Each workspace has a **base** — the branch it was created from and will merge
back into. When you don't pass `--base`, WorkFlow resolves it in this order:

1. The `--base` / `-b` flag on `wf add`.
2. `base:` in the repo's [`.workFlow.yaml`](/guide/configuration#base).
3. `default_base` in the [global config](/guide/configuration#default-base).
4. The repo's **detected default branch** — `origin/HEAD` if set, otherwise the
   first of `development`, `main`, or `master` that exists.
5. The currently checked-out branch, as a last resort.

If you're creating a *new* branch and the resolved base doesn't exist, `wf add`
stops with a clear message listing the available branches — rather than surfacing
git's raw `fatal: invalid reference` error.

## The registry

The **registry** is WorkFlow's only persisted state: a small JSON file listing
registered projects and tracked worktrees. Everything else — git stats, whether
a tmux window is open, active/clean status — is **derived live** on every command
and dashboard refresh.

It lives under your config directory:

```
$XDG_CONFIG_HOME/workFlow/registry.json
```

Because the registry is the source of truth, WorkFlow can reconcile it against
live tmux after a restart — see [`wf resurrect`](/guide/tmux#resurrect). You
rarely touch this file directly; the `wf project` and workspace commands manage
it for you.

## Terminals

A **terminal** is a tracked shell for a workspace. Inside tmux it's a real
**window** at the worktree's path, tracked by name; outside tmux it's whatever
your [universal launcher](/guide/tmux#without-tmux) opens (your editor, a copied
path, etc.). Terminals are always derived live — WorkFlow asks tmux what's open
rather than storing it.
