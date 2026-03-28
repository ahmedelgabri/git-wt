package ui

import (
	"bufio"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var (
	isTerminal = func(f *os.File) bool {
		return term.IsTerminal(int(f.Fd()))
	}
	getTerminalSize = func(f *os.File) (int, int, error) {
		return term.GetSize(int(f.Fd()))
	}
)

// stdinReader can be overridden in tests to provide canned input.
var (
	stdinReader       func() *bufio.Reader
	sharedStdinReader = bufio.NewReader(os.Stdin)
)

func getReader() *bufio.Reader {
	if stdinReader != nil {
		return stdinReader()
	}
	return sharedStdinReader
}

// NoColor reports whether styling should be suppressed.
func NoColor() bool {
	return os.Getenv("NO_COLOR") != ""
}

// InputTTY reports whether stdin is an interactive terminal.
func InputTTY() bool {
	return stdinReader == nil && isTerminal(os.Stdin)
}

// StdoutTTY reports whether stdout is an interactive terminal.
func StdoutTTY() bool {
	return isTerminal(os.Stdout)
}

// StderrTTY reports whether stderr is an interactive terminal.
func StderrTTY() bool {
	return isTerminal(os.Stderr)
}

// CanPrompt reports whether interactive prompt UIs can run.
func CanPrompt() bool {
	return InputTTY() && StderrTTY()
}

// CanRenderSelection reports whether interactive selector UIs can run.
func CanRenderSelection() bool {
	return InputTTY() && StdoutTTY()
}

// TerminalSize returns the width/height for a terminal-backed file.
func TerminalSize(f *os.File) (width, height int, ok bool) {
	if f == nil || !isTerminal(f) {
		return 0, 0, false
	}
	width, height, err := getTerminalSize(f)
	if err != nil {
		return 0, 0, false
	}
	return width, height, true
}

// StdoutSize returns stdout's terminal dimensions when available.
func StdoutSize() (width, height int, ok bool) {
	return TerminalSize(os.Stdout)
}

// StderrSize returns stderr's terminal dimensions when available.
func StderrSize() (width, height int, ok bool) {
	return TerminalSize(os.Stderr)
}

// NewProgram creates a Bubble Tea program bound to the given output stream.
func NewProgram(model tea.Model, output *os.File, opts ...tea.ProgramOption) *tea.Program {
	programOpts := make([]tea.ProgramOption, 0, len(opts)+1)
	if output != nil {
		programOpts = append(programOpts, tea.WithOutput(output))
	}
	programOpts = append(programOpts, opts...)
	return tea.NewProgram(model, programOpts...)
}

// ForegroundStyle returns a colorized style unless NO_COLOR is set.
func ForegroundStyle(color lipgloss.TerminalColor) lipgloss.Style {
	style := lipgloss.NewStyle()
	if NoColor() {
		return style
	}
	return style.Foreground(color)
}
