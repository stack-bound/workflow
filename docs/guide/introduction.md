# Introduction

**WorkFlow** (`wf`) is a local-first, CLI-first developer tool that orchestrates
**git worktrees** — one isolated workspace per piece of work — with live git
status, review, and merge from one cockpit. It's written in Go and works in any
terminal, lighting up with extra powers when you run it inside **tmux**.

## The problem

Switching between several pieces of in-flight work in a single checkout means
stashing changes, rebuilding, and losing your place. Git *worktrees* solve the
isolation problem — a separate working directory per branch — but managing them
by hand (creating them in the right place, running setup, copying env files,
remembering which is which, cleaning up after merge) is tedious.

WorkFlow makes worktrees a first-class workflow:

> **See every workspace across your projects in one dashboard → open a terminal
> in any of them → review the diff → commit and merge → clean up.**

## What it gives you

- **A worktree per task.** `wf add <branch>` creates a branch and an isolated
  worktree directory, runs your setup commands, and copies or symlinks the files
  each workspace needs (like `.env`).
- **A live ledger.** `wf list` or the interactive dashboard shows every
  workspace across every registered project with live git status: ahead/behind
  its base, lines added/removed, a dirty flag, and whether it's still **active**
  or **clean**.
- **Review and merge in one place.** Inspect a workspace's diff, then `wf merge`
  it — WorkFlow merges into the base branch, removes the worktree, deletes the
  branch, and (in tmux) closes its window, all in one step.
- **tmux when you want it.** Inside tmux, each workspace gets a real window in
  *your* session that you navigate with *your* keys. WorkFlow is a guest — it
  never wraps or owns your session — and everything still works without tmux.

## Design philosophy

These principles shape every command (read more in
[Architecture &amp; Design](/guide/architecture)):

- **Guest, not owner.** WorkFlow never bootstraps or captures its own tmux
  session. It creates real windows inside the session you already run.
- **Structured data → it draws; live programs → real terminals.** Lists, diffs,
  and the dashboard are rendered by WorkFlow; shells and editors run in real
  terminals — there's no embedded terminal.
- **CLI-first.** Every capability is a subcommand. The dashboard and sidebar are
  optional surfaces over the same engine, and `--json` keeps it scriptable.
- **tmux is optional and auto-detected.** The core works in any terminal; tmux
  is a power-up that adds window and jump commands.
- **Persist durable facts; derive the rest.** Only the project and worktree
  registry is stored. Git stats and "is a window open?" are computed live.

## Where to next

<div class="vp-doc">

- New here? Start with [Installation](/guide/installation), then the
  [Getting Started](/guide/getting-started) walkthrough.
- Want the mental model? Read [Core Concepts](/guide/concepts).
- Living in the terminal UI? See [The Dashboard](/guide/dashboard) and
  [tmux Integration](/guide/tmux).
- Looking something up? Jump to the [Command Reference](/reference/commands).

</div>
