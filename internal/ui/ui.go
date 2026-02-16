package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var noColor = os.Getenv("NO_COLOR") != ""

var (
	accentColor    = lipgloss.AdaptiveColor{Light: "#0d9488", Dark: "#5eead4"}
	successColor   = lipgloss.AdaptiveColor{Light: "#16a34a", Dark: "#4ade80"}
	errorColor     = lipgloss.AdaptiveColor{Light: "#dc2626", Dark: "#f87171"}
	warnColor      = lipgloss.AdaptiveColor{Light: "#d97706", Dark: "#fbbf24"}
	subtleColor    = lipgloss.AdaptiveColor{Light: "#78716c", Dark: "#a8a29e"}
	mutedColor     = lipgloss.AdaptiveColor{Light: "#a8a29e", Dark: "#78716c"}
	highlightColor = lipgloss.AdaptiveColor{Light: "#7c3aed", Dark: "#c4b5fd"}
)

var (
	accentStyle    = lipgloss.NewStyle().Foreground(accentColor)
	successStyle   = lipgloss.NewStyle().Foreground(successColor)
	errorStyle     = lipgloss.NewStyle().Foreground(errorColor)
	warnStyle      = lipgloss.NewStyle().Foreground(warnColor)
	subtleStyle    = lipgloss.NewStyle().Foreground(subtleColor)
	mutedStyle     = lipgloss.NewStyle().Foreground(mutedColor)
	highlightStyle = lipgloss.NewStyle().Foreground(highlightColor)
	boldStyle      = lipgloss.NewStyle().Bold(true)
	dimStyle       = lipgloss.NewStyle().Faint(true)
)

// Color accessors for use by other packages (e.g. picker).
func AccentColor() lipgloss.AdaptiveColor    { return accentColor }
func SuccessColor() lipgloss.AdaptiveColor   { return successColor }
func ErrorColor() lipgloss.AdaptiveColor     { return errorColor }
func WarnColor() lipgloss.AdaptiveColor      { return warnColor }
func SubtleColor() lipgloss.AdaptiveColor    { return subtleColor }
func MutedColor() lipgloss.AdaptiveColor     { return mutedColor }
func HighlightColor() lipgloss.AdaptiveColor { return highlightColor }

func render(style lipgloss.Style, s string) string {
	if noColor {
		return s
	}
	return style.Render(s)
}

func Green(s string) string     { return render(successStyle, s) }
func Red(s string) string       { return render(errorStyle, s) }
func Yellow(s string) string    { return render(warnStyle, s) }
func Accent(s string) string    { return render(accentStyle, s) }
func Subtle(s string) string    { return render(subtleStyle, s) }
func Muted(s string) string     { return render(mutedStyle, s) }
func Highlight(s string) string { return render(highlightStyle, s) }

func Bold(s string) string {
	if noColor {
		return s
	}
	return boldStyle.Render(s)
}

func Dim(s string) string {
	if noColor {
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
	fmt.Printf("%s %s\n", Green("✓"), msg)
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
	return fmt.Sprintf("%s%s%s", prefix, Green("✓"), " "+msg)
}

// FailPrefix returns a formatted failure message with a custom prefix.
func FailPrefix(prefix, msg string) string {
	return fmt.Sprintf("%s%s%s", prefix, Red("✗"), " "+msg)
}

// stdinReader can be overridden in tests to provide canned input.
var stdinReader func() *bufio.Reader

func getReader() *bufio.Reader {
	if stdinReader != nil {
		return stdinReader()
	}
	return bufio.NewReader(os.Stdin)
}

// Confirm prints a styled prompt and returns true if the user enters "y".
func Confirm(msg string) bool {
	fmt.Printf("%s %s ", Accent("?"), msg)
	reader := getReader()
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input) == "y"
}

// PromptInput prints a styled prompt and returns the trimmed user input.
func PromptInput(msg string) string {
	fmt.Printf("%s %s ", Accent("?"), msg)
	reader := getReader()
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// PromptDangerous prints a red-styled prompt and returns true if the user's
// input matches the expected confirmation string exactly.
func PromptDangerous(msg, expect string) bool {
	fmt.Printf("%s %s ", Red("!"), msg)
	reader := getReader()
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input) == expect
}
