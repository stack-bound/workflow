// Package ide discovers the editors and IDEs installed on this machine and
// builds the command to open a workspace folder in one. It probes a curated
// catalog of known editors (rather than scanning every installed app) and
// merges any user-defined editors from global config, so the "wf edit" picker
// and the dashboard show exactly what is present and launchable.
package ide

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/stack-bound/workflow/internal/config"
)

// IDE is a launchable editor detected on this machine.
type IDE struct {
	ID   string   // stable id, e.g. "goland", "code"
	Name string   // display name, e.g. "GoLand"
	GUI  bool     // windowed app (launch detached) vs terminal editor (run attached)
	Exec []string // argv to launch; the target directory is appended at launch time
}

// candidate is a catalog entry with the per-platform hints used to detect it.
type candidate struct {
	id   string
	name string
	gui  bool
	// bins are PATH launcher names tried first on every OS (when found, the
	// binary itself is the launch command).
	bins []string
	// desktops are .desktop file base names (without the suffix) searched on
	// Linux when no PATH launcher is found; the Exec= line becomes the command.
	desktops []string
	// macApps are .app bundle names searched under the macOS application dirs;
	// when found the app is opened via "open -na <App> --args <dir>".
	macApps []string
}

// catalog is the curated set of known editors wf probes for. GUI apps launch
// detached; terminal editors take over the terminal.
var catalog = []candidate{
	{id: "code", name: "VS Code", gui: true, bins: []string{"code"}, desktops: []string{"code", "visual-studio-code"}, macApps: []string{"Visual Studio Code.app"}},
	{id: "code-insiders", name: "VS Code Insiders", gui: true, bins: []string{"code-insiders"}, desktops: []string{"code-insiders"}, macApps: []string{"Visual Studio Code - Insiders.app"}},
	{id: "codium", name: "VSCodium", gui: true, bins: []string{"codium"}, desktops: []string{"codium", "vscodium"}, macApps: []string{"VSCodium.app"}},
	{id: "cursor", name: "Cursor", gui: true, bins: []string{"cursor"}, desktops: []string{"cursor"}, macApps: []string{"Cursor.app"}},
	{id: "zed", name: "Zed", gui: true, bins: []string{"zed", "zeditor"}, desktops: []string{"dev.zed.Zed", "zed"}, macApps: []string{"Zed.app"}},
	{id: "sublime", name: "Sublime Text", gui: true, bins: []string{"subl"}, desktops: []string{"sublime_text", "sublime-text"}, macApps: []string{"Sublime Text.app"}},
	{id: "idea", name: "IntelliJ IDEA", gui: true, bins: []string{"idea"}, desktops: []string{"jetbrains-idea", "jetbrains-idea-ce", "intellij-idea-ultimate", "intellij-idea-community"}, macApps: []string{"IntelliJ IDEA.app", "IntelliJ IDEA Community Edition.app"}},
	{id: "goland", name: "GoLand", gui: true, bins: []string{"goland"}, desktops: []string{"jetbrains-goland", "goland"}, macApps: []string{"GoLand.app"}},
	{id: "pycharm", name: "PyCharm", gui: true, bins: []string{"pycharm"}, desktops: []string{"jetbrains-pycharm", "jetbrains-pycharm-ce", "pycharm-professional", "pycharm-community"}, macApps: []string{"PyCharm.app", "PyCharm Community Edition.app"}},
	{id: "webstorm", name: "WebStorm", gui: true, bins: []string{"webstorm"}, desktops: []string{"jetbrains-webstorm", "webstorm"}, macApps: []string{"WebStorm.app"}},
	{id: "phpstorm", name: "PhpStorm", gui: true, bins: []string{"phpstorm"}, desktops: []string{"jetbrains-phpstorm", "phpstorm"}, macApps: []string{"PhpStorm.app"}},
	{id: "rubymine", name: "RubyMine", gui: true, bins: []string{"rubymine"}, desktops: []string{"jetbrains-rubymine", "rubymine"}, macApps: []string{"RubyMine.app"}},
	{id: "clion", name: "CLion", gui: true, bins: []string{"clion"}, desktops: []string{"jetbrains-clion", "clion"}, macApps: []string{"CLion.app"}},
	{id: "rider", name: "Rider", gui: true, bins: []string{"rider"}, desktops: []string{"jetbrains-rider", "rider"}, macApps: []string{"Rider.app"}},
	{id: "fleet", name: "Fleet", gui: true, bins: []string{"fleet"}, desktops: []string{"jetbrains-fleet", "fleet"}, macApps: []string{"Fleet.app"}},
	// Terminal editors: PATH-only, run attached.
	{id: "nvim", name: "Neovim", bins: []string{"nvim"}},
	{id: "vim", name: "Vim", bins: []string{"vim"}},
	{id: "emacs", name: "Emacs", bins: []string{"emacs"}},
	{id: "helix", name: "Helix", bins: []string{"hx"}},
	{id: "nano", name: "Nano", bins: []string{"nano"}},
}

// Detector probes the machine for installed editors. Its fields are injectable
// so tests can run without real editors installed.
type Detector struct {
	GOOS        string                       // defaults to runtime.GOOS
	LookPath    func(string) (string, error) // defaults to exec.LookPath
	DesktopDirs []string                     // Linux .desktop search dirs
	AppDirs     []string                     // macOS application dirs
	statFile    func(string) (os.FileInfo, error)
	openFile    func(string) (*os.File, error)
	globFiles   func(string) ([]string, error)
}

// NewDetector returns a Detector wired to the real filesystem and OS.
func NewDetector() *Detector {
	return &Detector{
		GOOS:        runtime.GOOS,
		LookPath:    exec.LookPath,
		DesktopDirs: linuxDesktopDirs(),
		AppDirs:     macAppDirs(),
		statFile:    os.Stat,
		openFile:    os.Open,
		globFiles:   filepath.Glob,
	}
}

// Detect returns the editors present on this machine: catalog entries probed
// against the system, with the user's custom IDEs (from global config) merged
// in. Custom entries override a catalog entry sharing the same id.
func Detect(g *config.Global) []IDE {
	return NewDetector().Detect(g)
}

// Detect runs the probe with this detector's (possibly injected) hooks.
func (d *Detector) Detect(g *config.Global) []IDE {
	if d.LookPath == nil {
		d.LookPath = exec.LookPath
	}
	if d.statFile == nil {
		d.statFile = os.Stat
	}
	if d.openFile == nil {
		d.openFile = os.Open
	}
	if d.globFiles == nil {
		d.globFiles = filepath.Glob
	}

	var out []IDE
	seen := map[string]bool{}
	for _, c := range catalog {
		if i, ok := d.detectCandidate(c); ok {
			out = append(out, i)
			seen[c.id] = true
		}
	}

	// Custom editors are assumed installed (the user declared them); they
	// override a catalog entry with the same id, else append.
	if g != nil {
		for _, spec := range g.IDEs {
			i, ok := specToIDE(spec)
			if !ok {
				continue
			}
			if seen[i.ID] {
				for j := range out {
					if out[j].ID == i.ID {
						out[j] = i
					}
				}
				continue
			}
			out = append(out, i)
			seen[i.ID] = true
		}
	}
	return out
}

// detectCandidate resolves one catalog entry against the machine, returning the
// launch command when present. PATH launchers win; then Linux .desktop files;
// then macOS .app bundles. A PATH launcher that is a wrapper script pointing at
// a target that no longer exists (a stale JetBrains Toolbox script after a snap
// revision bump — it starts and exits 127) is skipped so a working .desktop/.app
// launcher below can win instead.
func (d *Detector) detectCandidate(c candidate) (IDE, bool) {
	for _, bin := range c.bins {
		path, err := d.LookPath(bin)
		if err != nil {
			continue
		}
		if d.scriptTargetMissing(path) {
			continue
		}
		return IDE{ID: c.id, Name: c.name, GUI: c.gui, Exec: []string{bin}}, true
	}
	if d.GOOS == "linux" {
		if argv, ok := d.findDesktopExec(c.desktops); ok {
			return IDE{ID: c.id, Name: c.name, GUI: c.gui, Exec: argv}, true
		}
	}
	if d.GOOS == "darwin" {
		if app, ok := d.findMacApp(c.macApps); ok {
			return IDE{ID: c.id, Name: c.name, GUI: c.gui, Exec: []string{"open", "-na", app, "--args"}}, true
		}
	}
	return IDE{}, false
}

// scriptTargetMissing reports whether path is a wrapper *script* (it starts with
// a shebang) whose first absolute-path command no longer exists — the signature
// of a stale JetBrains Toolbox launcher after a snap revision bump, which starts
// and then exits 127. Binaries, PATH-based wrappers (no absolute target), and
// unreadable files are treated as usable (false), so detection only ever skips a
// launcher it is confident is broken.
func (d *Detector) scriptTargetMissing(path string) bool {
	f, err := d.openFile(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(io.LimitReader(f, 4096))
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if first {
			first = false
			if !strings.HasPrefix(line, "#!") {
				return false // not a shell wrapper script
			}
			continue
		}
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		target := execLineTarget(line)
		if target == "" {
			continue // not an absolute-path command line; keep looking
		}
		_, err := d.statFile(target)
		return err != nil // dead iff the wrapper's target is missing
	}
	return false
}

// execLineTarget returns the absolute path a wrapper line runs, or "". It matches
// the shapes a generated launcher uses — `"/abs/path" "$@"` and `exec /abs/path
// …` — and deliberately ignores anything else (PATH commands, cd/export lines)
// so a working script is never mistaken for a dead one.
func execLineTarget(line string) string {
	line = strings.TrimSpace(strings.TrimPrefix(line, "exec "))
	if strings.HasPrefix(line, `"/`) {
		if end := strings.IndexByte(line[1:], '"'); end >= 0 {
			return line[1 : 1+end]
		}
		return ""
	}
	if strings.HasPrefix(line, "/") {
		if i := strings.IndexAny(line, " \t"); i >= 0 {
			return line[:i]
		}
		return line
	}
	return ""
}

// findDesktopExec looks for a .desktop launcher for any of names in the search
// dirs and returns its parsed Exec= command (field codes stripped). It tries the
// plain "<name>.desktop" first, then snap's "<snap>_<app>.desktop" convention —
// the Ubuntu menu entry for a snap, e.g.
// intellij-idea-ultimate_intellij-idea-ultimate.desktop, whose Exec is the
// reliable "/snap/bin/<snap>" wrapper.
func (d *Detector) findDesktopExec(names []string) ([]string, bool) {
	for _, dir := range d.DesktopDirs {
		for _, name := range names {
			path := filepath.Join(dir, name+".desktop")
			if _, err := d.statFile(path); err == nil {
				if argv, ok := d.parseDesktopExec(path); ok {
					return argv, true
				}
			}
			for _, snapPath := range d.snapDesktops(dir, name) {
				if argv, ok := d.parseDesktopExec(snapPath); ok {
					return argv, true
				}
			}
		}
	}
	return nil, false
}

// snapDesktops returns the snap-style "<name>_*.desktop" menu entries in dir;
// snap names its desktop files "<snap>_<app>.desktop" rather than the bare app
// name, so a plain lookup misses them.
func (d *Detector) snapDesktops(dir, name string) []string {
	matches, err := d.globFiles(filepath.Join(dir, name+"_*.desktop"))
	if err != nil {
		return nil
	}
	return matches
}

// parseDesktopExec reads the Exec= line from a .desktop file, dropping the
// freedesktop field codes (%U, %F, …) that have no meaning for opening a folder.
func (d *Detector) parseDesktopExec(path string) ([]string, bool) {
	f, err := d.openFile(path)
	if err != nil {
		return nil, false
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "Exec=") {
			continue
		}
		fields := strings.Fields(strings.TrimPrefix(line, "Exec="))
		var argv []string
		for _, fld := range fields {
			if strings.HasPrefix(fld, "%") { // field code: %U %F %f %u %i %c %k …
				continue
			}
			argv = append(argv, fld)
		}
		if len(argv) > 0 {
			return argv, true
		}
	}
	return nil, false
}

// findMacApp returns the first of apps that exists under the application dirs.
func (d *Detector) findMacApp(apps []string) (string, bool) {
	for _, dir := range d.AppDirs {
		for _, app := range apps {
			if _, err := d.statFile(filepath.Join(dir, app)); err == nil {
				return app, true
			}
		}
	}
	return "", false
}

// specToIDE converts a user-defined editor spec into an IDE, splitting its
// command line into argv. A spec without an id or command is skipped.
func specToIDE(spec config.IDESpec) (IDE, bool) {
	argv := strings.Fields(spec.Cmd)
	if spec.ID == "" || len(argv) == 0 {
		return IDE{}, false
	}
	name := spec.Name
	if name == "" {
		name = spec.ID
	}
	return IDE{ID: spec.ID, Name: name, GUI: spec.GUI, Exec: argv}, true
}

// Find returns the detected IDE with the given id.
func Find(ides []IDE, id string) (IDE, bool) {
	for _, i := range ides {
		if i.ID == id {
			return i, true
		}
	}
	return IDE{}, false
}

// LaunchCmd builds (without running) the command to open dir in the editor. For
// macOS "open -na <App> --args" form, dir is the argument passed to the app.
func LaunchCmd(i IDE, dir string) *exec.Cmd {
	args := append(append([]string{}, i.Exec[1:]...), dir)
	return exec.Command(i.Exec[0], args...)
}

// launchGrace is how long RunDetached watches a freshly started GUI editor
// before declaring it launched. A launcher that is going to fail outright (a
// stale wrapper script, a binary that no longer exists) exits within this
// window; one that has really opened a window — or handed off to a running
// instance — is either still alive or has exited cleanly by then. Kept as a var
// so tests can shorten it.
var launchGrace = 300 * time.Millisecond

// RunDetached starts a GUI editor without waiting for it to finish, so the
// caller (the CLI or the dashboard) returns immediately. Its stdin/stdout are
// detached from the terminal so the app cannot scribble over it. It then watches
// the process for launchGrace: a launcher that exits non-zero in that window
// never really opened (e.g. a stale JetBrains Toolbox script pointing at a snap
// revision that no longer exists exits 127), so its error — with any captured
// stderr — is returned instead of a false "launched". A process still running
// after the window, or one that exits 0 immediately (it handed off to a running
// instance), counts as a successful launch.
func RunDetached(cmd *exec.Cmd) error {
	cmd.Stdin, cmd.Stdout = nil, nil
	// Capture stderr to a scratch file so a quick failure can be explained,
	// without pinning an in-memory buffer open for the whole life of an editor
	// that keeps running.
	errLog, _ := os.CreateTemp("", "wf-launch-*.log")
	if errLog != nil {
		cmd.Stderr = errLog
	} else {
		cmd.Stderr = nil
	}

	if err := cmd.Start(); err != nil {
		closeAndRemove(errLog)
		return err
	}

	// Reap the child in the background so it never lingers as a zombie; the
	// channel lets us notice an immediate exit within the grace window.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			detail := readErrLog(errLog)
			closeAndRemove(errLog)
			if detail != "" {
				return fmt.Errorf("%w: %s", err, detail)
			}
			return err
		}
		closeAndRemove(errLog)
		return nil
	case <-time.After(launchGrace):
		// Still running: a real editor window. Unlink the scratch log now — the
		// process keeps writing to its inherited fd and POSIX frees the inode on
		// exit — and leave the goroutine above to reap the child when it closes.
		if errLog != nil {
			_ = os.Remove(errLog.Name())
			_ = errLog.Close()
		}
		return nil
	}
}

// readErrLog returns the trimmed tail of a launcher's captured stderr, or "".
func readErrLog(f *os.File) string {
	if f == nil {
		return ""
	}
	b, err := os.ReadFile(f.Name())
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// closeAndRemove closes and deletes a scratch file, tolerating a nil file.
func closeAndRemove(f *os.File) {
	if f == nil {
		return
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
}

// linuxDesktopDirs returns the standard freedesktop application directories,
// including snap and flatpak exports, honoring XDG_DATA_HOME / XDG_DATA_DIRS.
func linuxDesktopDirs() []string {
	var dirs []string
	home, _ := os.UserHomeDir()
	dataHome := os.Getenv("XDG_DATA_HOME")
	if dataHome == "" && home != "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	if dataHome != "" {
		dirs = append(dirs, filepath.Join(dataHome, "applications"))
		dirs = append(dirs, filepath.Join(dataHome, "flatpak", "exports", "share", "applications"))
	}
	dirs = append(dirs,
		"/usr/share/applications",
		"/usr/local/share/applications",
		"/var/lib/snapd/desktop/applications",
		"/var/lib/flatpak/exports/share/applications",
	)
	return dirs
}

// macAppDirs returns the standard macOS application directories.
func macAppDirs() []string {
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, "Applications"))
	}
	return append(dirs, "/Applications")
}
