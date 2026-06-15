package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestPromptBranch(t *testing.T) {
	cands := []string{"development", "main", "master"}

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"empty keeps default", "\n", "development"},
		{"number selects candidate", "2\n", "main"},
		{"last number", "3\n", "master"},
		{"free text is a branch name", "feature/x\n", "feature/x"},
		{"out-of-range falls back to default", "9\n", "development"},
		{"eof keeps default", "", "development"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := promptBranch(&out, strings.NewReader(tc.input), cands, "development")
			if err != nil {
				t.Fatalf("promptBranch: %v", err)
			}
			if got != tc.want {
				t.Errorf("promptBranch(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

func TestPromptBranchMarksDefault(t *testing.T) {
	var out bytes.Buffer
	if _, err := promptBranch(&out, strings.NewReader("\n"),
		[]string{"development", "master"}, "development"); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "development") || !strings.Contains(s, "(default)") {
		t.Errorf("prompt should mark the default; got:\n%s", s)
	}
	// development must be listed before master.
	if strings.Index(s, "development") > strings.Index(s, "master") {
		t.Errorf("development should be listed first; got:\n%s", s)
	}
}
