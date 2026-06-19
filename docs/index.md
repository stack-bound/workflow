---
layout: home

hero:
  name: WorkFlow
  text: One isolated workspace per piece of work
  tagline: A local-first, CLI-first tool that orchestrates git worktrees — with a live-status dashboard and tmux integration. See every workspace across your projects, review the diff, merge, and clean up from one cockpit.
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: Why WorkFlow?
      link: /guide/introduction
    - theme: alt
      text: View on GitHub
      link: https://github.com/stack-bound/workflow

features:
  - icon: 🌳
    title: A worktree per task
    details: "wf add <branch> spins up a branch and an isolated git worktree as its own directory — run setup, copy and symlink files into it automatically. Switch tasks without stashing or rebuilding."
    link: /guide/getting-started
    linkText: Create your first workspace
  - icon: 📊
    title: Live cross-project dashboard
    details: A TUI ledger of every project and its workspaces with live git status — ahead/behind, ±lines, dirty flag, and an active/clean state. Auto-refreshing, with a scrollable colorized diff viewer.
    link: /guide/dashboard
    linkText: Explore the dashboard
  - icon: 🪟
    title: tmux as a guest
    details: Inside tmux, WorkFlow creates real windows in your current session — one per workspace — that you navigate with your own keys. It never wraps or owns your session, and works fine without tmux too.
    link: /guide/tmux
    linkText: How tmux integration works
  - icon: ⌨️
    title: CLI-first & scriptable
    details: Every capability is a subcommand; the dashboard and sidebar are optional surfaces over the same engine. wf list --json makes the whole ledger easy to pipe into your own tooling.
    link: /reference/commands
    linkText: Command reference
  - icon: ⚙️
    title: Per-repo setup that just runs
    details: An optional .workFlow.yaml sets the base branch, the worktree location, setup commands to run, and files to copy or symlink into every new worktree — so each workspace is ready the moment it exists.
    link: /guide/configuration
    linkText: Configure a repo
  - icon: ✅
    title: Review, merge, clean up
    details: Inspect a workspace's diff against its base, then merge it — WorkFlow merges the branch, removes the worktree, deletes the branch, and closes its tmux window in one step. Or rm to drop it without merging.
    link: /guide/concepts
    linkText: The workspace lifecycle
---
