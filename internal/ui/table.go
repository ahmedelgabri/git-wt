package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// TableColumn defines a rendered column in a static table view.
type TableColumn struct {
	Title    string
	MinWidth int
	MaxWidth int
}

var (
	titleStyle       = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	tableHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(accentColor)
	tableRuleStyle   = lipgloss.NewStyle().Foreground(subtleColor)
)

func outputIsTTY() bool {
	return StdoutTTY()
}

func sectionBoxStyle() lipgloss.Style {
	style := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	if NoColor() {
		return style
	}
	return style.BorderForeground(subtleColor)
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

	style := sectionBoxStyle()
	if width, _, ok := StdoutSize(); ok {
		contentWidth := width - style.GetHorizontalFrameSize()
		if contentWidth <= 0 {
			return wrapText(content, width)
		}
		content = wrapText(content, contentWidth)
	}
	return style.Render(content)
}

// RenderTable renders a responsive static table using lipgloss so ANSI-colored
// content is measured and truncated correctly. On narrow terminals, columns
// shrink below their preferred minimums; if even the headers cannot fit, rows
// are rendered as stacked fields instead.
func RenderTable(columns []TableColumn, rows [][]string) string {
	if len(columns) == 0 {
		return ""
	}

	widths := tableColumnWidths(columns, rows)
	if maxWidth, ok := tableContentWidth(); ok {
		var fits bool
		widths, fits = fitTableWidths(columns, widths, maxWidth)
		if !fits {
			return renderStackedTable(columns, rows, maxWidth)
		}
	}

	header := tableLine(columns, widths, func(col int, value string) string {
		return render(tableHeaderStyle, value)
	})
	lines := []string{header, render(tableRuleStyle, strings.Repeat("─", ansi.StringWidth(header)))}
	for _, row := range rows {
		lines = append(lines, tableLineValues(row, widths, nil))
	}
	return strings.Join(lines, "\n")
}

func tableContentWidth() (int, bool) {
	width, _, ok := StdoutSize()
	if !ok {
		return 0, false
	}
	return width - sectionBoxStyle().GetHorizontalFrameSize(), true
}

func fitTableWidths(columns []TableColumn, preferred []int, maxWidth int) ([]int, bool) {
	cellSpacing := 3*len(columns) - 1
	available := maxWidth - cellSpacing
	if available < len(columns) {
		return nil, false
	}

	minimums := make([]int, len(columns))
	minimumTotal := 0
	preferredTotal := 0
	for i, col := range columns {
		minimums[i] = ansi.StringWidth(col.Title)
		if minimums[i] < 1 {
			minimums[i] = 1
		}
		if minimums[i] > preferred[i] && preferred[i] > 0 {
			minimums[i] = preferred[i]
		}
		minimumTotal += minimums[i]
		preferredTotal += preferred[i]
	}
	if minimumTotal > available {
		return nil, false
	}
	if preferredTotal <= available {
		return preferred, true
	}

	widths := append([]int(nil), minimums...)
	remaining := available - minimumTotal
	for remaining > 0 {
		grew := false
		for i := range widths {
			if widths[i] >= preferred[i] {
				continue
			}
			widths[i]++
			remaining--
			grew = true
			if remaining == 0 {
				break
			}
		}
		if !grew {
			break
		}
	}
	return widths, true
}

func renderStackedTable(columns []TableColumn, rows [][]string, maxWidth int) string {
	lines := make([]string, 0, len(rows)*(len(columns)+1))
	if len(rows) == 0 {
		for _, col := range columns {
			lines = append(lines, render(tableHeaderStyle, col.Title))
		}
		return strings.Join(lines, "\n")
	}

	for rowIndex, row := range rows {
		if rowIndex > 0 {
			lines = append(lines, "")
		}
		for colIndex, col := range columns {
			value := ""
			if colIndex < len(row) {
				value = row[colIndex]
			}
			line := render(tableHeaderStyle, col.Title+":")
			if value != "" {
				line += " " + value
			}
			lines = append(lines, wrapText(line, maxWidth))
		}
	}
	return strings.Join(lines, "\n")
}

func wrapText(s string, width int) string {
	if width <= 0 {
		return s
	}
	return ansi.Wrap(s, width, " /._")
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
