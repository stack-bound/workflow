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

	text = "  " + workspaceLine(r.view)
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
		c := m.confirm
		verb := "Merge"
		if c.action == "rm" {
			verb = "Remove (discard, deletes branch + unmerged work for)"
		}
		status = errStyle.Render(fmt.Sprintf("%s %s/%s? [y/n]", verb, c.project, c.branch))
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
		help = "↑/↓ move · enter diff · a add · o open · c copy · m merge · x rm · r refresh · q quit"
	}
	return "\n" + status + "\n" + helpStyle.Render(help)
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
