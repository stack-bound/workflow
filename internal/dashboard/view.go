package dashboard

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/status"
	"github.com/stack-bound/workflow/internal/workspace"
)

// theme is the dashboard's palette — Catppuccin Mocha hex values rather than
// bare ANSI indices, so the look is consistent across terminal themes (lipgloss
// down-samples to the terminal's colour profile automatically). Colours are
// named by role at the var block below so call sites read semantically.
var (
	cCrust   = lipgloss.Color("#11111b") // near-black, for text on bright bars
	cText    = lipgloss.Color("#cdd6f4") // primary foreground
	cSubtext = lipgloss.Color("#a6adc8") // secondary foreground
	cOverlay = lipgloss.Color("#6c7086") // dim detail / help
	cSurface = lipgloss.Color("#313244") // panel / footer background
	cSel     = lipgloss.Color("#45475a") // selected-row background
	cMauve   = lipgloss.Color("#cba6f7") // primary accent (header, projects)
	cBlue    = lipgloss.Color("#89b4fa") // clean state
	cGreen   = lipgloss.Color("#a6e3a1") // active state / additions
	cPeach   = lipgloss.Color("#fab387") // dirty / uncommitted
	cRed     = lipgloss.Color("#f38ba8") // errors / deletions
	cTeal    = lipgloss.Color("#94e2d5") // branch refs
	cLav     = lipgloss.Color("#b4befe") // tmux-open indicator
)

// styles
var (
	headerBarStyle = lipgloss.NewStyle().Foreground(cCrust).Background(cMauve).Bold(true)
	footerBarStyle = lipgloss.NewStyle().Foreground(cSubtext).Background(cSurface)

	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(cMauve)
	colHeadStyle = lipgloss.NewStyle().Foreground(cOverlay).Bold(true)

	projectStyle = lipgloss.NewStyle().Bold(true).Foreground(cMauve)
	accentBar    = lipgloss.NewStyle().Foreground(cMauve)
	pathStyle    = lipgloss.NewStyle().Foreground(cOverlay)
	baseRefStyle = lipgloss.NewStyle().Foreground(cTeal)
	connStyle    = lipgloss.NewStyle().Foreground(cOverlay)

	activeStyle = lipgloss.NewStyle().Foreground(cGreen)
	cleanStyle  = lipgloss.NewStyle().Foreground(cBlue)
	dirtyState  = lipgloss.NewStyle().Foreground(cPeach)
	errStyle    = lipgloss.NewStyle().Foreground(cRed)

	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(cText).Background(cSel)
	helpStyle     = lipgloss.NewStyle().Foreground(cOverlay)
	okStyle       = lipgloss.NewStyle().Foreground(cGreen)

	diffAddStyle  = lipgloss.NewStyle().Foreground(cGreen)
	diffDelStyle  = lipgloss.NewStyle().Foreground(cRed)
	diffHunkStyle = lipgloss.NewStyle().Foreground(cTeal)
	diffMetaStyle = lipgloss.NewStyle().Foreground(cOverlay)

	dirtyStyle = lipgloss.NewStyle().Foreground(cPeach) // the * uncommitted-changes marker
	tmuxStyle  = lipgloss.NewStyle().Foreground(cLav)   // the ▣ tmux-window-open indicator
)

// Column widths for a worktree row. columnHeader reuses them so the headings
// line up with the values underneath.
const (
	colBranch = 26
	colState  = 9
	colAB     = 11
	colDiff   = 12
)

// wsLeadWidth is the visible width before a worktree row's branch name: cursor
// prefix (2) + tree connector (3) + agent-status cell (2) + state circle and a
// trailing space (2). columnHeader pads by this so "BRANCH" sits over the names.
const wsLeadWidth = 9

// Project (base) row column widths: the name pads to projNameCol so each
// project's base summary starts at the same column, and the base branch pads to
// baseRefCol so the clean/dirty word lines up across projects.
const (
	projNameCol = 16
	baseRefCol  = 16
)

// bodyHeight is the number of rows available for the diff viewport (title +
// blank above, help + scroll below).
func (m Model) bodyHeight() int {
	h := m.height - 4
	if h < 1 {
		return 1
	}
	return h
}

// View renders the active surface.
func (m Model) View() string {
	if !m.ready {
		return "loading…"
	}
	switch m.mode {
	case modeDiff:
		return m.viewDiff()
	case modePicker:
		return m.viewPicker()
	default:
		return m.viewLedger()
	}
}

// viewPicker draws the IDE chooser as a centered box over the ledger.
func (m Model) viewPicker() string {
	return overlayBox(m.viewLedger(), m.picker.Box(), m.width, m.height)
}

// overlayBox composites box centered over base, replacing the base rows the box
// spans (rows above and below it stay visible, so it reads as a floating popup).
// It centers within the body, leaving the last two lines (status + help) clear.
func overlayBox(base, box string, width, height int) string {
	baseLines := strings.Split(base, "\n")
	boxLines := strings.Split(box, "\n")

	// The ledger renders only its natural content height, which can be shorter
	// than the terminal. Pad it up to the full screen height so the centered box
	// has room — otherwise box rows past the end of the base get dropped below
	// and the box is clipped (its bottom border vanishes).
	for len(baseLines) < height {
		baseLines = append(baseLines, "")
	}

	boxW := 0
	for _, l := range boxLines {
		if w := lipgloss.Width(l); w > boxW {
			boxW = w
		}
	}
	leftPad := (width - boxW) / 2
	if leftPad < 0 {
		leftPad = 0
	}
	pad := strings.Repeat(" ", leftPad)

	// Vertically center the box in the body, keeping the header (first two lines)
	// and the status/help (last two) visible so it reads as a popup over the
	// ledger rather than a full-screen takeover.
	const top = 2
	avail := len(baseLines) - top - 2
	start := top + (avail-len(boxLines))/2
	if start < top {
		start = top
	}
	for i, bl := range boxLines {
		row := start + i
		if row < 0 {
			continue
		}
		// Extend the base when the box is taller than the screen so its bottom
		// rows still render instead of being clipped.
		for row >= len(baseLines) {
			baseLines = append(baseLines, "")
		}
		baseLines[row] = pad + bl
	}
	return strings.Join(baseLines, "\n")
}

func (m Model) viewLedger() string {
	var b strings.Builder
	b.WriteString(m.headerBar())
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		b.WriteString(helpStyle.Render("  No projects registered. Add one from the CLI: ") +
			baseRefStyle.Render("wf project add"))
		b.WriteString("\n")
		return b.String() + m.footer()
	}

	b.WriteString(columnHeader())
	b.WriteString("\n")
	for i, r := range m.rows {
		b.WriteString(m.renderRow(i, r))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.ledgerLegend())

	return b.String() + m.footer()
}

// counts totals projects and worktrees for the header summary.
func (m Model) counts() (projects, worktrees int) {
	for _, r := range m.rows {
		switch r.kind {
		case rowProject:
			projects++
		case rowWorkspace:
			worktrees++
		}
	}
	return projects, worktrees
}

// headerBar renders the full-width title bar: the app name on the left and a
// project/worktree summary on the right, on a solid accent background that
// spans the terminal width.
func (m Model) headerBar() string {
	projects, worktrees := m.counts()
	left := " ✦ WorkFlow"
	right := fmt.Sprintf("%d projects · %d worktrees ", projects, worktrees)
	w := m.width
	if w <= 0 {
		w = lipgloss.Width(left) + lipgloss.Width(right) + 1
	}
	gap := w - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	content := left + strings.Repeat(" ", gap) + right
	return headerBarStyle.Width(w).Render(content)
}

// columnHeader is the single dim heading row above the ledger. It is padded by
// wsLeadWidth so "BRANCH" aligns with the worktree branch names below it.
func columnHeader() string {
	h := strings.Repeat(" ", wsLeadWidth) +
		fmt.Sprintf("%-*s%-*s%-*s%-*s%s",
			colBranch, "BRANCH", colState, "STATE", colAB, "↓|↑", colDiff, "± LINES", "BASE")
	return colHeadStyle.Render(h)
}

// ledgerLegend is the one-line key beneath the ledger. The agent-status glyphs
// come from the resolved config so the key matches the rows above.
func (m Model) ledgerLegend() string {
	sep := helpStyle.Render("   ")
	look := m.global.StatusLook()
	parts := []string{
		legendGlyph(look.Look["working"]) + helpStyle.Render(" working"),
		legendGlyph(look.Look["waiting"]) + helpStyle.Render(" waiting"),
		activeStyle.Render("●") + helpStyle.Render(" active"),
		cleanStyle.Render("○") + helpStyle.Render(" clean"),
		tmuxStyle.Render("▣") + helpStyle.Render(" tmux open"),
		helpStyle.Render("↓behind|↑ahead vs base"),
		diffAddStyle.Render("+added") + helpStyle.Render(" ") + diffDelStyle.Render("-removed"),
		dirtyStyle.Render("*") + helpStyle.Render(" uncommitted"),
	}
	return "  " + strings.Join(parts, sep)
}

// legendGlyph renders a status glyph in its own colour for the legend.
func legendGlyph(l config.Look) string {
	if l.Glyph == "" {
		return ""
	}
	if l.Color != "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(l.Color)).Render(l.Glyph)
	}
	return l.Glyph
}

// agentLook returns the resolved Look for a workspace's effective agent status.
// The dashboard highlights only *active* agents, so an idle/absent status yields
// a zero Look (blank cell). The status is already TTL-resolved at refresh time.
func (m Model) agentLook(v *workspace.View) config.Look {
	if v == nil {
		return config.Look{}
	}
	st, ok := m.statuses[v.Worktree.Path]
	if !ok || st == status.Idle {
		return config.Look{}
	}
	return m.global.StatusLook().Look[string(st)]
}

// agentCell renders the fixed 2-column agent-status indicator. colored controls
// whether the glyph carries its state colour (off inside the highlighted
// selected row). An idle/absent status renders as two blanks so the columns to
// its right stay aligned with the header.
func agentCell(l config.Look, colored bool) string {
	if l.Glyph == "" {
		return "  "
	}
	out := l.Glyph
	if colored && l.Color != "" {
		out = lipgloss.NewStyle().Foreground(lipgloss.Color(l.Color)).Render(l.Glyph)
	}
	if pad := 2 - lipgloss.Width(l.Glyph); pad > 0 {
		out += strings.Repeat(" ", pad)
	}
	return out
}

// renderRow dispatches a flattened ledger row to the project (base) or worktree
// renderer, applying the cursor and the live tmux-open indicator.
func (m Model) renderRow(i int, r row) string {
	if r.kind == rowProject {
		return m.renderProjectRow(i, r)
	}
	return m.renderWorkspaceRow(i, r)
}

// renderProjectRow draws a project's section/base row: an accent bar, the
// project name, the branch the root checkout is on with its clean/dirty state,
// and the project path dimmed to the right. It is the launch target for the
// base branch (t/e), so the open indicator shows when its root window is up.
func (m Model) renderProjectRow(i int, r row) string {
	selected := i == m.cursor
	open := m.openPaths[r.projectPath]

	cursor := "  "
	if selected {
		cursor = "❯ "
	}

	var left string
	if selected {
		// One highlight over a plain (un-accented) body; the dim path trails to
		// the right outside the highlight.
		body := appendOpenPlain("▌ "+pad(r.project, projNameCol)+"  "+mainSummaryPlain(r.main), open)
		left = cursor + selectedStyle.Render(body)
	} else {
		left = cursor + accentBar.Render("▌ ") + projectStyle.Render(pad(r.project, projNameCol)) +
			"  " + mainSummaryStyled(r.main)
		if open {
			left += " " + tmuxStyle.Render("▣")
		}
	}
	return left + m.padPath(r.projectPath, left)
}

// padPath right-aligns the dim, home-shortened project path to the terminal
// width, given the already-rendered left segment. An over-long path is clipped
// from the left (keeping the informative tail); when there is no room the path
// is dropped rather than wrapped. On an unknown width it falls back to a fixed
// gap.
func (m Model) padPath(path, left string) string {
	if path == "" {
		return ""
	}
	path = shortenPath(path)
	if m.width <= 0 {
		return "  " + pathStyle.Render(path)
	}
	avail := m.width - lipgloss.Width(left) - 3 // 2-space gap + 1 right margin
	if avail < 8 {
		return ""
	}
	path = truncatePathLeft(path, avail)
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(path) - 1
	if gap < 2 {
		gap = 2
	}
	return strings.Repeat(" ", gap) + pathStyle.Render(path)
}

// shortenPath rewrites a leading home directory to ~ for a tidier display.
func shortenPath(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		if p == home {
			return "~"
		}
		if strings.HasPrefix(p, home+string(os.PathSeparator)) {
			return "~" + p[len(home):]
		}
	}
	return p
}

// truncatePathLeft clips p from the left to a visible width of w, marking the
// cut with a leading … so the (more useful) tail of the path survives.
func truncatePathLeft(p string, w int) string {
	if lipgloss.Width(p) <= w {
		return p
	}
	r := []rune(p)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > w {
		r = r[1:]
	}
	return "…" + string(r)
}

// mainSummaryStyled renders the base checkout's "● <branch> <state>" with accent
// colours; mainSummaryPlain is the same without colour, for the selected row.
// The branch is padded so the state word lines up across projects.
func mainSummaryStyled(mc workspace.MainCheckout) string {
	if mc.Err != nil || mc.Branch == "" {
		return errStyle.Render("● " + pad("(no branch)", baseRefCol))
	}
	mark, word, style := mainState(mc)
	return style.Render(mark) + " " + baseRefStyle.Render(pad(mc.Branch, baseRefCol)) + style.Render(word)
}

func mainSummaryPlain(mc workspace.MainCheckout) string {
	if mc.Err != nil || mc.Branch == "" {
		return "● " + pad("(no branch)", baseRefCol)
	}
	mark, word, _ := mainState(mc)
	return mark + " " + pad(mc.Branch, baseRefCol) + word
}

// mainState reports the circle, word, and colour for a base checkout: a peach
// "● dirty" when the root has uncommitted changes, else a blue "○ clean".
func mainState(mc workspace.MainCheckout) (mark, word string, style lipgloss.Style) {
	if mc.Dirty {
		return "●", "dirty", dirtyState
	}
	return "○", "clean", cleanStyle
}

// renderWorkspaceRow draws a worktree as a tree child: a connector (├─/└─), the
// agent-status cell, the aligned status columns, the live tmux-open indicator,
// and the worktree directory dimmed to the right. The selected row takes one
// highlight over the plain layout; every other row is accent-coloured field by
// field so a colour flags a status, not the whole line.
func (m Model) renderWorkspaceRow(i int, r row) string {
	selected := i == m.cursor
	open := r.view != nil && m.openPaths[r.view.Worktree.Path]

	cursor := "  "
	if selected {
		cursor = "❯ "
	}
	conn := connectorFor(m.isLastWorkspace(i))

	var left string
	if selected {
		left = cursor + connStyle.Render(conn) + agentCell(m.agentLook(r.view), false) +
			selectedStyle.Render(workspaceLine(r.view))
	} else {
		left = cursor + connStyle.Render(conn) + agentCell(m.agentLook(r.view), true) +
			workspaceStyled(r.view)
	}
	if open {
		left += " " + tmuxStyle.Render("▣")
	}
	path := ""
	if r.view != nil {
		path = r.view.Worktree.Path
	}
	return left + m.padPath(path, left)
}

// isLastWorkspace reports whether the worktree row at i is the final child of
// its project (the next row is a different project or the end), so the tree
// connector can switch from ├─ to └─.
func (m Model) isLastWorkspace(i int) bool {
	next := i + 1
	return next >= len(m.rows) || m.rows[next].kind == rowProject
}

// connectorFor returns the 3-cell tree connector for a worktree row.
func connectorFor(last bool) string {
	if last {
		return "└─ "
	}
	return "├─ "
}

// appendOpenPlain appends the colourless tmux-open indicator to a line, for use
// inside a selection highlight (where the teal glyph would clash with the
// highlight background).
func appendOpenPlain(line string, open bool) string {
	if open {
		return line + " ▣"
	}
	return line
}

// wsState reports the status circle and word for a workspace, plus the colour
// both carry: a filled ● "active" (green) when it holds work not yet in base
// (dirty, ahead, or an open PR), or a hollow ○ "clean" (blue) when nothing is
// outstanding.
func wsState(v *workspace.View) (mark, word string, style lipgloss.Style) {
	if v.Active() {
		return "●", "active", activeStyle
	}
	return "○", "clean", cleanStyle
}

// behindAhead renders the commit gap to base, behind first to match the column
// heading: ↓ counts commits on base this branch lacks, ↑ commits it has on top.
func behindAhead(v *workspace.View) string {
	return fmt.Sprintf("↓%d|↑%d", v.Stat.Behind, v.Stat.Ahead)
}

// changesText is the plain "+added -removed" diff summary against base, with a
// trailing * when the working tree has uncommitted changes.
func changesText(v *workspace.View) string {
	s := fmt.Sprintf("+%d -%d", v.Stat.Added, v.Stat.Deleted)
	if v.Stat.Dirty {
		s += " *"
	}
	return s
}

// fitBranch sizes a branch name to exactly the branch column: short names are
// space-padded, and a name wider than the column is clipped with a trailing … so
// an over-long branch can never push the columns after it out of alignment.
func fitBranch(name string) string {
	if lipgloss.Width(name) <= colBranch {
		return fmt.Sprintf("%-*s", colBranch, name)
	}
	r := []rune(name)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > colBranch {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// pad right-pads s with spaces to a visible width of w (no-op if already wider).
func pad(s string, w int) string {
	if d := w - lipgloss.Width(s); d > 0 {
		return s + strings.Repeat(" ", d)
	}
	return s
}

// workspaceLine formats a worktree's status columns WITHOUT accent colours. It
// is the layout reference and the body of the selected row (which gets one
// highlight).
func workspaceLine(v *workspace.View) string {
	w := v.Worktree
	if v.StatErr != nil {
		return fmt.Sprintf("! %s %s", fitBranch(w.Branch), v.StatErr.Error())
	}
	mark, word, _ := wsState(v)
	return mark + " " + fitBranch(w.Branch) +
		pad(word, colState) + pad(behindAhead(v), colAB) + pad(changesText(v), colDiff) + w.Base
}

// workspaceStyled formats a worktree's status columns with accent colours: the
// branch name stays default (it is the row's identity), the status circle and
// word carry the active/clean colour, the diff counts are green/red with a
// yellow * for uncommitted work, and the ahead/behind and base columns are
// dimmed as secondary detail.
func workspaceStyled(v *workspace.View) string {
	w := v.Worktree
	if v.StatErr != nil {
		return errStyle.Render(fmt.Sprintf("! %s %s", fitBranch(w.Branch), v.StatErr.Error()))
	}
	mark, word, style := wsState(v)
	return style.Render(mark) + " " +
		lipgloss.NewStyle().Foreground(cText).Render(fitBranch(w.Branch)) +
		style.Render(pad(word, colState)) +
		helpStyle.Render(pad(behindAhead(v), colAB)) +
		styledChanges(v) +
		helpStyle.Render(w.Base)
}

// styledChanges renders the diff column with semantic colour — green additions,
// red deletions, a yellow * for uncommitted work — padded to colDiff so the base
// column beyond it stays aligned.
func styledChanges(v *workspace.View) string {
	add := fmt.Sprintf("+%d", v.Stat.Added)
	del := fmt.Sprintf("-%d", v.Stat.Deleted)
	raw := add + " " + del
	colored := diffAddStyle.Render(add) + " " + diffDelStyle.Render(del)
	if v.Stat.Dirty {
		raw += " *"
		colored += " " + dirtyStyle.Render("*")
	}
	if p := colDiff - lipgloss.Width(raw); p > 0 {
		colored += strings.Repeat(" ", p)
	}
	return colored
}

func (m Model) viewDiff() string {
	header := titleStyle.Render("diff — "+m.diffTitle) + "\n\n"
	help := helpStyle.Render("↑/↓ scroll · r reload · q back")
	scroll := helpStyle.Render(fmt.Sprintf("%3.0f%%", m.vp.ScrollPercent()*100))
	footer := "\n" + help + "  " + scroll
	return header + m.vp.View() + footer
}

// footer renders the status line and a mode-specific help bar (full-width,
// on a dim surface so the screen reads as framed top and bottom).
func (m Model) footer() string {
	var status string
	switch {
	case m.mode == modeInput:
		status = m.input.View()
	case m.mode == modeConfirm:
		status = m.confirm.prompt()
	case m.status != "":
		if m.statusErr {
			status = errStyle.Render(m.status)
		} else {
			status = okStyle.Render(m.status)
		}
	}

	var help string
	switch m.mode {
	case modeInput:
		help = "enter create · esc cancel"
	case modeConfirm:
		help = "y confirm · n cancel"
	case modePicker:
		help = "↑/↓ move · enter open · d default · a autolaunch · esc cancel"
	default:
		// "e" edits (autolaunch or picker) and "o" configures the editor; "t"
		// jumps to the tmux window when wf is running inside tmux. On a project
		// row these act on the base checkout at the project root.
		editHelp := "e edit · o config"
		if m.inTmux {
			editHelp += " · t term"
		}
		help = "↑↓ move · enter diff · a add · " + editHelp + " · c copy · m merge · x rm · r refresh · q quit"
	}
	return "\n " + status + "\n" + m.helpBar(help)
}

// helpBar renders the help text as a full-width footer bar.
func (m Model) helpBar(help string) string {
	content := " " + help
	w := m.width
	if w <= 0 {
		return footerBarStyle.Render(content)
	}
	if p := w - lipgloss.Width(content); p > 0 {
		content += strings.Repeat(" ", p)
	}
	return footerBarStyle.Width(w).Render(content)
}

// prompt renders the y/n confirmation line for a pending action. Merge is a
// plain caution; remove first weighs whether the workspace still holds work that
// deletion would discard — uncommitted changes or commits not yet on base —
// warning in red when it does (or when status is unknown) and reassuring in
// green when the branch is safe to drop.
func (c confirm) prompt() string {
	if c.action != "rm" {
		return errStyle.Render(fmt.Sprintf("Merge %s/%s? [y/n]", c.project, c.branch))
	}
	if risk := c.removeRisk(); risk != "" {
		return errStyle.Render(fmt.Sprintf("Remove %s/%s? This discards %s — work will be lost. Are you sure? [y/n]", c.project, c.branch, risk))
	}
	return okStyle.Render(fmt.Sprintf("Safe to remove %s/%s: no uncommitted changes, nothing unmerged vs %s. Are you sure? [y/n]", c.project, c.branch, c.base))
}

// removeRisk describes the work that removing this workspace would lose, or ""
// when the branch is clean and fully merged into base (safe to drop). The +/-
// diff counts are deliberately *not* used as a safety signal: a branch merely
// behind base shows a non-zero diff while holding no work of its own. A failed
// status check returns a cautious note rather than vouching for safety.
func (c confirm) removeRisk() string {
	if c.statErr {
		return "changes that couldn't be verified (git status unavailable)"
	}
	var parts []string
	if c.stat.Dirty {
		parts = append(parts, "uncommitted changes")
	}
	if c.stat.Ahead > 0 {
		noun := "commit"
		if c.stat.Ahead > 1 {
			noun = "commits"
		}
		parts = append(parts, fmt.Sprintf("%d unmerged %s (+%d -%d vs %s)", c.stat.Ahead, noun, c.stat.Added, c.stat.Deleted, c.base))
	}
	return strings.Join(parts, " and ")
}

// colorizeDiff applies semantic colors to a unified diff for the viewport.
func colorizeDiff(s string) string {
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		switch {
		case strings.HasPrefix(ln, "+++"), strings.HasPrefix(ln, "---"):
			lines[i] = diffMetaStyle.Render(ln)
		case strings.HasPrefix(ln, "@@"):
			lines[i] = diffHunkStyle.Render(ln)
		case strings.HasPrefix(ln, "+"):
			lines[i] = diffAddStyle.Render(ln)
		case strings.HasPrefix(ln, "-"):
			lines[i] = diffDelStyle.Render(ln)
		case strings.HasPrefix(ln, "diff "), strings.HasPrefix(ln, "index "),
			strings.HasPrefix(ln, "new file"), strings.HasPrefix(ln, "deleted file"),
			strings.HasPrefix(ln, "#"):
			lines[i] = diffMetaStyle.Render(ln)
		}
	}
	return strings.Join(lines, "\n")
}
