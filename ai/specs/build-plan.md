# AutoCode — Build Plan

A browser-based, self-hosted agentic dev environment, written in Go on top of the `stackllm` harness.

> Companion file: [`progress.md`](./progress.md) — the living build tracker. Update it as work lands.

## Context

We love Paseo's model — projects mapped to git repos, one-click **worktree workspaces**, an agent (Claude Code/Codex/Opencode) working in each, a live file-diff on the right, AI commit messages, and commit→PR→merge without leaving the app. But Paseo is an Electron desktop app that shells out to provider CLIs, which makes it awkward to run headless on a server and automate.

We want the same value delivered as a **server-runnable (or local) browser application written in Go**, built on our own `stackllm` harness library (embedded, no CLI to install — we talk to providers directly). The headline goal is automation: external systems (starting with our PM tool **ArkPM**) can hand work to a bot that picks it up, runs an agent in an isolated workspace, verifies its own work, opens a PR, and asks a human only when genuinely stuck. When attended, the same app is a great browser-based worktree + PR manager and agent cockpit.

The outcome: a single Go binary (built on our existing Go-backend + Vue-frontend template) that manages Projects → Workspaces → agent Sessions, runs everything in per-workspace Docker containers, and is fully API-drivable so any external system can automate it.

This document is the synthesis of a decision-by-decision design interview. Decisions are recorded below so the rationale travels with the plan.

---

## Decisions (resolved during design interview)

| Area | Decision |
|---|---|
| **Positioning** | AutoCode is a **standalone, API-first** app. It runs work autonomously (do task → open PR → report done) and raises a needs-input flag when it can't. ArkPM/others are external consumers, never a dependency. |
| **Deployment / repos** | AutoCode runs on a host (local *or* server). A **Project is just a filesystem path** on that host; the developer sets the repo up manually first. (Stage-2: auto-clone from a GitHub URL.) The "pick a local folder from the browser" problem is sidestepped. |
| **Object model** | **Project → Workspaces → Sessions.** A Workspace (git worktree + Docker container) is **first-class and agent-optional** — usable for hand-work with diff/terminal/PR tooling. A Session is an *optional* agent engagement attached to a Workspace. |
| **Agent topology** | **One lead agent** (the user's only conversational counterpart) + **sub-agents spawned via a tool** (each a `stackllm` loop run to completion, returning a result). Two channels: **conversation** (lead only) and **intervention** (attach/watch/pause/inject/resume on *any* agent). Sub-agents are **non-interactive but interruptible**. Lead renders live sub-agent mini-windows. |
| **Wait model** | **Durable suspend/resume.** Any turn-yield (end-of-turn message *or* a structured `ask_user` tool call) checkpoints the session to SQLite and suspends to `awaiting-user` — no held goroutine/context. Resume on reply via UI **or** API. `ask_user` supports 1–N structured questions with option buttons + free text. |
| **Notifications** | Internal **event bus**; live in-app push to Vue via SSE; **pluggable outbound notifiers** (webhook→ArkPM, email, Slack/Discord, web-push), configurable per project/user. |
| **Isolation** | **Docker container per workspace** from day one, behind an **Executor interface**; agent/setup commands run via `docker exec`. |
| **Container env** | Per-project **base image + in-container setup scripts**; auto-detect an existing `Dockerfile`/`.devcontainer` and offer to use it. |
| **Agent definitions** | Adopt the **`.claude/agents` markdown+frontmatter** convention (name, description, `tools` allow-list, model, effort + body = system prompt), compiled into `stackllm` configs. AutoCode-specific needs ride as **optional namespaced frontmatter keys** (e.g. spawnable sub-agents, container/network perms). Lead is a built-in default, overridable by a special agent file. |
| **Verification / hooks** | Mirror **Claude Code's hooks** (`settings.json`-style). **Harness-enforced** `Stop`/`SubagentStop` hooks run verification (tests/lint/QA); if not clean, the **hook output is fed back into the agent** and it may not finish. `PreToolUse` gates tools. Fully configurable per project/agent. |
| **Providers (v1)** | **OpenAI-compatible only** (OpenAI/Gemini/Ollama/etc.) via `stackllm` as-is. **No Anthropic/Claude in v1** (revisitable: add a native Anthropic provider to `stackllm` later for full thinking/effort fidelity). Effort knob maps to what the provider exposes (e.g. OpenAI `reasoning_effort`). |
| **Model/effort selection** | Resolves **agent-def default → project default → per-session launch override** (Paseo-style dropdowns at launch). |
| **Provider auth** | **Server-side credential store** built on `stackllm/auth` (API keys + OAuth flows, browser-managed). A designated **service credential** powers unattended bot runs (token stored + auto-refreshed). Per-user creds optional later. |
| **Git host** | **GitHub first**, behind a **VCS-host interface** (GitLab/Gitea later). Bot authenticates as a **GitHub App** (PAT fallback). AI-generated commit/PR text from the diff. Default = push-branch + open PR; direct local merge allowed for trusted/simple tasks. |
| **User auth / stack** | Provided by our **Go-backend + Vue-SPA single-binary template** with Users/Roles/Permissions ACL and **email+password+2FA**. AutoCode is a module on top. |
| **Persistence** | **One SQLite DB** for both AutoCode domain data *and* `stackllm` sessions. |
| **Work intake (v1)** | Manual UI trigger **+** a programmatic **job API**. Jobs persisted with states; an **in-process runner with a concurrency cap** (no queue framework). |
| **External integration** | A **separate, deterministic, no-AI "integrator" app** (built later) bridges ArkPM↔AutoCode: polls ArkPM, starts jobs via AutoCode's API, monitors, feeds results back, and owns its own concurrency. **Nothing built for this now** — just keep the job API integrator-friendly. |
| **Live diff** | `git diff` vs the workspace base branch, recomputed on **fs-watch** events (+ a nudge after each agent file-write), streamed to the Vue diff viewer over **SSE**. Reflects true on-disk state (agent *and* manual/terminal edits). |
| **Terminal** | Interactive terminal in v1: **WebSocket → `docker exec` PTY → xterm.js**. |
| **Lifecycle** | Container **idle-stops** after a configurable timeout; **worktree + branch persist** until explicit **archive/close** (removes container + worktree, optionally the branch). A **reaper** enforces a max-live-containers cap and cleans orphans. |

---

## What `stackllm` gives us vs. what we build

**Reuse from `stackllm`:** single-agent ReAct loop with per-block streaming hooks; Go-function tools with auto JSON-Schema; durable **SQLite session store with branching message trees**; `auth/` (token sources, storage, OAuth); `config/`/`profile/` provider management; OpenAI-compatible `provider/`; and its `web/` HTTP+SSE adapter as a starting point.

**Build on top (not native to `stackllm`):**
- **Orchestration layer** — the lead↔sub-agent model (sub-agents = a tool that runs a child `stackllm` loop to completion).
- **`.claude/agents` parser** → `stackllm` agent configs.
- **Hooks engine** (`Stop`/`SubagentStop`/`PreToolUse`) with block-and-feed-back.
- **Structured `ask_user`** tool + the durable suspend/resume state machine.
- **Executor** (Docker) and the workspace/worktree manager.
- **VCS-host interface** (GitHub App).
- **Event bus + notifiers**, live-diff streamer, terminal PTY bridge.

---

## Architecture overview

Single Go binary (our template) serving a Vue SPA. Major internal packages (names indicative):

- `project/` — Project CRUD: path, base image / Dockerfile-devcontainer detection, setup scripts, default agent, verification hooks, git remote.
- `workspace/` — Workspace lifecycle: create worktree → start container → run setup; idle-stop; archive/close; reaper.
- `executor/` — `Executor` interface; Docker backend (`docker exec`, mounts, PTY).
- `vcs/` — `Host` interface; GitHub App adapter (auth, push, open PR, status, merge).
- `agent/` — orchestration over `stackllm`: lead loop, `spawn_subagent` tool, `ask_user` tool, intervention (pause/inject/resume), model/effort resolution.
- `agentdef/` — `.claude/agents` + hooks (`settings.json`) loader/parser.
- `hooks/` — hook runner (events, blocking, feed-back).
- `session/` — durable suspend/resume state machine on top of `stackllm` sessions.
- `job/` — job intake (API + UI), states, in-process runner, concurrency cap.
- `events/` — event bus + pluggable notifiers (webhook/email/Slack/web-push) + SSE fan-out.
- `creds/` — provider credential store on `stackllm/auth` (service credential).
- `diff/` — fs-watch + `git diff` streamer.
- `httpapi/` — REST + SSE + WebSocket (terminal) endpoints; the integrator-friendly job API.

> **Git worktrees in containers:** a worktree's `.git` points back into the main repo (`../../.git/worktrees/...`). The container must see the project tree at a **stable, consistent path** for in-container git to resolve. Plan: mount the project root (main repo + worktrees) at a fixed path and run all git ops there.

---

## Milestones

### M1 — Walking skeleton (NO AI): the worktree/PR manager
Proves and de-risks the hardest infrastructure before any agent exists.
- Project CRUD (path-based) + per-project base image / Dockerfile-devcontainer detection + setup scripts.
- Workspace create: new branch + **git worktree** → **Docker container** (mount project root at stable path) → run setup scripts.
- **Live diff** (`git diff` vs base, fs-watch → SSE) in the Vue UI.
- **Interactive terminal** (WebSocket → `docker exec` PTY → xterm.js).
- Commit (manual message) + **GitHub PR** via `vcs/` GitHub App; direct local merge option.
- **Lifecycle**: idle-stop container, persist worktree, archive/close, reaper + concurrency cap.
- **Deliverable:** a usable browser worktree+PR manager with zero AI.

### M2 — Single lead agent end-to-end
- Lead agent via `stackllm` (OpenAI-compatible provider), running in the workspace container with file/bash/git tools through the `Executor`.
- **`ask_user`** tool + **durable suspend/resume** (`awaiting-user`, resume via UI/API).
- **AI commit/PR messages** from the diff.
- **`Stop`-hook verification** (harness-enforced, feed-back-until-clean).
- Model/effort selection at launch; **provider credential store** with the service credential.

### M3 — Orchestration
- **`spawn_subagent`** tool — child `stackllm` loops run to completion; live **sub-agent mini-windows**.
- **Intervention** channel: attach/watch/pause/inject-steer/resume on any agent.
- **`.claude/agents`** role loading (lead + sub-agents) + **`PreToolUse`** gating; agent picker at launch (e.g. "quick-fix" vs "feature-orchestrator").

### M4 — Autonomy & ops
- Programmatic **job API** (integrator-friendly) + job states + in-process runner/concurrency cap.
- **Event bus + outbound notifiers** (webhook/email/Slack/web-push) + live web push.
- Full provider/credential management UI; reaper/limits hardening.

### Future (explicitly not now)
- **Integrator app** (separate, deterministic, no-AI) for ArkPM and other systems.
- **Stage-2** auto-clone a project from a GitHub URL.
- **Dev-server exposure** (Paseo-style branch URLs / port mapping/proxy per container).
- **Native Anthropic provider** in `stackllm` (re-enable Claude with full thinking/effort fidelity).
- Additional VCS adapters (GitLab/Gitea).

---

## Verification (end-to-end, per milestone)

- **M1:** On a test repo, create a Project (path) → create a Workspace → confirm a container starts and setup scripts run (`docker ps`, logs) → edit a file in the terminal and in an editor → confirm the live diff updates over SSE → commit → confirm a GitHub PR opens with the expected branch/diff → archive and confirm container+worktree are removed and the reaper respects the cap.
- **M2:** Launch the lead agent with a simple task → watch it stream, edit files, and trigger the `Stop` hook → force a failing test and confirm the failure is fed back and the agent cannot finish until green → make the task ambiguous and confirm `ask_user` suspends the session durably (restart the binary; session resumes on answer via both UI and API) → confirm AI-generated commit/PR text.
- **M3:** Run the `feature-orchestrator` agent → confirm sub-agents appear as live windows, run non-interactively, and return results → pause a sub-agent, inject a steering instruction, resume → confirm `.claude/agents` roles and `PreToolUse` gating are honored.
- **M4:** POST a job to the API → confirm it runs through pending→running→done states and emits events → confirm a configured webhook + web-push fire on needs-input and done → exercise the concurrency cap with several simultaneous jobs.

---

## Open questions to confirm before/at implementation
- Exact template package layout (where Users/ACL/router live) so AutoCode modules slot in cleanly.
- Baseline agent tool catalog (proposed: file read/list/grep/edit, bash-in-container, git ops; web fetch/search optional) — gated by agent-def `tools` allow-list + `PreToolUse`.
- Default container base images to ship/recommend.
- GitHub App registration details (permissions, installation flow) for the bot identity.
