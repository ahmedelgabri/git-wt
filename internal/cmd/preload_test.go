package cmd

import (
	"context"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/ui"
	tea "github.com/charmbracelet/bubbletea"
)

func TestPreloadModelStatusAndDone(t *testing.T) {
	m := newPreloadModel(context.Background(), "Loading…", func(context.Context, func(ui.AsyncPhase, string)) (int, error) {
		return 0, nil
	})

	updated, _ := m.Update(preloadStatusMsg{phase: ui.AsyncPartial, message: "Scanning…"})
	result := updated.(*preloadModel[int])
	if result.phase != ui.AsyncPartial {
		t.Fatalf("phase = %v, want %v", result.phase, ui.AsyncPartial)
	}
	if result.message != "Scanning…" {
		t.Fatalf("message = %q, want %q", result.message, "Scanning…")
	}

	updated, _ = result.Update(preloadDoneMsg[int]{value: 42})
	result = updated.(*preloadModel[int])
	if result.phase != ui.AsyncReady {
		t.Fatalf("phase = %v, want %v", result.phase, ui.AsyncReady)
	}
	if result.value != 42 {
		t.Fatalf("value = %d, want %d", result.value, 42)
	}
}

func TestPreloadModelCancel(t *testing.T) {
	m := newPreloadModel(context.Background(), "Loading…", func(context.Context, func(ui.AsyncPhase, string)) (int, error) {
		return 0, nil
	})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := updated.(*preloadModel[int])
	if result.err != context.Canceled {
		t.Fatalf("cancel err = %v, want %v", result.err, context.Canceled)
	}
	if result.phase != ui.AsyncCanceled {
		t.Fatalf("phase = %v, want %v", result.phase, ui.AsyncCanceled)
	}
}

func TestRunPreloadFallback(t *testing.T) {
	called := false
	value, err := runPreload(context.Background(), "Loading…", func(context.Context, func(ui.AsyncPhase, string)) (int, error) {
		called = true
		return 7, nil
	})
	if err != nil {
		t.Fatalf("runPreload() = %v, want nil", err)
	}
	if !called {
		t.Fatal("runPreload() should call the load function")
	}
	if value != 7 {
		t.Fatalf("value = %d, want %d", value, 7)
	}
}
