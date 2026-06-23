package dashboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// popupBox renders a rounded, themed popup: an accent-coloured title, a body
// (already wrapped/sized by the caller), and a dim help line, all inside a
// border tinted with the same accent. It is the shared shell for the text-input
// and confirmation overlays, mirroring the menu/picker boxes so every prompt
// reads as the same kind of floating card.
func popupBox(title, body, help string, accent lipgloss.Color) string {
	var b strings.Builder
	b.WriteString(titleStyle.Foreground(accent).Render(title))
	b.WriteString("\n\n")
	b.WriteString(body)
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render(help))
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accent).
		Padding(0, 2).
		Render(b.String())
}

// popupTextWidth is the wrap width for a popup's body, clamped so the card stays
// a comfortable size on both narrow and wide terminals.
func (m Model) popupTextWidth() int {
	tw := m.width - 12
	if tw > 52 {
		tw = 52
	}
	if tw < 24 {
		tw = 24
	}
	return tw
}

// inputFieldWidth sizes the text-input field so its rendered line (prompt +
// value) fits inside the popup body without wrapping the box.
func (m Model) inputFieldWidth() int {
	w := m.popupTextWidth() - 10
	if w < 8 {
		w = 8
	}
	return w
}

// viewInputPopup draws the add-workspace / rename-project text input as a
// centered card over the ledger.
func (m Model) viewInputPopup() string {
	title := "New workspace · " + m.addProject
	help := "enter create · esc cancel"
	if m.mode == modeRename {
		title = "Rename project · " + m.menuProject
		help = "enter rename · esc cancel"
	}
	body := lipgloss.NewStyle().Width(m.popupTextWidth()).Render(m.input.View())
	box := popupBox(title, body, help, cMauve)
	return overlayBox(m.viewLedger(), box, m.width, m.height)
}

// viewConfirm draws a y/n confirmation as a centered card, coloured by what is
// at stake: red for a destructive action, green when it is safe, peach for a
// merge caution.
func (m Model) viewConfirm() string {
	c := m.confirm
	accent := c.accent()
	body := lipgloss.NewStyle().Width(m.popupTextWidth()).Foreground(accent).Render(c.message())
	box := popupBox(c.title(), body, "y confirm · n/esc cancel", accent)
	return overlayBox(m.viewLedger(), box, m.width, m.height)
}
