package ui

import (
	"os"
	"strings"

	bubbletable "github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"
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
)

func outputIsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// Title renders a section title.
func Title(s string) string {
	return render(titleStyle, s)
}

// Section renders a titled section. On TTYs it adds a border; otherwise it
// emits simple plain-text output.
func Section(title string, body ...string) string {
	parts := []string{Title(title)}
	for _, part := range body {
		if strings.TrimSpace(part) == "" {
			continue
		}
		parts = append(parts, part)
	}
	content := strings.Join(parts, "\n")
	if !outputIsTTY() {
		return content
	}
	return sectionBox.Render(content)
}

// RenderTable renders a static table using Charm's bubbles/table component.
func RenderTable(columns []TableColumn, rows [][]string) string {
	if len(columns) == 0 {
		return ""
	}

	bCols := make([]bubbletable.Column, len(columns))
	totalWidth := 0
	for i, col := range columns {
		width := lipgloss.Width(col.Title)
		for _, row := range rows {
			if i >= len(row) {
				continue
			}
			if w := lipgloss.Width(row[i]); w > width {
				width = w
			}
		}
		if col.MinWidth > 0 && width < col.MinWidth {
			width = col.MinWidth
		}
		if col.MaxWidth > 0 && width > col.MaxWidth {
			width = col.MaxWidth
		}
		bCols[i] = bubbletable.Column{Title: col.Title, Width: width}
		totalWidth += width + 2
	}

	bRows := make([]bubbletable.Row, 0, len(rows))
	for _, row := range rows {
		bRows = append(bRows, bubbletable.Row(row))
	}

	styles := bubbletable.DefaultStyles()
	styles.Header = styles.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderBottom(true).
		BorderForeground(subtleColor).
		Bold(true)
	if !noColor() {
		styles.Header = styles.Header.Foreground(accentColor)
	}
	styles.Cell = styles.Cell.Padding(0, 1)
	styles.Selected = styles.Cell

	height := len(rows) + 1
	if height < 2 {
		height = 2
	}

	m := bubbletable.New(
		bubbletable.WithColumns(bCols),
		bubbletable.WithRows(bRows),
		bubbletable.WithHeight(height),
		bubbletable.WithWidth(totalWidth),
		bubbletable.WithStyles(styles),
	)
	return m.View()
}
