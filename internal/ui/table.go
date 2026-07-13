package ui

import (
	"os"
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

func outputIsTTY(output *os.File) bool {
	return output != nil && isTerminal(output)
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

// Section renders a section for stdout. The title is optional. On TTYs it adds
// a border; otherwise it emits simple plain-text output.
func Section(title string, body ...string) string {
	return SectionFor(os.Stdout, title, body...)
}

// SectionFor renders a section sized for output.
func SectionFor(output *os.File, title string, body ...string) string {
	parts := make([]string, 0, len(body)+1)
	if strings.TrimSpace(title) != "" {
		parts = append(parts, Title(title))
	}
	parts = append(parts, body...)
	content := strings.Join(parts, "\n")
	if !outputIsTTY(output) {
		return content
	}

	style := sectionBoxStyle()
	if width, _, ok := TerminalSize(output); ok {
		contentWidth := width - style.GetHorizontalFrameSize()
		if contentWidth <= 0 {
			return wrapText(content, width)
		}
		content = wrapText(content, contentWidth)
		if textExceedsWidth(content, contentWidth) {
			return wrapText(content, width)
		}
	}
	return style.Render(content)
}

// RenderTable renders a responsive static table for stdout.
func RenderTable(columns []TableColumn, rows [][]string) string {
	return RenderTableFor(os.Stdout, columns, rows)
}

// RenderTableFor renders a responsive static table sized for output using
// lipgloss so ANSI-colored content is measured and truncated correctly. On
// narrow terminals, columns shrink below their preferred minimums; if even the
// headers cannot fit, rows are rendered as stacked fields instead.
func RenderTableFor(output *os.File, columns []TableColumn, rows [][]string) string {
	if len(columns) == 0 {
		return ""
	}

	widths := tableColumnWidths(columns, rows)
	if maxWidth, ok := tableContentWidth(output); ok {
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

func tableContentWidth(output *os.File) (int, bool) {
	width, _, ok := TerminalSize(output)
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

	wrapped := ansi.Wrap(s, width, " /._")
	// Wrap can leave a breakpoint one cell past the limit. Hardwrap enforces
	// the width without changing lines that already fit.
	wrapped = ansi.Hardwrap(wrapped, width, true)
	return preserveANSIStylesAcrossLines(wrapped)
}

func textExceedsWidth(s string, width int) bool {
	for _, line := range strings.Split(s, "\n") {
		if ansi.StringWidth(line) > width {
			return true
		}
	}
	return false
}

// preserveANSIStylesAcrossLines closes and reopens active SGR styles so the
// resets Lipgloss adds around each border do not clear wrapped continuations.
func preserveANSIStylesAcrossLines(s string) string {
	if !strings.Contains(s, "\n") || !strings.Contains(s, "\x1b[") {
		return s
	}

	var result strings.Builder
	result.Grow(len(s))
	activeStyles := ""
	for i := 0; i < len(s); {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			end := i + 2
			for end < len(s) && (s[end] < 0x40 || s[end] > 0x7e) {
				end++
			}
			if end < len(s) {
				sequence := s[i : end+1]
				result.WriteString(sequence)
				if s[end] == 'm' {
					params := sequence[2 : len(sequence)-1]
					switch {
					case params == "" || params == "0":
						activeStyles = ""
					case strings.HasPrefix(params, "0;") || strings.HasPrefix(params, "0:"):
						activeStyles = sequence
					default:
						activeStyles += sequence
					}
				}
				i = end + 1
				continue
			}
		}

		if s[i] == '\n' && activeStyles != "" {
			result.WriteString(ansi.ResetStyle)
			result.WriteByte('\n')
			result.WriteString(activeStyles)
			i++
			continue
		}
		result.WriteByte(s[i])
		i++
	}
	return result.String()
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
