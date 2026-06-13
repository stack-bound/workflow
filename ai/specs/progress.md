# AutoCode — Build Progress

> **Living document.** Update this file as the build progresses — tick items, move milestone status, and append dated notes. It is the single source of truth for "where are we?". See [`build-plan.md`](./build-plan.md) for the full spec and rationale.

## How to use this file
- Update the **Status** of each task as it moves: `[ ]` todo → `[~]` in progress → `[x]` done. Use `[!]` for blocked.
- When a milestone's tasks are all `[x]`, set its **Status** to ✅ Done and record the date.
- Append a dated entry to the **Changelog** whenever something meaningful lands or a decision changes.
- If a decision in `build-plan.md` changes, note it in **Decision changes** and update the build plan too.

## Status legend
`[ ]` todo · `[~]` in progress · `[x]` done · `[!]` blocked

## Milestone status at a glance
| Milestone | Status | Started | Completed |
|---|---|---|---|
| M1 — Walking skeleton (no AI) | ⬜ Not started | — | — |
| M2 — Single lead agent | ⬜ Not started | — | — |
| M3 — Orchestration | ⬜ Not started | — | — |
| M4 — Autonomy & ops | ⬜ Not started | — | — |

---

## M1 — Walking skeleton (NO AI): worktree/PR manager
**Goal:** de-risk the hardest infra and ship a usable browser worktree+PR manager with zero AI.

- [ ] Bootstrap AutoCode module inside the Go+Vue single-binary template (router, ACL hooks, SQLite wiring)
- [ ] Single SQLite DB shared with `stackllm` sessions (schema/migrations for domain data)
- [ ] `project/` — Project CRUD (path-based), base-image config, `Dockerfile`/`.devcontainer` detection, setup scripts, git remote
- [ ] `executor/` — `Executor` interface + Docker backend (`docker exec`, mounts)
- [ ] `workspace/` — create branch + git worktree; mount project root at stable path; start container; run setup scripts
- [ ] `diff/` — fs-watch + `git diff` vs base, stream over SSE
- [ ] Vue: live diff viewer
- [ ] Terminal — WebSocket → `docker exec` PTY → xterm.js in Vue
- [ ] `vcs/` — `Host` interface + GitHub App adapter (auth, push, open PR, status, merge)
- [ ] Commit (manual message) + open GitHub PR; direct local merge option
- [ ] Lifecycle — idle-stop container, persist worktree until archive/close, reaper + max-live-containers cap
- [ ] **M1 end-to-end verification** (see build-plan.md → Verification → M1)

## M2 — Single lead agent end-to-end
**Goal:** one lead agent does a task, verifies via Stop hook, opens a PR; can ask the user and survive restarts.

- [ ] `agent/` — lead agent over `stackllm` (OpenAI-compatible provider) running in the workspace container
- [ ] Baseline tool catalog through `Executor` (file read/list/grep/edit, bash-in-container, git ops)
- [ ] `creds/` — server-side provider credential store on `stackllm/auth` + service credential (browser-managed, OAuth/API key)
- [ ] Model + effort selection at launch (agent-def → project → session override)
- [ ] `ask_user` tool (1–N structured questions + free text)
- [ ] `session/` — durable suspend/resume state machine (`awaiting-user`), resume via UI **and** API
- [ ] AI-generated commit/PR text from the diff
- [ ] `hooks/` — hook runner + `Stop` hook verification (harness-enforced, feed-back-until-clean)
- [ ] **M2 end-to-end verification** (see build-plan.md → Verification → M2)

## M3 — Orchestration
**Goal:** lead + sub-agents with live observability and intervention; role definitions from `.claude/agents`.

- [ ] `spawn_subagent` tool — child `stackllm` loop run to completion, returns result to lead
- [ ] Vue: live sub-agent mini-windows (status + transcript)
- [ ] Intervention channel — attach/watch/pause/inject-steer/resume on any agent
- [ ] `agentdef/` — `.claude/agents` parser → `stackllm` configs (incl. namespaced extensions); lead default overridable
- [ ] `PreToolUse` hook gating + per-agent `tools` allow-list enforcement
- [ ] Agent picker at launch (e.g. quick-fix vs feature-orchestrator)
- [ ] **M3 end-to-end verification** (see build-plan.md → Verification → M3)

## M4 — Autonomy & ops
**Goal:** programmatic job intake, notifications, and production-ready ops.

- [ ] `job/` — programmatic job API (integrator-friendly) + job states + in-process runner with concurrency cap
- [ ] `events/` — event bus + SSE fan-out
- [ ] Outbound notifiers — webhook (→ArkPM), email, Slack/Discord, web-push (per project/user config)
- [ ] Full provider/credential management UI
- [ ] Reaper / resource-limit hardening
- [ ] **M4 end-to-end verification** (see build-plan.md → Verification → M4)

---

## Future / not now (tracked, not scheduled)
- [ ] Integrator app (separate, deterministic, no-AI) — ArkPM ↔ AutoCode bridge
- [ ] Stage-2: auto-clone a project from a GitHub URL
- [ ] Dev-server exposure (branch URLs / per-container port mapping/proxy)
- [ ] Native Anthropic provider in `stackllm` (re-enable Claude with full thinking/effort fidelity)
- [ ] Additional VCS adapters (GitLab/Gitea)

---

## Decision changes
_Record any deviation from `build-plan.md` here, with date and reason._
- _(none yet)_

## Changelog
- **2026-06-13** — Build plan + progress tracker authored from the design interview. No code yet.
