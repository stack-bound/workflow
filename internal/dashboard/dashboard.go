// Package dashboard is WorkFlow's Bubble Tea TUI: the cross-project ledger
// (projects → worktrees with live git status and an active/done flag), a
// scrollable diff viewer, and actions wired straight to the engine. It works
// in any terminal; the tmux power-ups arrive in M3.
package dashboard

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/fsnotify/fsnotify"

	"github.com/stack-bound/workflow/internal/config"
	"github.com/stack-bound/workflow/internal/git"
	"github.com/stack-bound/workflow/internal/ide"
	"github.com/stack-bound/workflow/internal/launcher"
	"github.com/stack-bound/workflow/internal/picker"
	"github.com/stack-bound/workflow/internal/status"
	"github.com/stack-bound/workflow/internal/tmux"
	"github.com/stack-bound/workflow/internal/workspace"
)

// refreshInterval is how often the ledger re-derives live git status while the
// user is on the ledger view.
const refreshInterval = 4 * time.Second

// errEmptyProjectPath guards launching a base checkout with no known root.
var errEmptyProjectPath = errors.New("no project root path to open")

// mode is the active interaction surface.
type mode int

const (
	modeLedger mode = iota
	modeDiff
	modeInput
	modeConfirm
	modePicker
	modeMenu   // a popup action menu (currently the per-project menu)
	modeRename // text input for renaming a project
	modeError  // a popup surfacing a failed action's message (e.g. merge blocked)
)

// rowKind distinguishes the three line kinds in a project block: a project
// header (name + path, the add target), the base/main checkout row, and a
// worktree line.
type rowKind int

const (
	rowProject rowKind = iota
	rowMain
	rowWorkspace
)

// row is one rendered line in the ledger. A project block is a header
// (rowProject) → its base checkout (rowMain) → its worktrees (rowWorkspace).
// The rowMain row carries the branch the root is on and its dirty state, so the
// base branch can be launched (tmux/editor) and its uncommitted diff viewed
// without a worktree — it is the target of the base-launch keys.
type row struct {
	kind        rowKind
	project     string
	projectPath string
	wsCount     int
	main        workspace.MainCheckout
	view        *workspace.View
}

// confirm holds a pending destructive action awaiting y/n.
type confirm struct {
	action  string // "merge" | "rm" | "deleteProject"
	project string
	branch  string

	// wsCount is the number of workspaces the action would drop. It is only
	// meaningful for "deleteProject", where it drives the warning (and forces the
	// registry removal when non-zero).
	wsCount int

	// base and the live status snapshot let the "rm" prompt weigh whether the
	// removal would actually discard work (uncommitted changes or unmerged
	// commits) rather than warn unconditionally. Captured when the prompt opens.
	base    string
	stat    git.Stat
	statErr bool // status couldn't be derived; treat rm as unsafe
}

// confirmFor builds a pending confirmation for an action against a workspace,
// snapshotting its base and live status so the prompt can describe what (if
// anything) the action puts at risk.
func confirmFor(action string, v *workspace.View) confirm {
	return confirm{
		action:  action,
		project: v.Worktree.Project,
		branch:  v.Worktree.Branch,
		base:    v.Worktree.Base,
		stat:    v.Stat,
		statErr: v.StatErr != nil,
	}
}

// Model is the dashboard state.
type Model struct {
	mgr    *workspace.Manager
	global *config.Global
	self   string // path to the wf binary, for suspend-and-run actions
	inTmux bool   // tmux integration available (jump-to-window, open indicator)

	rows      []row
	cursor    int
	mode      mode
	openPaths map[string]bool         // worktree paths with a tmux window open right now
	statuses  map[string]status.State // worktree path → live (TTL-resolved) agent status

	watcher *fsnotify.Watcher // watches the status dir for instant updates

	vp          viewport.Model
	diffTitle   string
	diffProject string
	diffBranch  string

	input      textinput.Model
	addProject string

	// menu is the active popup action menu; menuProject is the project it acts
	// on (the only menu today is the per-project rename/delete menu, reached by
	// pressing enter on a project header row).
	menu        menu
	menuProject string

	confirm confirm

	// IDE picker overlay state. picker holds the chooser; the three fields below
	// remember which workspace it was opened for so its result can launch the
	// right path and persist to the right project.
	picker        picker.Model
	pickerProject string
	pickerBranch  string
	pickerPath    string

	status    string
	statusErr bool

	// errTitle/errBody hold the content of the modeError popup raised when a
	// suspend-and-run action fails, so the engine's message reads as a card over
	// the ledger instead of a buried status line.
	errTitle string
	errBody  string

	// startupTasks are best-effort jobs run concurrently off the launch path when
	// the program starts (see StartupTask). notices accumulates whatever they
	// report so the status line shows every notice rather than letting a later one
	// clobber an earlier one.
	startupTasks []StartupTask
	notices      []string

	width, height int
	ready         bool
}

// --- messages ---

type ledgerMsg struct {
	projects  []workspace.ProjectView
	openPaths map[string]bool
	statuses  map[string]status.State
	err       error
}

// statusChangedMsg fires when the status dir changes (an agent updated its
// state); it triggers an immediate refresh. watcherReadyMsg carries the lazily
// created fsnotify watcher back onto the model.
type statusChangedMsg struct{}

type watcherReadyMsg struct{ w *fsnotify.Watcher }

type diffMsg struct {
	title   string
	content string
	err     error
}

type actionMsg struct {
	msg     string
	err     error
	refresh bool

	// errTitle and detail describe a failed suspend-and-run action (see runSelf).
	// When errTitle is set the handler raises the failure as a themed popup
	// rather than a one-line status; detail carries the subprocess's captured
	// stderr, which holds the engine's real explanation (the process exit error
	// alone is just "exit status 1").
	errTitle string
	detail   string
}

// editMsg carries the result of resolving a workspace's editor preferences off
// the main loop (path lookup + per-project default/autolaunch + machine
// detection), so the key handler stays free of engine calls. Update then either
// autolaunches the default or opens the picker.
type editMsg struct {
	project     string
	branch      string
	path        string
	ides        []ide.IDE
	defaultID   string
	autolaunch  bool
	forcePicker bool
	err         error
}

type tickMsg time.Time

// noticeMsg carries the one-line result of a finished startup task (see
// StartupTask). A non-empty notice is appended to the status line; an empty one
// (the task had nothing to report) is ignored.
type noticeMsg struct{ text string }

// StartupTask is a unit of best-effort work run when the dashboard opens. The
// dashboard wraps each task in a tea.Cmd and fires them from Init, so they run
// concurrently off the launch path — a slow task never delays the ledger
// appearing — and surfaces the one-line string each returns in the status line
// (or shows nothing when it returns ""). Tasks are constructed in the cli layer,
// which may read settings, hit the network, etc.; passing them in keeps the
// dashboard package free of any dependency on cli. The Claude Code hook
// auto-update is the first such task — register more launch-time jobs alongside
// it rather than computing anything before Run.
type StartupTask func() string

// New builds a dashboard model over the given engine and config. Any startup
// tasks run concurrently when the program starts (see StartupTask and Init).
func New(mgr *workspace.Manager, global *config.Global, tasks ...StartupTask) Model {
	self, err := os.Executable()
	if err != nil || self == "" {
		self = "wf" // fall back to PATH lookup
	}
	ti := textinput.New()
	ti.Placeholder = "branch name"
	ti.CharLimit = 100
	ti.Prompt = "branch: "
	return Model{
		mgr:          mgr,
		global:       global,
		self:         self,
		inTmux:       tmux.Available(),
		input:        ti,
		status:       "loading…",
		startupTasks: tasks,
	}
}

// Run starts the dashboard program. Any startup tasks (see StartupTask) run
// concurrently off the launch path once the program starts; each surfaces a
// one-line notice in the status line when it has something to report, and the
// dashboard opens immediately no matter how slow a task is.
func Run(mgr *workspace.Manager, global *config.Global, tasks ...StartupTask) error {
	p := tea.NewProgram(New(mgr, global, tasks...), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// Init kicks off the first refresh, the auto-refresh tick (a safety net), the
// fsnotify watcher that makes status updates feel instant, and any registered
// startup tasks — all as concurrent commands, so none of them delay the first
// paint.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{m.refreshCmd(), tickCmd(), watchStatusCmd()}
	return tea.Batch(append(cmds, m.startupCmds()...)...)
}

// startupCmds wraps each registered startup task in a tea.Cmd that runs it off
// the main loop and reports its result via a noticeMsg. Running the tasks as
// commands (rather than computing their notices before launch) is what keeps a
// slow task from blocking the dashboard from opening.
func (m Model) startupCmds() []tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.startupTasks))
	for _, task := range m.startupTasks {
		task := task
		cmds = append(cmds, func() tea.Msg { return noticeMsg{text: task()} })
	}
	return cmds
}

// --- commands ---

func tickCmd() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) refreshCmd() tea.Cmd {
	inTmux := m.inTmux
	ttl := m.global.StatusLook().TTL
	return func() tea.Msg {
		pv, err := m.mgr.Ledger()
		msg := ledgerMsg{projects: pv, err: err}
		// Derive the live "window open?" set alongside git status. Best-effort:
		// a tmux query failure just leaves the indicators off.
		if inTmux {
			if open, oerr := tmux.OpenWorkspaces(); oerr == nil {
				msg.openPaths = open
			}
		}
		// Read each workspace's agent status (TTL-resolved). This is the safety
		// net; fsnotify drives the instant updates between refreshes.
		msg.statuses = readStatuses(pv, ttl)
		return msg
	}
}

// readStatuses reads each workspace's status file and resolves it through the
// TTL so a stale working status renders as idle (waiting/ready persist). It also
// reads each project's base/root checkout, keyed by the project-root path — the
// same key set-status writes — so the base row can show an agent working at the
// project root.
func readStatuses(projects []workspace.ProjectView, ttl time.Duration) map[string]status.State {
	now := time.Now()
	out := make(map[string]status.State)
	for _, pv := range projects {
		if root := pv.Project.Path; root != "" {
			if st, ok, err := status.ReadBase(pv.Project.Name, root); err == nil && ok {
				out[root] = status.Effective(st.State, st.TS, ttl, now)
			}
		}
		for _, v := range pv.Workspaces {
			wt := v.Worktree
			st, ok, err := status.ReadFor(wt.Project, wt.Branch, wt.Path)
			if err != nil || !ok {
				continue
			}
			out[wt.Path] = status.Effective(st.State, st.TS, ttl, now)
		}
	}
	return out
}

// watchStatusCmd creates the fsnotify watcher on the status dir (creating the
// dir first — fsnotify silently no-ops on a missing path) and hands it back via
// watcherReadyMsg. It returns nil on any failure, leaving the 4s tick as the
// fallback.
func watchStatusCmd() tea.Cmd {
	return func() tea.Msg {
		dir, err := status.EnsureDir()
		if err != nil {
			return nil
		}
		w, err := fsnotify.NewWatcher()
		if err != nil {
			return nil
		}
		if err := w.Add(dir); err != nil {
			_ = w.Close()
			return nil
		}
		return watcherReadyMsg{w: w}
	}
}

// listenStatusCmd blocks for one watcher event, coalesces any immediate burst,
// and emits a single statusChangedMsg. It is re-issued after each event to keep
// listening.
func (m Model) listenStatusCmd() tea.Cmd {
	w := m.watcher
	if w == nil {
		return nil
	}
	return func() tea.Msg {
		select {
		case _, ok := <-w.Events:
			if !ok {
				return nil
			}
		case _, ok := <-w.Errors:
			if !ok {
				return nil
			}
		}
		// Coalesce a burst of writes (a hook may touch several files) into one
		// refresh by draining whatever is already queued.
		for {
			select {
			case _, ok := <-w.Events:
				if !ok {
					return statusChangedMsg{}
				}
			default:
				return statusChangedMsg{}
			}
		}
	}
}

func (m Model) diffCmd(project, branch string) tea.Cmd {
	return func() tea.Msg {
		content, err := m.mgr.Diff(branch, project)
		return diffMsg{title: project + "/" + branch, content: content, err: err}
	}
}

// mainDiffCmd shows a project base checkout's uncommitted diff (the root's
// working-tree changes vs HEAD), since the trunk has no base of its own.
func (m Model) mainDiffCmd(project, branch string) tea.Cmd {
	return func() tea.Msg {
		content, err := m.mgr.MainDiff(project)
		title := project + " · " + branch + " (base)"
		return diffMsg{title: title, content: content, err: err}
	}
}

func (m Model) copyCmd(project, branch string) tea.Cmd {
	return func() tea.Msg {
		path, err := m.mgr.Path(branch, project)
		if err != nil {
			return actionMsg{err: err}
		}
		return m.copyPathCmd(path, branch)()
	}
}

// copyPathCmd copies a known path (e.g. a project root) to the clipboard,
// labelling the confirmation with label.
func (m Model) copyPathCmd(path, label string) tea.Cmd {
	return func() tea.Msg {
		if err := launcher.NewUniversal(m.global).CopyPath(path); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{msg: "copied path for " + label}
	}
}

// runSelf suspends the TUI and re-invokes the wf binary so engine operations
// that stream git/setup output to the terminal (add, merge, rm) display
// cleanly, then refreshes the ledger.
func (m Model) runSelf(okMsg string, args ...string) tea.Cmd {
	cmd := exec.Command(m.self, args...)
	// Tee the subprocess stderr: it still streams to the terminal during the
	// suspend (so add/merge setup output shows as before), but we also capture it
	// so a failure can surface the engine's real message in a popup. tea.ExecProcess
	// only wires Stderr to the terminal when it's nil, so setting it here is honoured.
	var stderr bytes.Buffer
	cmd.Stderr = io.MultiWriter(os.Stderr, &stderr)
	title := actionErrTitle(args)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return actionMsg{msg: okMsg, err: err, errTitle: title, detail: stderr.String(), refresh: true}
	})
}

// actionErrTitle names the failure popup for a suspend-and-run action from its
// wf subcommand (the first arg), so a blocked merge/add/rm/forget reads with a
// fitting header instead of a bare "Error".
func actionErrTitle(args []string) string {
	sub := ""
	if len(args) > 0 {
		sub = args[0]
	}
	switch sub {
	case "merge":
		return "Merge failed"
	case "add":
		return "Add failed"
	case "rm":
		return "Remove failed"
	case "forget":
		return "Forget failed"
	default:
		return "Action failed"
	}
}

// engineError pulls the engine's explanation out of a subprocess's captured
// stderr. The CLI prints its top-level failure as a final "wf: <message>" line
// (cli.Execute), which we want verbatim and without the prefix; any preceding
// git/setup progress (e.g. from a failed add) is discarded. Falls back to the
// process exit error when no such line is present.
func engineError(detail string, err error) string {
	found := ""
	for _, line := range strings.Split(detail, "\n") {
		line = strings.TrimSpace(line)
		if msg := strings.TrimPrefix(line, "wf: "); msg != line {
			found = msg // keep the last one: cli.Execute prints it after any progress
		}
	}
	if found != "" {
		return found
	}
	if err != nil {
		return err.Error()
	}
	return strings.TrimSpace(detail)
}

// openWindowCmd jumps to the workspace's tmux window (creating it if needed).
// Unlike the editor/add/merge actions it needs no suspend: select-window simply
// shifts focus to a peer window, leaving the dashboard running in its own.
func (m Model) openWindowCmd(project, branch string) tea.Cmd {
	return func() tea.Msg {
		path, err := m.mgr.Path(branch, project)
		if err != nil {
			return actionMsg{err: err}
		}
		if err := launcher.NewTmux().Open(path, launcher.IdleName(m.global, branch)); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{msg: "opened window for " + branch, refresh: true}
	}
}

// openMainWindowCmd jumps to (or creates) a tmux window on a project's base
// checkout at its root, so the trunk can be launched without a worktree. The
// window is named after the base branch and tagged with the root path, so the
// open indicator lights up on the project row like any other workspace.
func (m Model) openMainWindowCmd(project, path, branch string) tea.Cmd {
	return func() tea.Msg {
		if path == "" {
			return actionMsg{err: errEmptyProjectPath}
		}
		name := branch
		if name == "" {
			name = project
		}
		if err := launcher.NewTmux().Open(path, launcher.IdleName(m.global, name)); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{msg: "opened base window for " + project, refresh: true}
	}
}

// startEditCmd resolves a workspace's editor preferences off the main loop and
// emits an editMsg. forcePicker (the "configure" key) always opens the picker;
// otherwise autolaunch decides. Engine calls live here, not in the key handler,
// so step()-based tests never touch a nil manager.
func (m Model) startEditCmd(project, branch string, forcePicker bool) tea.Cmd {
	return m.editCmd(project, branch, "", forcePicker)
}

// startEditPathCmd is startEditCmd for a known directory (a project base
// checkout at its root) rather than a worktree resolved by branch.
func (m Model) startEditPathCmd(project, branch, path string, forcePicker bool) tea.Cmd {
	return m.editCmd(project, branch, path, forcePicker)
}

// editCmd resolves editor preferences and emits an editMsg. When path is empty
// it is resolved from the worktree's branch; otherwise the given path is used
// directly. Either way the IDE preferences are read per project.
func (m Model) editCmd(project, branch, path string, forcePicker bool) tea.Cmd {
	mgr := m.mgr
	g := m.global
	return func() tea.Msg {
		if path == "" {
			p, err := mgr.Path(branch, project)
			if err != nil {
				return editMsg{err: err}
			}
			path = p
		}
		defaultID, autolaunch, _ := mgr.ProjectIDEPrefs(project)
		if defaultID == "" {
			defaultID = g.DefaultIDE
		}
		return editMsg{
			project:     project,
			branch:      branch,
			path:        path,
			ides:        ide.Detect(g),
			defaultID:   defaultID,
			autolaunch:  autolaunch,
			forcePicker: forcePicker,
		}
	}
}

// launchIDECmd opens dir in the chosen editor. A GUI app launches detached (it
// must not block or suspend the dashboard); a terminal editor runs via
// ExecProcess so it can take over the screen, then the ledger refreshes.
func (m Model) launchIDECmd(i ide.IDE, dir, branch string) tea.Cmd {
	cmd := ide.LaunchCmd(i, dir)
	if i.GUI {
		return func() tea.Msg {
			if err := ide.RunDetached(cmd); err != nil {
				return actionMsg{err: err}
			}
			return actionMsg{msg: "launched " + i.Name + " for " + branch}
		}
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return actionMsg{msg: "opened " + branch + " in " + i.Name, err: err, refresh: true}
	})
}

// persistDefaultCmd writes a project's default editor. When alsoAutolaunch is
// set it toggles autolaunch (off when the editor is already the autolaunching
// default, on otherwise); otherwise it leaves autolaunch unchanged.
func (m Model) persistDefaultCmd(project string, i ide.IDE, alsoAutolaunch bool) tea.Cmd {
	mgr := m.mgr
	return func() tea.Msg {
		curDefault, curAuto, _ := mgr.ProjectIDEPrefs(project)
		autolaunch := curAuto
		msg := i.Name + " is the default editor for " + project
		if alsoAutolaunch {
			// Toggle off only when this editor is already the autolaunching
			// default; otherwise turn autolaunch on.
			autolaunch = !curAuto || curDefault != i.ID
			if autolaunch {
				msg = "autolaunch on: " + i.Name + " for " + project
			} else {
				msg = "autolaunch off for " + project
			}
		}
		if err := mgr.SetProjectDefaultIDE(project, i.ID, autolaunch); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{msg: msg}
	}
}

// --- update ---

// Update is the Bubble Tea event loop.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.input.Width = m.inputFieldWidth()
		bodyH := m.bodyHeight()
		if !m.ready {
			m.vp = viewport.New(msg.Width, bodyH)
			m.ready = true
		} else {
			m.vp.Width, m.vp.Height = msg.Width, bodyH
		}
		return m, nil

	case tickMsg:
		// Re-derive live status only while idle on the ledger.
		if m.mode == modeLedger {
			return m, tea.Batch(m.refreshCmd(), tickCmd())
		}
		return m, tickCmd()

	case ledgerMsg:
		if msg.err != nil {
			m.status, m.statusErr = msg.err.Error(), true
			return m, nil
		}
		m.setRows(msg.projects)
		m.openPaths = msg.openPaths
		m.statuses = msg.statuses
		if m.status == "loading…" {
			m.status, m.statusErr = "", false
		}
		return m, nil

	case watcherReadyMsg:
		m.watcher = msg.w
		return m, m.listenStatusCmd()

	case statusChangedMsg:
		// An agent changed state: refresh now and keep listening.
		return m, tea.Batch(m.refreshCmd(), m.listenStatusCmd())

	case diffMsg:
		if msg.err != nil {
			m.status, m.statusErr = msg.err.Error(), true
			m.mode = modeLedger
			return m, nil
		}
		m.diffTitle = msg.title
		content := msg.content
		if content == "" {
			content = "(no changes against base)"
		}
		m.vp.SetContent(colorizeDiff(content))
		m.vp.GotoTop()
		m.mode = modeDiff
		return m, nil

	case actionMsg:
		if msg.err != nil && msg.errTitle != "" {
			// A suspend-and-run action (merge/add/rm/forget) failed: show the
			// engine's real message as a popup instead of "failed: exit status 1".
			m.errTitle = msg.errTitle
			m.errBody = engineError(msg.detail, msg.err)
			m.status, m.statusErr = "", false
			m.mode = modeError
		} else if msg.err != nil {
			m.status, m.statusErr = "failed: "+msg.err.Error(), true
		} else if msg.msg != "" {
			m.status, m.statusErr = msg.msg, false
		}
		if msg.refresh {
			return m, m.refreshCmd()
		}
		return m, nil

	case editMsg:
		return m.handleEditMsg(msg)

	case noticeMsg:
		return m.handleNotice(msg), nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward anything else to the active sub-component.
	if m.mode == modeDiff {
		var cmd tea.Cmd
		m.vp, cmd = m.vp.Update(msg)
		return m, cmd
	}
	if m.mode == modeInput || m.mode == modeRename {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	switch m.mode {
	case modeInput:
		return m.handleInputKey(msg)
	case modeRename:
		return m.handleRenameKey(msg)
	case modeConfirm:
		return m.handleConfirmKey(msg)
	case modeDiff:
		return m.handleDiffKey(msg)
	case modePicker:
		return m.handlePickerKey(msg)
	case modeMenu:
		return m.handleMenuKey(msg)
	case modeError:
		return m.handleErrorKey(msg)
	default:
		return m.handleLedgerKey(msg)
	}
}

// handleErrorKey dismisses the failed-action popup on any key, returning to the
// ledger and clearing the message.
func (m Model) handleErrorKey(_ tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.mode = modeLedger
	m.errTitle, m.errBody = "", ""
	return m, nil
}

// handleNotice surfaces a finished startup task's notice in the status line.
// Empty notices (nothing to report) are ignored. When several tasks report, their
// notices accumulate and are joined with " · " so a later one never clobbers an
// earlier one; the result persists like any status message until the next action
// overwrites it. Rebuilding the line from the accumulated slice means the order
// in which the concurrent tasks finish doesn't change what is shown.
func (m Model) handleNotice(msg noticeMsg) Model {
	if msg.text == "" {
		return m
	}
	m.notices = append(m.notices, msg.text)
	m.status, m.statusErr = strings.Join(m.notices, " · "), false
	return m
}

// handleEditMsg acts on resolved editor preferences: autolaunch the default
// when configured (and not forcing the picker), else open the picker overlay.
func (m Model) handleEditMsg(msg editMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.status, m.statusErr = msg.err.Error(), true
		return m, nil
	}
	m.pickerProject, m.pickerBranch, m.pickerPath = msg.project, msg.branch, msg.path
	if !msg.forcePicker && msg.autolaunch && msg.defaultID != "" {
		if i, ok := ide.Find(msg.ides, msg.defaultID); ok {
			return m, m.launchIDECmd(i, msg.path, msg.branch)
		}
	}
	m.picker = picker.New(msg.ides, msg.defaultID)
	m.mode = modePicker
	m.status, m.statusErr = "", false
	return m, nil
}

// handlePickerKey forwards a key to the picker overlay; when the picker reports
// a choice it returns to the ledger and acts on it (launch / persist default).
func (m Model) handlePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	np, _ := m.picker.Update(msg)
	m.picker = np.(picker.Model)
	if !m.picker.Done() {
		return m, nil
	}
	m.mode = modeLedger
	res := m.picker.Result()
	switch res.Action {
	case picker.Launch:
		return m, m.launchIDECmd(res.IDE, m.pickerPath, m.pickerBranch)
	case picker.SetDefault:
		return m, m.persistDefaultCmd(m.pickerProject, res.IDE, false)
	case picker.SetDefaultAutolaunch:
		return m, m.persistDefaultCmd(m.pickerProject, res.IDE, true)
	default: // picker.Cancel
		m.status, m.statusErr = "cancelled", false
		return m, nil
	}
}

func (m Model) handleLedgerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		m.moveCursor(-1)
	case "down", "j":
		m.moveCursor(1)
	case "g", "home":
		m.cursor = 0
	case "G", "end":
		m.cursor = len(m.rows) - 1
		if m.cursor < 0 {
			m.cursor = 0
		}
	case "r":
		m.status, m.statusErr = "refreshing…", false
		return m, m.refreshCmd()
	case "enter", "d":
		if r, ok := m.currentWorkspace(); ok {
			m.diffProject = r.view.Worktree.Project
			m.diffBranch = r.view.Worktree.Branch
			return m, m.diffCmd(m.diffProject, m.diffBranch)
		}
		// On the base (main) row, show the root checkout's uncommitted diff.
		if r, ok := m.currentMain(); ok {
			m.diffProject = r.project
			m.diffBranch = r.main.Branch
			return m, m.mainDiffCmd(r.project, r.main.Branch)
		}
		// On a project header, enter opens the project menu (rename/delete). The
		// diff alias "d" has nothing to show on a header, so it falls through to
		// the hint below.
		if r, ok := m.currentProjectRow(); ok && msg.String() == "enter" {
			return m.openProjectMenu(r.project)
		}
		m.status, m.statusErr = "select a workspace to view its diff", true
	case "a":
		proj := m.currentProject()
		if proj == "" {
			m.status, m.statusErr = "no project to add a workspace to (register one: wf project add)", true
			return m, nil
		}
		m.addProject = proj
		m.mode = modeInput
		m.input.Prompt = "branch: "
		m.input.Width = m.inputFieldWidth()
		m.input.SetValue("")
		m.input.Focus()
		m.status, m.statusErr = "new workspace in "+proj, false
		return m, textinput.Blink
	case "e":
		// Edit: autolaunch the project default when set, else open the picker.
		// On the base (main) row this opens the base checkout at the project root.
		if r, ok := m.currentWorkspace(); ok {
			return m, m.startEditCmd(r.view.Worktree.Project, r.view.Worktree.Branch, false)
		}
		if r, ok := m.currentMain(); ok {
			return m, m.startEditPathCmd(r.project, r.main.Branch, r.projectPath, false)
		}
	case "o":
		// Configure editor: always open the picker (set default / autolaunch).
		if r, ok := m.currentWorkspace(); ok {
			return m, m.startEditCmd(r.view.Worktree.Project, r.view.Worktree.Branch, true)
		}
		if r, ok := m.currentMain(); ok {
			return m, m.startEditPathCmd(r.project, r.main.Branch, r.projectPath, true)
		}
	case "t":
		if !m.inTmux {
			m.status, m.statusErr = "tmux not detected (run wf inside tmux)", true
			return m, nil
		}
		if r, ok := m.currentWorkspace(); ok {
			return m, m.openWindowCmd(r.view.Worktree.Project, r.view.Worktree.Branch)
		}
		// On the base (main) row, open a tmux window on the base checkout at the root.
		if r, ok := m.currentMain(); ok {
			return m, m.openMainWindowCmd(r.project, r.projectPath, r.main.Branch)
		}
	case "c":
		if r, ok := m.currentWorkspace(); ok {
			return m, m.copyCmd(r.view.Worktree.Project, r.view.Worktree.Branch)
		}
		if r, ok := m.currentMain(); ok {
			return m, m.copyPathCmd(r.projectPath, r.project)
		}
	case "m":
		if r, ok := m.currentWorkspace(); ok {
			m.confirm = confirmFor("merge", r.view)
			m.mode = modeConfirm
		} else if _, ok := m.currentMain(); ok {
			m.status, m.statusErr = "merge applies to a worktree, not the base checkout", true
		}
	case "x":
		if r, ok := m.currentWorkspace(); ok {
			m.confirm = confirmFor("rm", r.view)
			m.mode = modeConfirm
		} else if _, ok := m.currentMain(); ok {
			m.status, m.statusErr = "remove applies to a worktree, not the base checkout", true
		}
	}
	return m, nil
}

func (m Model) handleDiffKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.mode = modeLedger
		return m, nil
	case "r":
		return m, m.diffCmd(m.diffProject, m.diffBranch)
	}
	var cmd tea.Cmd
	m.vp, cmd = m.vp.Update(msg)
	return m, cmd
}

func (m Model) handleInputKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		branch := strings.TrimSpace(m.input.Value())
		m.mode = modeLedger
		m.input.Blur()
		if branch == "" {
			m.status, m.statusErr = "add cancelled (empty branch)", true
			return m, nil
		}
		m.status, m.statusErr = "creating "+branch+"…", false
		return m, m.runSelf("created "+branch, "add", branch, "--project", m.addProject)
	case tea.KeyEsc:
		m.mode = modeLedger
		m.input.Blur()
		m.status, m.statusErr = "add cancelled", false
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) handleConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		c := m.confirm
		m.mode = modeLedger
		switch c.action {
		case "merge":
			m.status, m.statusErr = "merging "+c.branch+"…", false
			return m, m.runSelf("merged "+c.branch, "merge", c.branch, "--project", c.project)
		case "rm":
			m.status, m.statusErr = "removing "+c.branch+"…", false
			return m, m.runSelf("removed "+c.branch, "rm", c.branch, "--project", c.project, "--force")
		case "deleteProject":
			m.status, m.statusErr = "deleting project "+c.project+"…", false
			// Force the registry removal when the project still has workspaces, so
			// the prompt's warning is honoured rather than the engine refusing.
			return m, m.deleteProjectCmd(c.project, c.wsCount > 0)
		}
	case "f", "F":
		// Forget is offered only from the rm prompt: unregister the workspace and
		// leave its files on disk — the way out when removal is blocked because
		// the worktree holds files we can't delete (e.g. root-owned Docker files).
		if m.confirm.action == "rm" {
			c := m.confirm
			m.mode = modeLedger
			m.status, m.statusErr = "forgetting "+c.branch+"…", false
			return m, m.runSelf("forgot "+c.branch, "forget", c.branch, "--project", c.project)
		}
	case "n", "N", "esc", "q":
		m.mode = modeLedger
		m.status, m.statusErr = "cancelled", false
	}
	return m, nil
}

// openProjectMenu pops the per-project action menu (rename/delete) over the
// ledger for the given project.
func (m Model) openProjectMenu(project string) (tea.Model, tea.Cmd) {
	m.menuProject = project
	m.menu = newProjectMenu(project)
	m.mode = modeMenu
	m.status, m.statusErr = "", false
	return m, nil
}

// handleMenuKey drives the popup action menu: arrows move between options,
// enter chooses one, esc/q dismisses it.
func (m Model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.menu.move(-1)
	case "down", "j":
		m.menu.move(1)
	case "g", "home":
		m.menu.cursor = 0
	case "G", "end":
		m.menu.move(len(m.menu.items))
	case "enter":
		return m.selectMenuItem()
	case "esc", "q":
		m.mode = modeLedger
		m.status, m.statusErr = "", false
	}
	return m, nil
}

// selectMenuItem acts on the highlighted menu option.
func (m Model) selectMenuItem() (tea.Model, tea.Cmd) {
	it, ok := m.menu.current()
	if !ok {
		m.mode = modeLedger
		return m, nil
	}
	switch it.id {
	case "rename":
		return m.startRename()
	case "delete":
		return m.startProjectDelete()
	default:
		m.mode = modeLedger
		return m, nil
	}
}

// startRename opens the rename text input pre-filled with the project's current
// name (cursor at the end) so the user can edit it in place.
func (m Model) startRename() (tea.Model, tea.Cmd) {
	m.mode = modeRename
	m.input.Prompt = "name: "
	m.input.Width = m.inputFieldWidth()
	m.input.SetValue(m.menuProject)
	m.input.CursorEnd()
	m.input.Focus()
	m.status, m.statusErr = "rename "+m.menuProject, false
	return m, textinput.Blink
}

// startProjectDelete opens the y/n confirmation for unregistering a project,
// snapshotting how many workspaces it still owns so the prompt can warn.
func (m Model) startProjectDelete() (tea.Model, tea.Cmd) {
	m.confirm = confirm{
		action:  "deleteProject",
		project: m.menuProject,
		wsCount: m.projectWorkspaceCount(m.menuProject),
	}
	m.mode = modeConfirm
	m.status, m.statusErr = "", false
	return m, nil
}

// handleRenameKey commits or cancels a project rename.
func (m Model) handleRenameKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEnter:
		newName := strings.TrimSpace(m.input.Value())
		old := m.menuProject
		m.mode = modeLedger
		m.input.Blur()
		switch newName {
		case "":
			m.status, m.statusErr = "rename cancelled (empty name)", true
			return m, nil
		case old:
			m.status, m.statusErr = "name unchanged", false
			return m, nil
		}
		m.status, m.statusErr = "renaming "+old+" → "+newName+"…", false
		return m, m.renameProjectCmd(old, newName)
	case tea.KeyEsc:
		m.mode = modeLedger
		m.input.Blur()
		m.status, m.statusErr = "rename cancelled", false
		return m, nil
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// renameProjectCmd renames a project through the engine (a registry edit, so no
// suspend is needed) and refreshes the ledger.
func (m Model) renameProjectCmd(old, newName string) tea.Cmd {
	mgr := m.mgr
	return func() tea.Msg {
		if err := mgr.RenameProject(old, newName); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{msg: "renamed " + old + " → " + newName, refresh: true}
	}
}

// deleteProjectCmd unregisters a project through the engine and refreshes the
// ledger. force drops any remaining worktree registrations (the repo on disk is
// untouched either way).
func (m Model) deleteProjectCmd(name string, force bool) tea.Cmd {
	mgr := m.mgr
	return func() tea.Msg {
		if err := mgr.RemoveProject(name, force); err != nil {
			return actionMsg{err: err}
		}
		return actionMsg{msg: "deleted project " + name, refresh: true}
	}
}

// --- row/cursor helpers ---

// setRows rebuilds the flattened ledger, preserving the selection where it can.
func (m *Model) setRows(projects []workspace.ProjectView) {
	prevKey := m.selectionKey()
	var rows []row
	for i := range projects {
		pv := projects[i]
		rows = append(rows,
			row{
				kind:        rowProject,
				project:     pv.Project.Name,
				projectPath: pv.Project.Path,
				wsCount:     len(pv.Workspaces),
				main:        pv.Main,
			},
			row{
				kind:        rowMain,
				project:     pv.Project.Name,
				projectPath: pv.Project.Path,
				main:        pv.Main,
			},
		)
		for j := range pv.Workspaces {
			v := &pv.Workspaces[j]
			rows = append(rows, row{kind: rowWorkspace, project: pv.Project.Name, view: v})
		}
	}
	m.rows = rows
	// Restore the cursor onto the same row when possible.
	if prevKey != "" {
		for i, r := range rows {
			if rowKey(r) == prevKey {
				m.cursor = i
				break
			}
		}
	}
	if m.cursor >= len(rows) {
		m.cursor = len(rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m *Model) moveCursor(delta int) {
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
}

func (m Model) current() (row, bool) {
	if m.cursor < 0 || m.cursor >= len(m.rows) {
		return row{}, false
	}
	return m.rows[m.cursor], true
}

// currentWorkspace returns the selected row when it is a workspace.
func (m Model) currentWorkspace() (row, bool) {
	r, ok := m.current()
	if ok && r.kind == rowWorkspace && r.view != nil {
		return r, true
	}
	return row{}, false
}

// currentMain returns the selected row when it is a base/main checkout — the
// target of the base-launch keys (t/e/o/enter/c).
func (m Model) currentMain() (row, bool) {
	r, ok := m.current()
	if ok && r.kind == rowMain {
		return r, true
	}
	return row{}, false
}

// currentProjectRow returns the selected row when it is a project header — the
// target of the project menu (enter).
func (m Model) currentProjectRow() (row, bool) {
	r, ok := m.current()
	if ok && r.kind == rowProject {
		return r, true
	}
	return row{}, false
}

// projectWorkspaceCount counts the worktree rows belonging to a project in the
// current ledger, so the delete prompt can say how many registrations it would
// drop.
func (m Model) projectWorkspaceCount(project string) int {
	n := 0
	for _, r := range m.rows {
		if r.kind == rowWorkspace && r.project == project {
			n++
		}
	}
	return n
}

// currentProject is the project of the selected row (header or workspace).
func (m Model) currentProject() string {
	if r, ok := m.current(); ok {
		return r.project
	}
	return ""
}

func (m Model) selectionKey() string {
	if r, ok := m.current(); ok {
		return rowKey(r)
	}
	return ""
}

func rowKey(r row) string {
	switch r.kind {
	case rowWorkspace:
		if r.view != nil {
			return "w\x00" + r.view.Worktree.Project + "\x00" + r.view.Worktree.Branch
		}
		return "w\x00" + r.project
	case rowMain:
		return "m\x00" + r.project
	default:
		return "p\x00" + r.project
	}
}
