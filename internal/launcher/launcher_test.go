package launcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stack-bound/workflow/internal/config"
)

func TestCopyPathWithClipboardCmd(t *testing.T) {
	out := filepath.Join(t.TempDir(), "clip.txt")
	u := NewUniversal(&config.Global{ClipboardCmd: "cat > " + out})
	if err := u.CopyPath("/some/work/path"); err != nil {
		t.Fatalf("CopyPath: %v", err)
	}
	got, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "/some/work/path" {
		t.Errorf("clipboard received %q, want %q", got, "/some/work/path")
	}
}

func TestCopyPathClipboardCmdError(t *testing.T) {
	u := NewUniversal(&config.Global{ClipboardCmd: "exit 3"})
	if err := u.CopyPath("/x"); err == nil {
		t.Error("expected error when clipboard command fails")
	}
}

func TestEditorCommandParsesArgs(t *testing.T) {
	u := NewUniversal(&config.Global{Editor: "code -n --wait"})
	cmd := u.EditorCommand("/work/tree")
	want := []string{"code", "-n", "--wait", "/work/tree"}
	if len(cmd.Args) != len(want) {
		t.Fatalf("args = %v, want %v", cmd.Args, want)
	}
	for i := range want {
		if cmd.Args[i] != want[i] {
			t.Errorf("arg %d = %q, want %q", i, cmd.Args[i], want[i])
		}
	}
}

func TestOpenInEditorSuccess(t *testing.T) {
	if _, err := exec.LookPath("true"); err != nil {
		t.Skip("no 'true' binary available")
	}
	u := NewUniversal(&config.Global{Editor: "true"})
	if err := u.OpenInEditor("/anything"); err != nil {
		t.Errorf("OpenInEditor with 'true' editor: %v", err)
	}
}

func TestOpenInEditorError(t *testing.T) {
	u := NewUniversal(&config.Global{Editor: "wf-no-such-editor-xyz"})
	if err := u.OpenInEditor("/anything"); err == nil {
		t.Error("expected error for a non-existent editor")
	}
}
