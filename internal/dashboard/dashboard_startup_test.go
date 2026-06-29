package dashboard

import "testing"

// A finished startup task's notice shows in the status line as a (non-error)
// message — the way the hook auto-update notice surfaces today.
func TestNoticeMsgShowsInStatus(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, noticeMsg{text: "updated agent-status hooks: Stop, SessionEnd"})
	if m.status != "updated agent-status hooks: Stop, SessionEnd" || m.statusErr {
		t.Fatalf("status = %q err=%v, want the notice as a non-error message", m.status, m.statusErr)
	}
}

// A task with nothing to report (an empty notice) leaves the status line as it
// was — it neither shows a blank message nor clears an earlier one.
func TestEmptyNoticeIgnored(t *testing.T) {
	m := New(nil, nil) // status starts at the "loading…" sentinel
	m, _ = step(m, noticeMsg{text: ""})
	if m.status != "loading…" {
		t.Errorf("empty notice changed status to %q, want it untouched", m.status)
	}
}

// Several tasks reporting must not clobber each other: their notices accumulate,
// joined, regardless of the order the concurrent tasks finish in.
func TestNoticesAccumulate(t *testing.T) {
	m := New(nil, nil)
	m, _ = step(m, noticeMsg{text: "first"})
	m, _ = step(m, noticeMsg{text: "second"})
	if m.status != "first · second" {
		t.Errorf("status = %q, want %q (notices appended, not clobbered)", m.status, "first · second")
	}
	// An empty notice between two real ones changes nothing.
	m, _ = step(m, noticeMsg{text: ""})
	if m.status != "first · second" {
		t.Errorf("empty notice altered accumulated status: %q", m.status)
	}
}

// The notice must survive the first ledger load, which clears only the
// "loading…" sentinel — in either arrival order (the async tasks and the ledger
// refresh race).
func TestNoticeSurvivesLedgerLoad(t *testing.T) {
	// Notice first, then the ledger load clears "loading…" but must keep the notice.
	m := New(nil, nil)
	m, _ = step(m, noticeMsg{text: "hooks updated"})
	m, _ = step(m, ledgerMsg{projects: sampleLedger()})
	if m.status != "hooks updated" {
		t.Errorf("notice lost after ledger load: status = %q", m.status)
	}

	// Other order: the ledger load lands first (status → ""), then the notice.
	m2 := New(nil, nil)
	m2, _ = step(m2, ledgerMsg{projects: sampleLedger()})
	m2, _ = step(m2, noticeMsg{text: "hooks updated"})
	if m2.status != "hooks updated" {
		t.Errorf("notice not shown when it lands after the ledger load: status = %q", m2.status)
	}
}

// Startup tasks are wired as commands, not run up front: New/startupCmds only
// build closures, and the task runs (and yields a noticeMsg) when its command
// executes — proving nothing is computed synchronously before launch.
func TestStartupTasksRunAsCommands(t *testing.T) {
	ran := false
	task := func() string { ran = true; return "did a thing" }

	m := New(nil, nil, task)
	cmds := m.startupCmds()
	if len(cmds) != 1 {
		t.Fatalf("startupCmds = %d, want 1", len(cmds))
	}
	if ran {
		t.Error("task ran while wiring the command; it must only run when the command executes")
	}

	msg := cmds[0]()
	if !ran {
		t.Error("task did not run when its command executed")
	}
	nm, ok := msg.(noticeMsg)
	if !ok || nm.text != "did a thing" {
		t.Fatalf("command msg = %#v, want noticeMsg{text:\"did a thing\"}", msg)
	}

	// Feeding the result back through Update surfaces it.
	m, _ = step(m, nm)
	if m.status != "did a thing" {
		t.Errorf("status after the task's notice = %q", m.status)
	}
}

// Init must batch the startup-task commands alongside the base ones, and a model
// with no tasks still returns its base commands.
func TestInitIncludesStartupTasks(t *testing.T) {
	if New(nil, nil, func() string { return "x" }).Init() == nil {
		t.Error("Init with a startup task returned nil")
	}
	if New(nil, nil).Init() == nil {
		t.Error("Init with no startup tasks returned nil")
	}
}
