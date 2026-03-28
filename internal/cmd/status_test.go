package cmd

import (
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func TestParseBranchStatus(t *testing.T) {
	input := `# branch.oid abc1234
# branch.head main
# branch.upstream origin/main
# branch.ab +2 -1
1 .M N... 100644 100644 100644 abc123 abc123 file.txt`

	upstream, ahead, behind, dirty := parseBranchStatus(input)
	if upstream != "origin/main" {
		t.Fatalf("upstream = %q, want %q", upstream, "origin/main")
	}
	if ahead != 2 || behind != 1 {
		t.Fatalf("ahead/behind = %d/%d, want 2/1", ahead, behind)
	}
	if !dirty {
		t.Fatal("dirty should be true")
	}
}

func TestParseBranchStatusCleanLocal(t *testing.T) {
	input := `# branch.oid abc1234
# branch.head detached`

	upstream, ahead, behind, dirty := parseBranchStatus(input)
	if upstream != "" {
		t.Fatalf("upstream = %q, want empty", upstream)
	}
	if ahead != 0 || behind != 0 {
		t.Fatalf("ahead/behind = %d/%d, want 0/0", ahead, behind)
	}
	if dirty {
		t.Fatal("dirty should be false")
	}
}

func TestStatusModelProgressAndCompletion(t *testing.T) {
	m := statusModel{
		rows:    []statusRow{{workspace: "main", branch: "main", path: "./main", flags: ui.Subtle("—")}},
		pending: 1,
		phase:   ui.AsyncLoading,
	}

	if view := m.View(); !strings.Contains(view, "loading 0/1") {
		t.Fatalf("loading view = %q, want progress line", view)
	}

	updated, _ := m.Update(statusResultMsg{index: 0, dirty: true, lastCommit: "1h"})
	result := updated.(statusModel)
	if result.phase != ui.AsyncReady {
		t.Fatalf("phase = %v, want %v", result.phase, ui.AsyncReady)
	}
	if result.pending != 0 {
		t.Fatalf("pending = %d, want 0", result.pending)
	}
	if !result.rows[0].loaded || !result.rows[0].dirty {
		t.Fatalf("row not updated correctly: %+v", result.rows[0])
	}
	if view := result.View(); !strings.Contains(view, "1 dirty") {
		t.Fatalf("ready view = %q, want dirty summary", view)
	}
}

func TestStatusModelCtrlC(t *testing.T) {
	m := statusModel{phase: ui.AsyncLoading}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := updated.(statusModel)
	if result.phase != ui.AsyncCanceled {
		t.Fatalf("phase = %v, want %v", result.phase, ui.AsyncCanceled)
	}
	if result.err == nil || result.err.Error() != "interrupted" {
		t.Fatalf("err = %v, want interrupted", result.err)
	}
}
