# Getting Started

This walkthrough takes you from a fresh checkout to a merged workspace in a
handful of commands. It assumes `wf` is [installed](/guide/installation) and
you're inside a git repository.

## 1. Register the repo as a project

WorkFlow tracks **projects** (git repos) and the **workspaces** (worktrees) under
them. Register the current repo first:

```sh
cd ~/code/myrepo
wf project add            # registers the current directory's repo
```

::: tip
You don't strictly need to run this first — `wf init` and `wf add` will offer to
register the repo for you (or pass `--yes` to do it without a prompt). See
[Configuration](/guide/configuration) for `wf init`.
:::

## 2. Create a workspace

```sh
wf add feature-x
```

This creates a `feature-x` branch and an isolated git worktree for it (by
default a sibling directory, `../myrepo_worktrees/feature-x`), then runs any
setup commands and file copy/symlink operations from your
[`.workFlow.yaml`](/guide/configuration#per-repo-workflow-yaml). Inside tmux, it
also gives the workspace a real window.

The base branch the new branch starts from is resolved automatically — see
[base-branch resolution](/guide/concepts#base-branch). Override it per-command
with `--base`:

```sh
wf add hotfix --base release-1.4
```

## 3. See your workspaces

```sh
wf list
```

```ansi
PROJECT  BRANCH     STATE   BASE         A/B    CHANGES   PATH
myrepo   feature-x  active  development  +2/-0  +84 -12 * /home/you/code/myrepo_worktrees/feature-x
```

`STATE` is **active** when the workspace holds work not yet in its base
(uncommitted changes, commits ahead, or an open PR) and **done** otherwise. The
`CHANGES` column ends with `*` when the working tree is dirty. For a machine-
readable view, add `--json`.

## 4. Jump into a workspace

```sh
wf open feature-x          # inside tmux: jump to its window
                           # outside tmux: open the worktree in your editor
```

Prefer to `cd` into it in your current shell? Print the path:

```sh
cd "$(wf path feature-x)"
```

The [shell `cd` helper](/guide/shell-integration#cd-helper) wraps this in a
one-word command.

## 5. Review and merge

When the work is ready, merge it back into its base branch:

```sh
wf merge feature-x
```

This merges `feature-x` into its base, then removes the worktree, deletes the
branch, and (in tmux) closes its window — leaving everything clean. To inspect
the diff before merging, open the [dashboard](/guide/dashboard) and press
<kbd>d</kbd>.

## 6. Or remove without merging

Abandoning the work? Drop the workspace without merging:

```sh
wf rm feature-x            # add --force if it has uncommitted or unmerged work
```

## The whole loop

```sh
wf project add                 # register the current repo as a project
wf add feature-x               # create a branch + worktree (+ run setup)
wf list                        # see every workspace with live git status
wf path feature-x              # print the worktree path (for shell cd)
wf open feature-x              # jump to its tmux window (or open in $EDITOR)
wf merge feature-x             # merge into base, then remove worktree + branch
wf rm feature-x                # remove a workspace without merging
```

## Prefer a UI?

Run `wf` with no arguments to open the **dashboard** — a cross-project ledger
where you can browse workspaces, scroll diffs, and add/open/merge/remove without
remembering flags:

```sh
wf
```

See [The Dashboard](/guide/dashboard) for the full keymap.
