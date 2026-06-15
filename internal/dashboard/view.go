package dashboard

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/stack-bound/workflow/internal/workspace"
)

// styles
var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	projectStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("13"))
	activeStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	cleanStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	diffAddStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	diffDelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	diffHunkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	diffMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))

	dirtyStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // the * uncommitted-changes marker
	tmuxStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("14")) // the ▣ tmux-window-open indicator
)

// Column widths for a workspace row. ledgerHeader reuses them so the headings
// line up with the marks and numbers underneath.
const (
	colBranch = 22
	colState  = 7
	colAB     = 12 // fits the "behind ahead" heading
	colDiff   = 12
)

// bodyHeight is the number of rows available between the title and footer.
func (m Model) bodyHeight() int {
	h := m.height - 4 // title line + blank + 2 footer lines
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
	default:
		return m.viewLedger()
	}
}

func (m Model) viewLedger() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("WorkFlow — dashboard"))
	b.WriteString("\n\n")

	if len(m.rows) == 0 {
		b.WriteString(helpStyle.Render("No projects registered. Add one from the CLI: wf project add"))
		b.WriteString("\n")
		return b.String() + m.footer()
	}

	for i, r := range m.rows {
		b.WriteString(m.renderRow(i, r))
		b.WriteString("\n")
		// Label the columns under each non-empty project so the marks and
		// numbers below have a heading to read them against.
		if r.kind == rowProject && r.wsCount > 0 {
			b.WriteString(ledgerHeader())
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(ledgerLegend())

	return b.String() + m.footer()
}

// ledgerHeader is the dim column-heading row drawn beneath each project. The six
// leading spaces stand in for a workspace row's cursor (2) + tmux mark (2) +
// status circle and its trailing space (2), so "branch" sits above the names.
func ledgerHeader() string {
	h := fmt.Sprintf("      %-*s %-*s %-*s %-*s %s",
		colBranch, "branch", colState, "state", colAB, "behind|ahead", colDiff, "diff", "base")
	return helpStyle.Render(h)
}

// ledgerLegend is the one-line key beneath the ledger explaining every glyph and
// column, each sample drawn in the same colour it carries in the rows above.
func ledgerLegend() string {
	sep := helpStyle.Render("   ")
	parts := []string{
		activeStyle.Render("●") + helpStyle.Render(" active"),
		cleanStyle.Render("○") + helpStyle.Render(" clean"),
		tmuxStyle.Render("▣") + helpStyle.Render(" tmux open"),
		helpStyle.Render("↓behind|↑ahead vs base"),
		diffAddStyle.Render("+added") + helpStyle.Render(" ") + diffDelStyle.Render("-removed"),
		dirtyStyle.Render("*") + helpStyle.Render(" uncommitted"),
	}
	return "  " + strings.Join(parts, sep)
}

func (m Model) renderRow(i int, r row) string {
	selected := i == m.cursor
	prefix := "  "
	if selected {
		prefix = "❯ "
	}

	if r.kind == rowProject {
		text := fmt.Sprintf("%s (%d)  %s", r.project, r.wsCount, r.projectPath)
		if selected {
			return prefix + selectedStyle.Render(text)
		}
		return prefix + projectStyle.Render(text)
	}

	// A leading ▣ marks a workspace with a tmux window open right now (derived
	// live each refresh); a plain indent keeps the columns aligned otherwise.
	open := r.view != nil && m.openPaths[r.view.Worktree.Path]

	// The selected row takes one highlight over the plain layout; every other
	// row is accent-coloured field by field (workspaceStyled) so the green flags
	// a status, not the whole line.
	if selected {
		indent := "  "
		if open {
			indent = "▣ "
		}
		return prefix + selectedStyle.Render(indent+workspaceLine(r.view))
	}

	indent := "  "
	if open {
		indent = tmuxStyle.Render("▣") + " "
	}
	return prefix + indent + workspaceStyled(r.view)
}

// wsState reports the status circle and word for a workspace, plus the colour
// both carry: a filled ● "active" (green) when it holds work not yet in base
// (dirty, ahead, or an open PR), or a hollow ○ "clean" (blue) when nothing is
// outstanding. Both states get a colour so neither reads as the privileged one —
// "clean" describes the git state, not that you are finished.
func wsState(v *workspace.View) (mark, word string, style lipgloss.Style) {
	if v.Active() {
		return "●", "active", activeStyle
	}
	return "○", "clean", cleanStyle
}

// behindAhead renders the commit gap to base, behind first to match the column
// heading: ↓ counts commits on base this branch lacks, ↑ commits it has on top.
// The pipe ties the pair into one column under the "behind|ahead" heading.
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
// space-padded (as %-*s did), and a name wider than the column is clipped with a
// trailing … so an over-long branch can never push the columns after it — state,
// behind|ahead, diff, base — out of alignment with the header.
func fitBranch(name string) string {
	if lipgloss.Width(name) <= colBranch {
		return fmt.Sprintf("%-*s", colBranch, name)
	}
	// Reserve one cell for the … marker, dropping trailing runes until the name
	// plus the marker fits the column exactly (width-aware for multibyte names).
	r := []rune(name)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > colBranch {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}

// workspaceLine formats a workspace status line WITHOUT accent colours. It is the
// layout reference and the body of the selected row (which gets one highlight).
func workspaceLine(v *workspace.View) string {
	w := v.Worktree
	if v.StatErr != nil {
		return fmt.Sprintf("! %s %s", fitBranch(w.Branch), v.StatErr.Error())
	}
	mark, word, _ := wsState(v)
	return fmt.Sprintf("%s %s %-*s %-*s %-*s %s",
		mark, fitBranch(w.Branch), colState, word, colAB, behindAhead(v), colDiff, changesText(v), w.Base)
}

// workspaceStyled formats a workspace status line with accent colours only: the
// branch name stays in the default foreground (it is the row's identity), the
// status circle and word carry the active/clean colour, the diff counts are
// green/red with a yellow * for uncommitted work, and the ahead/behind and base
// columns are dimmed as secondary detail.
func workspaceStyled(v *workspace.View) string {
	w := v.Worktree
	if v.StatErr != nil {
		return errStyle.Render(fmt.Sprintf("! %s %s", fitBranch(w.Branch), v.StatErr.Error()))
	}
	mark, word, style := wsState(v)
	cols := []string{
		style.Render(mark),
		fitBranch(w.Branch),
		style.Render(fmt.Sprintf("%-*s", colState, word)),
		helpStyle.Render(fmt.Sprintf("%-*s", colAB, behindAhead(v))),
		styledChanges(v),
		helpStyle.Render(w.Base),
	}
	return strings.Join(cols, " ")
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
	if pad := colDiff - lipgloss.Width(raw); pad > 0 {
		colored += strings.Repeat(" ", pad)
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

// footer renders the status line and a mode-specific help line.
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
	default:
		// Inside tmux, "o" opens the editor and "t" jumps to the window; without
		// tmux there is just the single "open".
		openHelp := "o open"
		if m.inTmux {
			openHelp = "o edit · t term"
		}
		help = "↑/↓ move · enter diff · a add · " + openHelp + " · c copy · m merge · x rm · r refresh · q quit"
	}
	return "\n" + status + "\n" + helpStyle.Render(help)
}

// prompt renders the y/n confirmation line for a pending action. Merge is a
// plain caution; remove first weighs whether the workspace still holds work
// that deletion would discard — uncommitted changes or commits not yet on
// base — warning in red when it does (or when status is unknown) and
// reassuring in green when the branch is safe to drop.
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
// when the branch is clean and fully merged into base (safe to drop). Note the
// +/- diff counts are deliberately *not* used as a safety signal: a branch that
// is merely behind base shows a non-zero diff while holding no work of its own.
// A failed status check returns a cautious note rather than vouching for safety.
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
