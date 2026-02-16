package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
)

var noColor = os.Getenv("NO_COLOR") != ""

var (
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("2"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("1"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	boldStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Faint(true)
)

func green(s string) string {
	if noColor {
		return s
	}
	return greenStyle.Render(s)
}

func red(s string) string {
	if noColor {
		return s
	}
	return redStyle.Render(s)
}

func yellow(s string) string {
	if noColor {
		return s
	}
	return yellowStyle.Render(s)
}

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
	fmt.Fprintf(os.Stderr, "%s %s\n", red("Error:"), msg)
}

func Warn(msg string) {
	fmt.Printf("%s %s\n", yellow("Warning:"), msg)
}

func Info(msg string) {
	fmt.Println(green(msg))
}

func Success(msg string) {
	fmt.Printf("%s %s\n", green("✓"), msg)
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
	return fmt.Sprintf("%s%s%s", prefix, green("✓"), " "+msg)
}

// FailPrefix returns a formatted failure message with a custom prefix.
func FailPrefix(prefix, msg string) string {
	return fmt.Sprintf("%s%s%s", prefix, red("✗"), " "+msg)
}

// Green applies green styling to a string.
func Green(s string) string { return green(s) }

// Red applies red styling to a string.
func Red(s string) string { return red(s) }

// Yellow applies yellow styling to a string.
func Yellow(s string) string { return yellow(s) }
