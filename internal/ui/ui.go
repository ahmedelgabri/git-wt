package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

func noColor() bool { return os.Getenv("NO_COLOR") != "" }

var (
	accentColor    = lipgloss.Color("6") // Cyan
	successColor   = lipgloss.Color("2") // Green
	errorColor     = lipgloss.Color("1") // Red
	warnColor      = lipgloss.Color("3") // Yellow
	subtleColor    = lipgloss.Color("8") // Bright Black
	highlightColor = lipgloss.Color("5") // Magenta
)

var (
	accentStyle    = lipgloss.NewStyle().Foreground(accentColor)
	successStyle   = lipgloss.NewStyle().Foreground(successColor)
	errorStyle     = lipgloss.NewStyle().Foreground(errorColor)
	warnStyle      = lipgloss.NewStyle().Foreground(warnColor)
	subtleStyle    = lipgloss.NewStyle().Foreground(subtleColor)
	highlightStyle = lipgloss.NewStyle().Foreground(highlightColor)
	boldStyle      = lipgloss.NewStyle().Bold(true)
	dimStyle       = lipgloss.NewStyle().Faint(true)
)

// Color accessors for use by other packages (e.g. picker).
func AccentColor() lipgloss.TerminalColor    { return accentColor }
func SuccessColor() lipgloss.TerminalColor   { return successColor }
func ErrorColor() lipgloss.TerminalColor     { return errorColor }
func WarnColor() lipgloss.TerminalColor      { return warnColor }
func SubtleColor() lipgloss.TerminalColor    { return subtleColor }
func MutedColor() lipgloss.TerminalColor     { return subtleColor }
func HighlightColor() lipgloss.TerminalColor { return highlightColor }

func render(style lipgloss.Style, s string) string {
	if noColor() {
		return s
	}
	return style.Render(s)
}

func Green(s string) string     { return render(successStyle, s) }
func Red(s string) string       { return render(errorStyle, s) }
func Yellow(s string) string    { return render(warnStyle, s) }
func Accent(s string) string    { return render(accentStyle, s) }
func Subtle(s string) string    { return render(subtleStyle, s) }
func Muted(s string) string     { return render(subtleStyle, s) }
func Highlight(s string) string { return render(highlightStyle, s) }

func Bold(s string) string {
	if noColor() {
		return s
	}
	return boldStyle.Render(s)
}

func Dim(s string) string {
	if noColor() {
		return s
	}
	return dimStyle.Render(s)
}

func Error(msg string) {
	fmt.Fprintf(os.Stderr, "%s %s\n", Red("Error:"), msg)
}

func Warn(msg string) {
	fmt.Printf("%s %s\n", Yellow("Warning:"), msg)
}

func Info(msg string) {
	fmt.Println(Green(msg))
}

func Success(msg string) {
	fmt.Printf("%s %s\n", Green("●"), msg)
}

func Errorf(format string, a ...any) {
	Error(fmt.Sprintf(format, a...))
}

func Warnf(format string, a ...any) {
	Warn(fmt.Sprintf(format, a...))
}

func Infof(format string, a ...any) {
	Info(fmt.Sprintf(format, a...))
}

func Successf(format string, a ...any) {
	Success(fmt.Sprintf(format, a...))
}

// SuccessPrefix returns a formatted success message with a custom prefix.
func SuccessPrefix(prefix, msg string) string {
	return fmt.Sprintf("%s%s%s", prefix, Green("●"), " "+msg)
}

// FailPrefix returns a formatted failure message with a custom prefix.
func FailPrefix(prefix, msg string) string {
	return fmt.Sprintf("%s%s%s", prefix, Red("●"), " "+msg)
}

// stdinReader can be overridden in tests to provide canned input.
var stdinReader func() *bufio.Reader

func getReader() *bufio.Reader {
	if stdinReader != nil {
		return stdinReader()
	}
	return bufio.NewReader(os.Stdin)
}

// useSimpleIO returns true when bubbletea should not be used (test mocks or
// non-TTY stdin like piped input in scripts and E2E tests).
func useSimpleIO() bool {
	return stdinReader != nil || !term.IsTerminal(int(os.Stdin.Fd()))
}

// Confirm prints a styled prompt and returns true if the user enters "y".
// Uses bubbletea on TTYs; falls back to simple stdin reading otherwise.
func Confirm(msg string) bool {
	if useSimpleIO() {
		fmt.Printf("%s %s ", Accent("?"), msg)
		reader := getReader()
		input, _ := reader.ReadString('\n')
		return strings.TrimSpace(input) == "y"
	}

	m := newConfirmModel(msg)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return false
	}
	return result.(confirmModel).confirmed
}

// PromptInput prints a styled prompt and returns the trimmed user input.
// Uses bubbletea on TTYs; falls back to simple stdin reading otherwise.
func PromptInput(msg string) string {
	if useSimpleIO() {
		fmt.Printf("%s %s ", Accent("?"), msg)
		reader := getReader()
		input, _ := reader.ReadString('\n')
		return strings.TrimSpace(input)
	}

	m := newInputModel(msg, Accent("?"), "")
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return ""
	}
	return result.(inputModel).Value()
}

// PromptDangerous prints a red-styled prompt and returns true if the user's
// input matches the expected confirmation string exactly.
// Uses bubbletea on TTYs; falls back to simple stdin reading otherwise.
func PromptDangerous(msg, expect string) bool {
	if useSimpleIO() {
		fmt.Printf("%s %s ", Red("!"), msg)
		reader := getReader()
		input, _ := reader.ReadString('\n')
		return strings.TrimSpace(input) == expect
	}

	m := newInputModel(msg, Red("!"), "")
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return false
	}
	return result.(inputModel).Value() == expect
}

// Spin shows an animated spinner while running fn. On TTYs, renders a
// bubbletea spinner; otherwise prints a static message.
// The callback must NOT write to stdout (use git.RunWithOutput or
// git.Query instead of git.Run).
func Spin(msg string, fn func() error) error {
	if useSimpleIO() {
		fmt.Printf("%s %s...\n", Accent("●"), msg)
		if err := fn(); err != nil {
			fmt.Printf("%s %s\n", Red("●"), msg)
			return err
		}
		fmt.Printf("%s %s\n", Green("●"), msg)
		return nil
	}

	m := newSpinnerModel(msg, fn)
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return err
	}
	return result.(spinnerModel).err
}
