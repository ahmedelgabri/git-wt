package cmd

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func TestRenderDoctorChecksWithFooter(t *testing.T) {
	got := renderDoctorChecksWithFooter([]doctorCheck{{Level: doctorOK, Name: "Repository", Detail: "/tmp/repo"}}, "loading footer")
	if !strings.Contains(got, "Repository") {
		t.Fatalf("renderDoctorChecksWithFooter() missing check content: %q", got)
	}
	if !strings.Contains(got, "loading footer") {
		t.Fatalf("renderDoctorChecksWithFooter() missing footer: %q", got)
	}
}

func TestWalkDoctorChecksEmitsRepository(t *testing.T) {
	repo := initGitRepo(t)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(repo); err != nil {
		t.Fatal(err)
	}

	var checks []doctorCheck
	hasErrors := walkDoctorChecks(context.Background(), repo, func(check doctorCheck) {
		checks = append(checks, check)
	})
	if len(checks) == 0 {
		t.Fatal("walkDoctorChecks() should emit at least one check")
	}
	if checks[0].Name != "Repository" {
		t.Fatalf("first check = %q, want %q", checks[0].Name, "Repository")
	}
	if !strings.Contains(checks[0].Detail, repo) {
		t.Fatalf("repository detail = %q, want to contain %q", checks[0].Detail, repo)
	}
	if hasErrors {
		t.Fatalf("walkDoctorChecks() on a healthy repo should not report errors: %+v", checks)
	}
}

func TestDoctorModelLifecycle(t *testing.T) {
	m := newDoctorModel("/tmp/repo")
	updated, _ := m.Update(doctorCheckMsg{check: doctorCheck{Level: doctorWarn, Name: "Repository layout", Detail: "standard git layout (.git directory)"}})
	result := updated.(*doctorModel)
	if result.phase != ui.AsyncPartial {
		t.Fatalf("phase = %v, want %v", result.phase, ui.AsyncPartial)
	}
	if len(result.checks) != 1 {
		t.Fatalf("checks = %d, want 1", len(result.checks))
	}

	updated, _ = result.Update(doctorDoneMsg{hasErrors: false})
	result = updated.(*doctorModel)
	if result.phase != ui.AsyncReady {
		t.Fatalf("phase = %v, want %v", result.phase, ui.AsyncReady)
	}
}

func TestDoctorModelCancel(t *testing.T) {
	m := newDoctorModel("/tmp/repo")
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := updated.(*doctorModel)
	if result.phase != ui.AsyncCanceled {
		t.Fatalf("phase = %v, want %v", result.phase, ui.AsyncCanceled)
	}
	if result.err != context.Canceled {
		t.Fatalf("err = %v, want %v", result.err, context.Canceled)
	}
}
