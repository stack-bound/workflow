package cli

import (
	"bytes"
	"io"
	"path/filepath"
	"testing"
)

func TestGenCompletion(t *testing.T) {
	root := newRootCmd()
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		var buf bytes.Buffer
		if err := genCompletion(root, sh, &buf); err != nil {
			t.Errorf("genCompletion(%s): %v", sh, err)
		}
		if buf.Len() == 0 {
			t.Errorf("genCompletion(%s) produced no output", sh)
		}
	}
	if err := genCompletion(root, "tcsh", io.Discard); err == nil {
		t.Error("expected error for unsupported shell")
	}
}

func TestDetectShell(t *testing.T) {
	cases := map[string]string{
		"/bin/bash":           "bash",
		"/usr/bin/zsh":        "zsh",
		"/usr/local/bin/fish": "fish",
		"/bin/tcsh":           "",
		"":                    "",
	}
	for in, want := range cases {
		t.Setenv("SHELL", in)
		if got := detectShell(); got != want {
			t.Errorf("detectShell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestCompletionInstallPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("XDG_CONFIG_HOME", "")

	bash, post, err := completionInstallPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "share", "bash-completion", "completions", "wf"); bash != want {
		t.Errorf("bash dest = %q, want %q", bash, want)
	}
	if post == "" {
		t.Error("bash post-install hint is empty")
	}

	zsh, _, err := completionInstallPath("zsh")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "share", "zsh", "site-functions", "_wf"); zsh != want {
		t.Errorf("zsh dest = %q, want %q", zsh, want)
	}

	fish, _, err := completionInstallPath("fish")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".config", "fish", "completions", "wf.fish"); fish != want {
		t.Errorf("fish dest = %q, want %q", fish, want)
	}

	if _, _, err := completionInstallPath("powershell"); err == nil {
		t.Error("expected error: install does not support powershell")
	}
}
