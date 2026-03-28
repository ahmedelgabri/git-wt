package ui

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeInputValue(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{name: "plain", in: "feature", want: "feature"},
		{name: "trim spaces", in: "  feature  ", want: "feature"},
		{name: "trim newline", in: "feature\n", want: "feature"},
		{name: "trim both", in: "  feature\n", want: "feature"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeInputValue(tt.in); got != tt.want {
				t.Fatalf("normalizeInputValue(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNormalizeConfirmMessage(t *testing.T) {
	for _, tt := range []struct {
		name string
		in   string
		want string
	}{
		{name: "already plain", in: "Continue?", want: "Continue?"},
		{name: "strip default suffix", in: "Continue? [y/N]:", want: "Continue?"},
		{name: "strip lowercase suffix", in: "Continue? [y/n]", want: "Continue?"},
		{name: "strip paren suffix", in: "Continue? (y/n)", want: "Continue?"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeConfirmMessage(tt.in); got != tt.want {
				t.Fatalf("normalizeConfirmMessage(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestInputTTYDisabledWhenMockReaderInstalled(t *testing.T) {
	cleanup := mockStdin("")
	defer cleanup()

	if InputTTY() {
		t.Fatal("InputTTY() should be false when stdinReader override is installed")
	}
	if CanPrompt() {
		t.Fatal("CanPrompt() should be false when stdinReader override is installed")
	}
}

func TestTTYHelpers(t *testing.T) {
	oldIsTerminal := isTerminal
	isTerminal = func(f *os.File) bool {
		switch f {
		case os.Stdin, os.Stderr:
			return true
		case os.Stdout:
			return false
		default:
			return false
		}
	}
	defer func() { isTerminal = oldIsTerminal }()

	if !InputTTY() {
		t.Fatal("InputTTY() should be true when stdin is marked interactive")
	}
	if StdoutTTY() {
		t.Fatal("StdoutTTY() should be false when stdout is marked non-interactive")
	}
	if !StderrTTY() {
		t.Fatal("StderrTTY() should be true when stderr is marked interactive")
	}
	if !CanPrompt() {
		t.Fatal("CanPrompt() should be true when stdin/stderr are interactive")
	}
	if CanRenderSelection() {
		t.Fatal("CanRenderSelection() should be false when stdout is not interactive")
	}
}

func TestTerminalSizeHelpers(t *testing.T) {
	oldIsTerminal := isTerminal
	oldGetTerminalSize := getTerminalSize
	isTerminal = func(f *os.File) bool { return f == os.Stdout }
	getTerminalSize = func(f *os.File) (int, int, error) {
		if f != os.Stdout {
			return 0, 0, nil
		}
		return 120, 40, nil
	}
	defer func() {
		isTerminal = oldIsTerminal
		getTerminalSize = oldGetTerminalSize
	}()

	width, height, ok := StdoutSize()
	if !ok {
		t.Fatal("StdoutSize() should report ok for interactive stdout")
	}
	if width != 120 || height != 40 {
		t.Fatalf("StdoutSize() = (%d, %d), want (120, 40)", width, height)
	}

	if _, _, ok := StderrSize(); ok {
		t.Fatal("StderrSize() should report not ok for non-interactive stderr")
	}
}

func TestForegroundStyleRespectsNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := ForegroundStyle(AccentColor()).Render("hello")
	if got != "hello" {
		t.Fatalf("ForegroundStyle().Render() with NO_COLOR = %q, want %q", got, "hello")
	}
}

func TestSectionTTYNoColorContainsNoANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	oldIsTerminal := isTerminal
	isTerminal = func(f *os.File) bool { return f == os.Stdout }
	defer func() { isTerminal = oldIsTerminal }()

	got := Section("Status", "hello")
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("Section() with NO_COLOR should not contain ANSI escapes: %q", got)
	}
	if !strings.Contains(got, "Status") || !strings.Contains(got, "hello") {
		t.Fatalf("Section() missing content: %q", got)
	}
}

func TestRenderTableTTYNoColorContainsNoANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "1")

	got := RenderTable([]TableColumn{{Title: "COL1"}, {Title: "COL2"}}, [][]string{{"a", "b"}})
	if strings.Contains(got, "\x1b[") {
		t.Fatalf("RenderTable() with NO_COLOR should not contain ANSI escapes: %q", got)
	}
}

func TestAsyncPhaseHelpers(t *testing.T) {
	if !AsyncReady.Done() || AsyncLoading.Done() {
		t.Fatal("AsyncPhase.Done() should only be true for terminal states")
	}
	if !AsyncLoading.Active() || AsyncCanceled.Active() {
		t.Fatal("AsyncPhase.Active() should be false for terminal states")
	}
}
