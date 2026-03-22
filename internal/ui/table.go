package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

// TableColumn defines a rendered column in a static table view.
type TableColumn struct {
	Title    string
	MinWidth int
	MaxWidth int
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	sectionBox = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(subtleColor).
			Padding(0, 1)
	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	tableRuleStyle   = lipgloss.NewStyle().Foreground(subtleColor)
)

func outputIsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// Title renders a section title.
func Title(s string) string {
	return render(titleStyle, s)
}

// Section renders a section. The title is optional. On TTYs it adds a border;
// otherwise it emits simple plain-text output.
func Section(title string, body ...string) string {
	parts := make([]string, 0, len(body)+1)
	if strings.TrimSpace(title) != "" {
		parts = append(parts, Title(title))
	}
	parts = append(parts, body...)
	content := strings.Join(parts, "\n")
	if !outputIsTTY() {
		return content
	}
	return sectionBox.Render(content)
}

// RenderTable renders a static table using lipgloss so ANSI-colored content is
// measured and truncated correctly.
func RenderTable(columns []TableColumn, rows [][]string) string {
	if len(columns) == 0 {
		return ""
	}

	widths := tableColumnWidths(columns, rows)
	header := tableLine(columns, widths, func(col int, value string) string {
		return tableHeaderStyle.Render(value)
	})
	lines := []string{header, tableRuleStyle.Render(strings.Repeat("─", ansi.StringWidth(header)))}
	for _, row := range rows {
		lines = append(lines, tableLineValues(row, widths, nil))
	}
	return strings.Join(lines, "\n")
}

func tableColumnWidths(columns []TableColumn, rows [][]string) []int {
	widths := make([]int, len(columns))
	for i, col := range columns {
		width := ansi.StringWidth(col.Title)
		for _, row := range rows {
			if i >= len(row) {
				continue
			}
			if w := ansi.StringWidth(row[i]); w > width {
				width = w
			}
		}
		if col.MinWidth > 0 && width < col.MinWidth {
			width = col.MinWidth
		}
		if col.MaxWidth > 0 && width > col.MaxWidth {
			width = col.MaxWidth
		}
		widths[i] = width
	}
	return widths
}

func tableLine(columns []TableColumn, widths []int, decorate func(col int, value string) string) string {
	values := make([]string, len(columns))
	for i, col := range columns {
		values[i] = col.Title
	}
	return tableLineValues(values, widths, decorate)
}

func tableLineValues(values []string, widths []int, decorate func(col int, value string) string) string {
	cells := make([]string, len(widths))
	for i := range widths {
		value := ""
		if i < len(values) {
			value = values[i]
		}
		value = fitTableCell(value, widths[i])
		if decorate != nil {
			value = decorate(i, value)
		}
		cells[i] = " " + value + " "
	}
	return strings.Join(cells, " ")
}

func fitTableCell(value string, width int) string {
	if width <= 0 {
		return ""
	}
	value = ansi.Truncate(value, width, "…")
	if pad := width - ansi.StringWidth(value); pad > 0 {
		value += strings.Repeat(" ", pad)
	}
	return value
}
