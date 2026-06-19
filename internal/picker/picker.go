// Package picker is the "Open with…" editor chooser shared by the dashboard
// (rendered as a centered overlay) and the "wf edit" command (run as a
// standalone Bubble Tea program). It is a pure model: it never launches an
// editor or writes config itself — it reports the chosen editor and action back
// to its caller, which keeps it unit-testable and reusable in both surfaces.
package picker

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/stack-bound/workflow/internal/ide"
)

// Action is what the user asked the picker to do with the highlighted editor.
type Action int

const (
	// Cancel means the picker was dismissed without a choice.
	Cancel Action = iota
	// Launch opens the highlighted editor now.
	Launch
	// SetDefault pins the highlighted editor as the repo default.
	SetDefault
	// SetDefaultAutolaunch pins the default and enables autolaunch.
	SetDefaultAutolaunch
)

// Result is the picker's outcome. IDE is meaningful for every action but Cancel.
type Result struct {
	Action Action
	IDE    ide.IDE
}

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	boxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("6")).Padding(0, 2)
	selectedStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	defaultStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	dimStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
)

// Model is the picker state.
type Model struct {
	ides      []ide.IDE
	cursor    int
	defaultID string // currently configured default (marked with a star)
	title     string

	width, height int

	done   bool
	result Result
}

// New builds a picker over the detected editors, ordering the configured
// default first (and pre-selecting it) so a quick Enter launches it.
func New(ides []ide.IDE, defaultID string) Model {
	return Model{
		ides:      orderDefaultFirst(ides, defaultID),
		defaultID: defaultID,
		title:     "Open with…",
	}
}

// orderDefaultFirst returns ides with the default id moved to the front, leaving
// the rest in catalog order.
func orderDefaultFirst(ides []ide.IDE, defaultID string) []ide.IDE {
	if defaultID == "" {
		return ides
	}
	out := make([]ide.IDE, 0, len(ides))
	var def *ide.IDE
	for i := range ides {
		if ides[i].ID == defaultID {
			d := ides[i]
			def = &d
			continue
		}
		out = append(out, ides[i])
	}
	if def == nil {
		return ides
	}
	return append([]ide.IDE{*def}, out...)
}

// Init satisfies tea.Model.
func (m Model) Init() tea.Cmd { return nil }

// Update handles navigation and the choose/cancel keys. On a terminal action it
// records the result, marks the picker done, and returns tea.Quit so a
// standalone program exits; an embedding caller checks Done() and swallows the
// quit.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.move(-1)
	case "down", "j":
		m.move(1)
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(m.ides) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "enter":
		return m.finish(Launch)
	case "d":
		return m.finish(SetDefault)
	case "a":
		return m.finish(SetDefaultAutolaunch)
	case "esc", "q", "ctrl+c":
		m.done = true
		m.result = Result{Action: Cancel}
		return m, tea.Quit
	}
	return m, nil
}

func (m *Model) move(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.ides) {
		m.cursor = len(m.ides) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// finish records a non-cancel action against the highlighted editor. With no
// editors to choose from it is a no-op so the picker stays open.
func (m Model) finish(a Action) (tea.Model, tea.Cmd) {
	i, ok := m.current()
	if !ok {
		return m, nil
	}
	m.done = true
	m.result = Result{Action: a, IDE: i}
	return m, tea.Quit
}

func (m Model) current() (ide.IDE, bool) {
	if m.cursor < 0 || m.cursor >= len(m.ides) {
		return ide.IDE{}, false
	}
	return m.ides[m.cursor], true
}

// Done reports whether the user has chosen or cancelled.
func (m Model) Done() bool { return m.done }

// Result returns the chosen editor and action (valid once Done()).
func (m Model) Result() Result { return m.result }

// View renders the picker centered on a full screen, for standalone use.
func (m Model) View() string {
	box := m.Box()
	if m.width == 0 || m.height == 0 {
		return box
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, box)
}

// Box renders just the bordered chooser, for an embedding caller (the
// dashboard) to composite over its own view.
func (m Model) Box() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(m.title))
	b.WriteString("\n\n")

	if len(m.ides) == 0 {
		b.WriteString(dimStyle.Render("No editors detected."))
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("Add one under `ides:` in your global config."))
		b.WriteString("\n\n")
		b.WriteString(helpStyle.Render("esc cancel"))
		return boxStyle.Render(b.String())
	}

	for i, x := range m.ides {
		b.WriteString(m.renderItem(i, x))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move · enter open · d default · a autolaunch · esc cancel"))
	return boxStyle.Render(b.String())
}

func (m Model) renderItem(i int, x ide.IDE) string {
	kind := "term"
	if x.GUI {
		kind = "gui"
	}
	star := ""
	if x.ID == m.defaultID {
		star = "  ★"
	}
	text := fmt.Sprintf("%-22s %-4s%s", x.Name, kind, star)
	if i == m.cursor {
		return "❯ " + selectedStyle.Render(text)
	}
	// Dim the kind tag and the default star on unselected rows.
	row := fmt.Sprintf("%-22s ", x.Name) + dimStyle.Render(fmt.Sprintf("%-4s", kind))
	if star != "" {
		row += defaultStyle.Render(star)
	}
	return "  " + row
}

// Run shows the picker as a standalone full-screen program and returns the
// user's choice.
func Run(ides []ide.IDE, defaultID string) (Result, error) {
	p := tea.NewProgram(New(ides, defaultID), tea.WithAltScreen())
	fm, err := p.Run()
	if err != nil {
		return Result{}, err
	}
	return fm.(Model).Result(), nil
}
