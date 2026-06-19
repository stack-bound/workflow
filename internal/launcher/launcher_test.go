package launcher

import (
	"os"
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
