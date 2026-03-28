package cmd

import (
	"context"
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/picker"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func TestBranchCandidateDesc(t *testing.T) {
	got := branchCandidateDesc("2d", "Alice", "Fix flaky CI")
	if got != "2d · Alice · Fix flaky CI" {
		t.Fatalf("branchCandidateDesc() = %q, want %q", got, "2d · Alice · Fix flaky CI")
	}

	got = branchCandidateDesc("", "Alice", "")
	if got != "Alice" {
		t.Fatalf("branchCandidateDesc() with sparse data = %q, want %q", got, "Alice")
	}
}

func TestParseRemoteBranchCandidates(t *testing.T) {
	output := strings.Join([]string{
		"origin/HEAD\t1710000000\tAlice\tHead ref",
		"origin/main\t1710000000\tAlice\tInitial commit",
		"origin/feature/login\t1710003600\tBob\tFix login",
		"upstream/feature/api\t1710007200\tCarol\tAdd API endpoint",
	}, "\n")

	got := parseRemoteBranchCandidates(output, map[string]bool{"main": true})
	if len(got) != 2 {
		t.Fatalf("parseRemoteBranchCandidates() returned %d candidates, want 2", len(got))
	}

	if got[0].RemoteRef != "origin/feature/login" {
		t.Fatalf("candidate[0].RemoteRef = %q, want %q", got[0].RemoteRef, "origin/feature/login")
	}
	if got[0].Branch != "feature/login" {
		t.Fatalf("candidate[0].Branch = %q, want %q", got[0].Branch, "feature/login")
	}
	if got[0].Author != "Bob" {
		t.Fatalf("candidate[0].Author = %q, want %q", got[0].Author, "Bob")
	}
	if got[0].Subject != "Fix login" {
		t.Fatalf("candidate[0].Subject = %q, want %q", got[0].Subject, "Fix login")
	}
	if got[0].Age == "" || got[0].Age == "n/a" {
		t.Fatalf("candidate[0].Age = %q, want non-empty relative age", got[0].Age)
	}

	if got[1].Remote != "upstream" {
		t.Fatalf("candidate[1].Remote = %q, want %q", got[1].Remote, "upstream")
	}
}

func TestRemoteBranchCandidatePickerItem(t *testing.T) {
	candidate := remoteBranchCandidate{
		RemoteRef: "origin/feature/login",
		Remote:    "origin",
		Branch:    "feature/login",
		Age:       "2d",
		Author:    "Bob",
		Subject:   "Fix login",
	}

	item := candidate.pickerItem()
	if item.Label != "feature/login [origin]" {
		t.Fatalf("picker label = %q, want %q", item.Label, "feature/login [origin]")
	}
	if item.Value != "origin/feature/login" {
		t.Fatalf("picker value = %q, want %q", item.Value, "origin/feature/login")
	}
	if item.Desc != "2d · Bob · Fix login" {
		t.Fatalf("picker desc = %q, want %q", item.Desc, "2d · Bob · Fix login")
	}
}

func TestAddPreloadModelStatusAndDone(t *testing.T) {
	m := newAddPreloadModel(context.Background())

	updated, _ := m.Update(addPreloadStatusMsg{phase: ui.AsyncPartial, message: "Loading remote branches…"})
	result := updated.(*addPreloadModel)
	if result.message != "Loading remote branches…" {
		t.Fatalf("message = %q, want %q", result.message, "Loading remote branches…")
	}

	updated, _ = result.Update(addPreloadDoneMsg{items: []picker.Item{{Label: "foo", Value: "bar"}}})
	result = updated.(*addPreloadModel)
	if result.phase != ui.AsyncReady {
		t.Fatalf("phase = %v, want %v", result.phase, ui.AsyncReady)
	}
	if len(result.items) != 1 || result.items[0].Label != "foo" {
		t.Fatalf("items = %+v, want one ready item", result.items)
	}
}

func TestAddPreloadModelCancel(t *testing.T) {
	m := newAddPreloadModel(context.Background())
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := updated.(*addPreloadModel)
	if result.err != context.Canceled {
		t.Fatalf("cancel err = %v, want %v", result.err, context.Canceled)
	}
	if result.phase != ui.AsyncCanceled {
		t.Fatalf("phase = %v, want %v", result.phase, ui.AsyncCanceled)
	}
}
