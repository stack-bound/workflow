# Docs captures

Tooling to (re)generate the terminal and TUI visuals used in the documentation.

The dashboard is being reworked in lipgloss, so the docs currently use a styled
stand-in (`TerminalHero.vue`) plus tagged placeholders. Once the redesign lands,
run the capture script and drop the fresh visuals into the pages noted below.

## Run it

```sh
bash docs/capture/capture.sh
```

Requires `git` and `go` (for the CLI captures) and, for the TUI captures, `tmux`.
It is fully isolated: it builds `wf`, creates a throwaway git repo with two
workspaces under a private `XDG_CONFIG_HOME`, and drives the TUI on a **private
tmux server** (`tmux -L wf_test`). It never touches your real config, repos, or
tmux session, and it is **not** part of the normal `npm run build` — it's only
for refreshing screenshots.

Outputs go to `docs/capture/output/` (git-ignored):

| File | Format |
| --- | --- |
| `wf-list.txt` | Plain `wf list` table |
| `wf-list.json` | `wf list --json` |
| `dashboard-ledger.ansi` | Dashboard ledger view (ANSI) |
| `dashboard-diff.ansi` | Dashboard diff viewer (ANSI) |
| `sidebar.ansi` | `wf sidebar` strip (ANSI) |
| `tmux-windows.txt` | tmux window list (a window per workspace) |

## Capture targets → where they go

The doc pages reference these target names in their "screenshot pending"
admonitions. When you update a visual, replace the matching placeholder.

| Target | Page | Slot |
| --- | --- | --- |
| `dashboard-ledger` | `docs/guide/dashboard.md` | The `<TerminalHero />` panel / ledger screenshot |
| `dashboard-diff` | `docs/guide/dashboard.md` | "Diff viewer" section |
| `sidebar` | `docs/guide/tmux.md` | "Sidebar" section |
| `tmux-windows` | `docs/guide/tmux.md` | "The model" section |
| `wf-list` | `docs/guide/getting-started.md` | Step 3 (`wf list` output block) |

## Turning captures into images

The `.ansi` files keep their colors. To embed them as crisp images, convert with
any ANSI-aware tool, then save into `docs/.vitepress/public/img/` and reference
them from the pages (replacing the placeholder admonition). For example:

- [`freeze`](https://github.com/charmbracelet/freeze) —
  `freeze --execute "$wf list"` or render an existing capture to SVG/PNG.
- [`aha`](https://github.com/theZiz/aha) — `aha < output/dashboard-ledger.ansi > ledger.html` (then screenshot), or
- [`termsvg`](https://github.com/MrMarble/termsvg) / `asciinema` for animated recordings.

Keep image filenames aligned with the target names above (e.g.
`img/dashboard-ledger.png`) so the mapping stays obvious.
