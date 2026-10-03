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

func TestRunTaskRawPreservesOutput(t *testing.T) {
	var buf strings.Builder
	err := runTaskRaw(TaskConfig{Message: "Fetching", ShowOutput: true, RawOutput: true}, func(_ context.Context, w io.Writer) error {
		_, _ = io.WriteString(w, "Counting objects: 10%\rCounting objects: 20%\r")
		return nil
	}, &buf)
	if err != nil {
		t.Fatalf("runTaskRaw() = %v, want nil", err)
	}
	got := buf.String()
	if !strings.Contains(got, "Counting objects: 10%\rCounting objects: 20%\r") {
		t.Fatalf("runTaskRaw() should preserve raw carriage returns, got %q", got)
	}
}

func TestNormalizeTaskLogChunk(t *testing.T) {
	got := normalizeTaskLogChunk("Counting objects: 10%\rCounting objects: 20%\r\nDone\n")
	want := "Counting objects: 10%\nCounting objects: 20%\nDone\n"
	if got != want {
		t.Fatalf("normalizeTaskLogChunk() = %q, want %q", got, want)
	}
}

func runTestTaskProgram(t *testing.T, m *taskModel) (*tea.Program, <-chan error) {
	t.Helper()
	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(io.Discard))
	result := make(chan error, 1)
	go func() { result <- runTaskProgram(m, p) }()
	return p, result
}

func waitTaskProgram(t *testing.T, result <-chan error) error {
	t.Helper()
	select {
	case err := <-result:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("task program hung")
		return nil
	}
}

func TestTaskProgramSuccess(t *testing.T) {
	m := newTaskModel(TaskConfig{Message: "Loading"}, func(context.Context, io.Writer) error {
		return nil
	})
	_, result := runTestTaskProgram(t, m)
	if err := waitTaskProgram(t, result); err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if m.phase != AsyncReady {
		t.Fatalf("taskModel phase = %v, want %v", m.phase, AsyncReady)
	}
}

func TestTaskProgramFailure(t *testing.T) {
	testErr := errors.New("task failed")
	m := newTaskModel(TaskConfig{Message: "Loading"}, func(context.Context, io.Writer) error {
		return testErr
	})
	_, result := runTestTaskProgram(t, m)
	if err := waitTaskProgram(t, result); !errors.Is(err, testErr) {
		t.Fatalf("err = %v, want %v", err, testErr)
	}
	if m.phase != AsyncError {
		t.Fatalf("taskModel phase = %v, want %v", m.phase, AsyncError)
	}
}

// A signal makes Bubble Tea quit on its own, as p.Quit does here. The caller
// must get cancellation, and only after the task has stopped.
func TestTaskProgramQuitWaitsForTask(t *testing.T) {
	started, stopped := make(chan struct{}), false
	m := newTaskModel(TaskConfig{Message: "Loading"}, func(ctx context.Context, _ io.Writer) error {
		close(started)
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond)
		stopped = true
		return ctx.Err()
	})
	p, result := runTestTaskProgram(t, m)
	<-started
	p.Quit()
	if err := waitTaskProgram(t, result); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if !stopped {
		t.Fatal("returned before the task stopped")
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
