#!/usr/bin/env bash
#
# Regenerate terminal/TUI captures for the WorkFlow docs.
#
# It builds `wf`, stands up a fully ISOLATED sandbox (a private XDG config dir
# and a throwaway git repo with a couple of worktrees), then captures the real
# CLI output and — if tmux is available — the dashboard and sidebar via a
# PRIVATE tmux server. Nothing touches your real config, repos, or tmux session.
#
# Run it after the dashboard's lipgloss redesign to refresh the visuals:
#
#     bash docs/capture/capture.sh
#
# Outputs land in docs/capture/output/ (git-ignored). See README.md for which
# capture fills which slot in the docs, and how to turn the .ansi files into
# images.
#
# tmux safety (per the repo's CLAUDE.md): this ONLY ever uses a private server
# on the `wf_test` socket (`tmux -L wf_test`) and ONLY ever kills that server.
# It never runs a bare `tmux kill-server`, and unsets $TMUX so it cannot land on
# your interactive session.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$here/../.." && pwd)"
out="$here/output"
sock="wf_test"
cols=100
rows=34

command -v git >/dev/null || { echo "error: git is required" >&2; exit 1; }
command -v go  >/dev/null || { echo "error: go is required"  >&2; exit 1; }

mkdir -p "$out"
sandbox="$(mktemp -d)"
cleanup() {
  tmux -L "$sock" kill-server 2>/dev/null || true
  rm -rf "$sandbox"
}
trap cleanup EXIT

# Isolate everything wf reads/writes.
export XDG_CONFIG_HOME="$sandbox/config"
export XDG_DATA_HOME="$sandbox/data"
mkdir -p "$XDG_CONFIG_HOME" "$XDG_DATA_HOME"
unset TMUX   # never operate on the developer's tmux server

wf="$sandbox/wf"
echo "› building wf…"
( cd "$repo_root" && go build -o "$wf" ./cmd/wf )

# A throwaway project with two workspaces and some activity, so the ledger shows
# active/clean states, ahead/behind, and a dirty marker.
proj="$sandbox/acme-api"
mkdir -p "$proj"
git -C "$proj" init -q -b development
git -C "$proj" config user.email demo@example.com
git -C "$proj" config user.name "WorkFlow Demo"
printf 'hello\n' > "$proj/README.md"
git -C "$proj" add -A
git -C "$proj" commit -qm "init"

echo "› seeding sandbox project + workspaces…"
"$wf" project add "$proj" --name acme-api >/dev/null
"$wf" add feature-login  --project acme-api --no-setup >/dev/null
"$wf" add fix-cache-key  --project acme-api --no-setup >/dev/null

# feature-login: one commit ahead of base + an uncommitted change (→ active, dirty).
wt="$("$wf" path feature-login --project acme-api)"
printf 'login feature\n' >> "$wt/README.md"
git -C "$wt" commit -qam "wip: login"
printf 'work in progress\n' >> "$wt/README.md"

echo "› capturing CLI output…"
"$wf" list        > "$out/wf-list.txt"
"$wf" list --json > "$out/wf-list.json"

if command -v tmux >/dev/null; then
  echo "› capturing TUI via private tmux server ($sock)…"
  tmux -L "$sock" kill-server 2>/dev/null || true

  # The server inherits XDG_CONFIG_HOME/XDG_DATA_HOME exported above.
  snap() { # snap <output-name> [keys-to-send...]
    local name="$1"; shift
    tmux -L "$sock" new-session -d -x "$cols" -y "$rows" -s cap "$wf"
    sleep 1.5
    for key in "$@"; do tmux -L "$sock" send-keys -t cap "$key"; sleep 0.6; done
    tmux -L "$sock" capture-pane -t cap -p -e > "$out/$name.ansi"
    tmux -L "$sock" kill-session -t cap 2>/dev/null || true
  }

  snap dashboard-ledger
  snap dashboard-diff d      # press 'd' to open the diff viewer

  # Sidebar runs as its own long-lived command.
  tmux -L "$sock" new-session -d -x "$cols" -y "$rows" -s side "$wf sidebar"
  sleep 1.5
  tmux -L "$sock" capture-pane -t side -p -e > "$out/sidebar.ansi"

  # The tmux window list, showing a real window per workspace.
  tmux -L "$sock" list-windows -a > "$out/tmux-windows.txt" 2>/dev/null || true

  tmux -L "$sock" kill-server 2>/dev/null || true
else
  echo "  (tmux not found — skipped dashboard-ledger, dashboard-diff, sidebar, tmux-windows)"
fi

echo "✓ captures written to docs/capture/output/"
