package ui

import (
	"bufio"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestPromptInputDistinguishesBlankFromEOF(t *testing.T) {
	old := stdinReader
	t.Cleanup(func() { stdinReader = old })
	for _, tc := range []struct {
		input   string
		wantErr error
	}{{"\n", nil}, {"", io.EOF}} {
		stdinReader = func() *bufio.Reader { return bufio.NewReader(strings.NewReader(tc.input)) }
		value, err := PromptInputResult("path")
		if value != "" || !errors.Is(err, tc.wantErr) {
			t.Fatalf("input %q: value=%q, error=%v", tc.input, value, err)
		}
	}
}

func TestTaskCancellationWaitsForWorker(t *testing.T) {
	m := newTaskModel(TaskConfig{Message: "working"}, func(context.Context, io.Writer) error { return nil })
	defer m.cancel()
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd != nil {
		t.Fatal("cancellation must wait for the worker before quitting")
	}
	if !errors.Is(m.ctx.Err(), context.Canceled) {
		t.Fatalf("worker context not canceled: %v", m.ctx.Err())
	}
	_, cmd = m.Update(taskFinishedMsg{})
	if cmd == nil {
		t.Fatal("worker completion should quit")
	}
	if !errors.Is(m.err, context.Canceled) {
		t.Fatalf("lost cancellation: %v", m.err)
	}
}
