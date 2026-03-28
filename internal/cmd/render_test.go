package cmd

import (
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/ui"
)

func TestRenderTableSectionWithFooter(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := renderTableSectionWithFooter(
		[]ui.TableColumn{{Title: "COL1"}, {Title: "COL2"}},
		[][]string{{"a", "b"}},
		[]string{"note line"},
		"summary line",
		"footer line",
	)
	for _, want := range []string{"COL1", "COL2", "a", "b", "note line", "summary line", "footer line"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderTableSectionWithFooter() missing %q in %q", want, got)
		}
	}
}

func TestRenderCommandHintsSection(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := renderCommandHintsSection([]commandHint{{
		Action:  "Create another worktree",
		Command: "cd repo && git wt add very-long-branch-name-with-extra-text",
	}})
	if !strings.Contains(got, "Create another worktree") {
		t.Fatalf("renderCommandHintsSection() missing action: %q", got)
	}
	if !strings.Contains(got, "git wt add") {
		t.Fatalf("renderCommandHintsSection() missing command: %q", got)
	}
}

func TestRenderRepoLayoutSection(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := renderRepoLayoutSection(".", []treeBranch{{Name: "main", Desc: "default worktree"}})
	for _, want := range []string{".bare", ".git", "main", "default worktree", "1 worktree(s) ready"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderRepoLayoutSection() missing %q in %q", want, got)
		}
	}
}
