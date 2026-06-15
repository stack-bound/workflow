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
	doneStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	okStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))

	diffAddStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	diffDelStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	diffHunkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	diffMetaStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
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
	} else {
		for i, r := range m.rows {
			b.WriteString(m.renderRow(i, r))
			b.WriteString("\n")
		}
	}

	return b.String() + m.footer()
}

func (m Model) renderRow(i int, r row) string {
	selected := i == m.cursor
	prefix := "  "
	if selected {
		prefix = "❯ "
	}

	var text string
	if r.kind == rowProject {
		text = fmt.Sprintf("%s (%d)  %s", r.project, r.wsCount, r.projectPath)
		if selected {
			return prefix + selectedStyle.Render(text)
		}
		return prefix + projectStyle.Render(text)
	}

	// A leading ▣ marks a workspace with a tmux window open right now (derived
	// live each refresh); a plain indent keeps the columns aligned otherwise.
	indent := "  "
	if r.view != nil && m.openPaths[r.view.Worktree.Path] {
		indent = "▣ "
	}
	text = indent + workspaceLine(r.view)
	if selected {
		return prefix + selectedStyle.Render(text)
	}
	switch {
	case r.view.StatErr != nil:
		return prefix + errStyle.Render(text)
	case r.view.Active():
		return prefix + activeStyle.Render(text)
	default:
		return prefix + doneStyle.Render(text)
	}
}

// workspaceLine formats a single workspace's status line (without styling).
func workspaceLine(v *workspace.View) string {
	w := v.Worktree
	if v.StatErr != nil {
		return fmt.Sprintf("! %-22s %s", w.Branch, v.StatErr.Error())
	}
	mark, state := "○", "done"
	if v.Active() {
		mark, state = "●", "active"
	}
	ab := fmt.Sprintf("↑%d ↓%d", v.Stat.Ahead, v.Stat.Behind)
	changes := fmt.Sprintf("+%d -%d", v.Stat.Added, v.Stat.Deleted)
	if v.Stat.Dirty {
		changes += " *"
	}
	return fmt.Sprintf("%s %-22s %-7s %-9s %-11s base:%s", mark, w.Branch, state, ab, changes, w.Base)
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
