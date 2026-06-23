package dashboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// menu is a small vertical-list popup: a title and a set of labelled options
// the user moves between with the arrow keys, choosing one with enter. It is a
// pure value (no engine calls) so the dashboard can drive and test it the same
// way it drives the picker. The selected option's id is what the caller acts
// on.
type menu struct {
	title  string
	items  []menuItem
	cursor int
}

// menuItem is one selectable row. id is the stable action key the dashboard
// switches on; label and desc are display-only.
type menuItem struct {
	id    string
	label string
	desc  string
}

// menu styles — the rounded popup box and its rows, drawn in the dashboard's
// Catppuccin palette (the colour vars live in view.go, same package).
var (
	menuBoxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(cMauve).Padding(0, 2)
	menuTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(cMauve)
	menuSelStyle   = lipgloss.NewStyle().Bold(true).Foreground(cText).Background(cSel)
	menuDescStyle  = lipgloss.NewStyle().Foreground(cOverlay)
)

// move shifts the cursor by delta, clamped to the item range.
func (mn *menu) move(delta int) {
	mn.cursor += delta
	if mn.cursor < 0 {
		mn.cursor = 0
	}
	if mn.cursor >= len(mn.items) {
		mn.cursor = len(mn.items) - 1
	}
	if mn.cursor < 0 {
		mn.cursor = 0
	}
}

// current returns the highlighted item (false when the menu is empty).
func (mn menu) current() (menuItem, bool) {
	if mn.cursor < 0 || mn.cursor >= len(mn.items) {
		return menuItem{}, false
	}
	return mn.items[mn.cursor], true
}

// Box renders the bordered popup for compositing over the ledger via
// overlayBox. Each row shows its label (the selected one highlighted) with a
// dimmed description trailing it.
func (mn menu) Box() string {
	var b strings.Builder
	b.WriteString(menuTitleStyle.Render(mn.title))
	b.WriteString("\n\n")
	for i, it := range mn.items {
		b.WriteString(mn.renderItem(i, it))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move · enter select · esc cancel"))
	return menuBoxStyle.Render(b.String())
}

// menuLabelWidth pads labels so the dimmed descriptions line up in a column.
const menuLabelWidth = 10

func (mn menu) renderItem(i int, it menuItem) string {
	label := pad(it.label, menuLabelWidth)
	if i == mn.cursor {
		row := "❯ " + menuSelStyle.Render(label)
		if it.desc != "" {
			row += " " + menuDescStyle.Render(it.desc)
		}
		return row
	}
	row := "  " + label
	if it.desc != "" {
		row += " " + menuDescStyle.Render(it.desc)
	}
	return row
}

// newProjectMenu builds the per-project action menu opened by enter on a
// project header row.
func newProjectMenu(project string) menu {
	return menu{
		title: "Project · " + project,
		items: []menuItem{
			{id: "rename", label: "Rename", desc: "Change the project's name"},
			{id: "delete", label: "Delete", desc: "Unregister it (repo on disk untouched)"},
		},
	}
}
