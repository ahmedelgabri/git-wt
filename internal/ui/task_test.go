package ui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTaskModelSuccess(t *testing.T) {
	m := newTaskModel(TaskConfig{Message: "Loading"}, func(context.Context, io.Writer) error {
		return nil
	})
	updated, _ := m.Update(taskFinishedMsg{err: nil})
	result := updated.(*taskModel)
	if result.err != nil {
		t.Fatalf("taskModel err = %v, want nil", result.err)
	}
	if result.phase != AsyncReady {
		t.Fatalf("taskModel phase = %v, want %v", result.phase, AsyncReady)
	}
}

func TestTaskModelFailure(t *testing.T) {
	testErr := errors.New("failed")
	m := newTaskModel(TaskConfig{Message: "Loading"}, func(context.Context, io.Writer) error {
		return testErr
	})
	updated, _ := m.Update(taskFinishedMsg{err: testErr})
	result := updated.(*taskModel)
	if !errors.Is(result.err, testErr) {
		t.Fatalf("taskModel err = %v, want %v", result.err, testErr)
	}
	if result.phase != AsyncError {
		t.Fatalf("taskModel phase = %v, want %v", result.phase, AsyncError)
	}
}

func TestTaskModelCancellation(t *testing.T) {
	m := newTaskModel(TaskConfig{Message: "Loading"}, func(context.Context, io.Writer) error {
		return nil
	})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	result := updated.(*taskModel)
	if !errors.Is(result.err, context.Canceled) {
		t.Fatalf("taskModel err = %v, want context.Canceled", result.err)
	}
	if result.phase != AsyncCanceled {
		t.Fatalf("taskModel phase = %v, want %v", result.phase, AsyncCanceled)
	}
}

func TestTaskModelStreamedOutput(t *testing.T) {
	m := newTaskModel(TaskConfig{Message: "Loading", ShowOutput: true}, func(context.Context, io.Writer) error {
		return nil
	})
	updated, _ := m.Update(taskLogMsg{text: "line one\nline two\n"})
	result := updated.(*taskModel)
	if result.phase != AsyncPartial {
		t.Fatalf("taskModel phase = %v, want %v", result.phase, AsyncPartial)
	}
	view := result.View()
	if !strings.Contains(view, "line one") || !strings.Contains(view, "line two") {
		t.Fatalf("taskModel view missing streamed output: %q", view)
	}
	if strings.Contains(view, "line two\n\n\n") {
		t.Fatalf("taskModel short output should not be padded with viewport blanks: %q", view)
	}
}

func TestTaskModelFullOutputShowsAllLines(t *testing.T) {
	m := newTaskModel(TaskConfig{Message: "Loading", ShowOutput: true, FullOutput: true}, func(context.Context, io.Writer) error {
		return nil
	})

	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("line %02d", i))
	}
	updated, _ := m.Update(taskLogMsg{text: strings.Join(lines, "\n") + "\n"})
	result := updated.(*taskModel)
	view := result.View()
	if !strings.Contains(view, "line 00") || !strings.Contains(view, "line 19") {
		t.Fatalf("taskModel full output should contain first and last lines: %q", view)
	}
}

func TestTaskProgramSuccess(t *testing.T) {
	m := newTaskModel(TaskConfig{Message: "Loading"}, func(context.Context, io.Writer) error {
		return nil
	})
	result := runModel(t, m, 5*time.Second)
	r := result.(*taskModel)
	if r.phase != AsyncReady {
		t.Fatalf("taskModel phase = %v, want %v", r.phase, AsyncReady)
	}
	if r.err != nil {
		t.Fatalf("taskModel err = %v, want nil", r.err)
	}
}

func TestTaskProgramFailure(t *testing.T) {
	testErr := errors.New("task failed")
	m := newTaskModel(TaskConfig{Message: "Loading"}, func(context.Context, io.Writer) error {
		return testErr
	})
	result := runModel(t, m, 5*time.Second)
	r := result.(*taskModel)
	if r.phase != AsyncError {
		t.Fatalf("taskModel phase = %v, want %v", r.phase, AsyncError)
	}
	if !errors.Is(r.err, testErr) {
		t.Fatalf("taskModel err = %v, want %v", r.err, testErr)
	}
}

func TestRunTaskFallback(t *testing.T) {
	cleanup := mockStdin("")
	defer cleanup()

	called := false
	err := RunTask(TaskConfig{Message: "fallback task", ShowOutput: true}, func(_ context.Context, w io.Writer) error {
		called = true
		fmt.Fprintln(w, "hello from task")
		return nil
	})
	if err != nil {
		t.Fatalf("RunTask() = %v, want nil", err)
	}
	if !called {
		t.Fatal("RunTask() fallback should call the task function")
	}
}
