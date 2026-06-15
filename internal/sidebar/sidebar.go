// Package sidebar is WorkFlow's live "now" strip: the workspace terminal
// windows currently open in tmux, refreshed continuously, with a slot for
// hook-driven agent status (filled in Stage 3). It is a thin, glanceable Bubble
// Tea surface meant to run in a split tmux pane; selecting an entry jumps to its
// window (select-window in the user's current session).
//
// Where the dashboard is the full ledger of everything tracked (even with no
// terminal open), the sidebar shows only what is open right now.
package sidebar

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/registry"
	"github.com/stack-bound/workflow/internal/status"
	"github.com/stack-bound/workflow/internal/tmux"
)

// refreshInterval is how often the sidebar re-queries tmux for open windows.
const refreshInterval = 1500 * time.Millisecond

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	activeStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("6"))
	openStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// entry is one open workspace window.
type entry struct {
	winID   string
	project string
	branch  string
	path    string
	name    string       // tmux window name (fallback label when untracked)
	active  bool         // currently the session's active window
	state   status.State // live (TTL-resolved) agent status
}

// label is the human-readable name for the entry.
func (e entry) label() string {
	if e.project != "" && e.branch != "" {
		return e.project + "/" + e.branch
	}
	if e.name != "" {
		return e.name
	}
	return e.path
}

// Model is the sidebar state.
type Model struct {
	registryPath string
	look         config.ResolvedStatus // resolved status glyphs/colors/TTL
	entries      []entry
	cursor       int
	errMsg       string
	width        int
	height       int
}

type tickMsg time.Time

type entriesMsg struct {
	entries []entry
	err     error
}

// New builds a sidebar model backed by the given registry path (used to map an
// open window's worktree path back to its project/branch). The status look
// defaults to the built-in preset; Run overrides it with the user's config.
func New(registryPath string) Model {
	return Model{registryPath: registryPath, look: (&config.StatusConfig{}).Resolve()}
}

// Run starts the sidebar program with a resolved status presentation.
func Run(registryPath string, look config.ResolvedStatus) error {
	m := New(registryPath)
	m.look = look
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Init kicks off the first poll and the refresh tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refreshCmd(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) refreshCmd() tea.Cmd {
	rp := m.registryPath
	ttl := m.look.TTL
	return func() tea.Msg {
		wins, err := tmux.Windows()
		if err != nil {
			return entriesMsg{err: err}
		}
		store, _ := registry.Load(rp) // best-effort: labels degrade to window name
		es := buildEntries(wins, store)
		attachStatus(es, ttl)
		return entriesMsg{entries: es}
	}
}

// attachStatus reads each entry's agent-status file and resolves it through the
// TTL, so a stale working/waiting shows as idle. Entries without a project+branch
// (untracked windows) can't be keyed, so they stay idle.
func attachStatus(es []entry, ttl time.Duration) {
	now := time.Now()
	for i := range es {
		if es[i].project == "" || es[i].branch == "" {
			continue
		}
		st, ok, err := status.ReadFor(es[i].project, es[i].branch, es[i].path)
		if err != nil || !ok {
			continue
		}
		es[i].state = status.Effective(st.State, st.TS, ttl, now)
	}
}

// buildEntries keeps only WorkFlow-tagged windows, joins them with the registry
// for nice labels, and sorts them stably. It is pure for testability.
func buildEntries(wins []tmux.Window, store *registry.Store) []entry {
	var es []entry
	for _, w := range wins {
		if w.Workspace == "" {
			continue // not a workspace window
		}
		e := entry{winID: w.ID, path: w.Workspace, name: w.Name, active: w.Active}
		if store != nil {
			if wt := worktreeByPath(store, w.Workspace); wt != nil {
				e.project, e.branch = wt.Project, wt.Branch
			}
		}
		es = append(es, e)
	}
	sort.Slice(es, func(i, j int) bool {
		ip, jp := es[i].project, es[j].project
		// Registry-known windows first; untracked ones (no project) sink to the
		// bottom. Then by project, then branch.
		if (ip == "") != (jp == "") {
			return ip != ""
		}
		if ip != jp {
			return ip < jp
		}
		return es[i].branch < es[j].branch
	})
	return es
}

func worktreeByPath(store *registry.Store, path string) *registry.Worktree {
	for i := range store.Worktrees {
		if store.Worktrees[i].Path == path {
			return &store.Worktrees[i]
		}
	}
	return nil
}

// Update is the Bubble Tea event loop.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tickMsg:
		return m, tea.Batch(m.refreshCmd(), tickCmd())
	case entriesMsg:
		if msg.err != nil {
			m.errMsg = msg.err.Error()
			return m, nil
		}
		m.errMsg = ""
		m.entries = msg.entries
		if m.cursor >= len(m.entries) {
			m.cursor = len(m.entries) - 1
		}
		if m.cursor < 0 {
			m.cursor = 0
		}
		return m, nil
	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.entries)-1 {
			m.cursor++
		}
	case "r":
		return m, m.refreshCmd()
	case "enter":
		if m.cursor >= 0 && m.cursor < len(m.entries) {
			id := m.entries[m.cursor].winID
			return m, func() tea.Msg {
				_ = tmux.SelectWindow(id)
				return m.refreshCmd()()
			}
		}
	}
	return m, nil
}

// View renders the live strip.
func (m Model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("now"))
	b.WriteString("\n\n")

	switch {
	case m.errMsg != "":
		b.WriteString(errStyle.Render(m.errMsg))
		b.WriteString("\n")
	case len(m.entries) == 0:
		b.WriteString(statusStyle.Render("no open workspace windows"))
		b.WriteString("\n")
	default:
		for i, e := range m.entries {
			b.WriteString(m.renderEntry(i, e))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/↓ move · enter jump · r refresh · q quit"))
	return b.String()
}

func (m Model) renderEntry(i int, e entry) string {
	mark := "●"
	line := fmt.Sprintf("%s %-20s %s", mark, e.label(), m.statusGlyph(e.state))
	if i == m.cursor {
		return activeStyle.Render("❯ " + line)
	}
	return "  " + openStyle.Render(line)
}

// statusGlyph renders the agent-status glyph for a state (idle → branch glyph),
// in its configured colour. Falls back to a dim em dash if the glyph is unset.
func (m Model) statusGlyph(st status.State) string {
	if st == "" {
		st = status.Idle
	}
	l := m.look.Look[string(st)]
	if l.Glyph == "" {
		return statusStyle.Render("—")
	}
	if l.Color != "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color(l.Color)).Render(l.Glyph)
	}
	return l.Glyph
}
