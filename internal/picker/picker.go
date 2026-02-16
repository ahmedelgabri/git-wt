package picker

import (
	"fmt"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/sahilm/fuzzy"
)

var (
	selectedStyle = lipgloss.NewStyle().Foreground(ui.SuccessColor())
	cursorStyle   = lipgloss.NewStyle().Foreground(ui.AccentColor()).Bold(true)
	dimStyle      = lipgloss.NewStyle().Faint(true)
	headerStyle   = lipgloss.NewStyle().Foreground(ui.WarnColor())
	matchStyle    = lipgloss.NewStyle().Foreground(ui.HighlightColor()).Bold(true)
	previewSep    = lipgloss.NewStyle().Foreground(ui.MutedColor())
	descStyle     = lipgloss.NewStyle().Foreground(ui.SubtleColor())
	keyStyle      = lipgloss.NewStyle().Foreground(ui.AccentColor())
	filterPrompt  = lipgloss.NewStyle().Foreground(ui.AccentColor())
)

type model struct {
	items    []Item
	filtered []int         // indices into items
	matchMap map[int][]int // itemIndex -> matched character positions
	cursor   int
	multi    bool
	prompt   string
	header   string

	previewFunc func(Item) string

	filter   textinput.Model
	viewport viewport.Model
	preview  string

	width  int
	height int

	result Result
	done   bool
}

type previewMsg struct {
	content string
	label   string
}

func newModel(cfg Config) model {
	ti := textinput.New()
	ti.Placeholder = cfg.Prompt
	if ti.Placeholder == "" {
		ti.Placeholder = "Type to filter..."
	}
	ti.Focus()

	vp := viewport.New(40, 20)

	indices := make([]int, len(cfg.Items))
	for i := range cfg.Items {
		indices[i] = i
	}

	return model{
		items:       cfg.Items,
		filtered:    indices,
		matchMap:    make(map[int][]int),
		multi:       cfg.Multi,
		prompt:      cfg.Prompt,
		header:      cfg.Header,
		previewFunc: cfg.PreviewFunc,
		filter:      ti,
		viewport:    vp,
	}
}

func (m model) Init() tea.Cmd {
	cmds := []tea.Cmd{textinput.Blink}
	if m.previewFunc != nil && len(m.filtered) > 0 {
		item := m.items[m.filtered[0]]
		cmds = append(cmds, fetchPreview(m.previewFunc, item))
	}
	return tea.Batch(cmds...)
}

func fetchPreview(fn func(Item) string, item Item) tea.Cmd {
	return func() tea.Msg {
		return previewMsg{content: fn(item), label: item.Label}
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.previewFunc != nil {
			m.viewport.Width = msg.Width / 2
		}
		m.viewport.Height = msg.Height - 4 // header + filter + footer
		return m, nil

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("ctrl+c", "esc"))):
			m.result = Result{Canceled: true}
			m.done = true
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
			m.result = m.collectResult()
			m.done = true
			return m, tea.Quit

		case key.Matches(msg, key.NewBinding(key.WithKeys("tab"))):
			if m.multi && len(m.filtered) > 0 {
				idx := m.filtered[m.cursor]
				m.items[idx].selected = !m.items[idx].selected
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
				}
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("up", "ctrl+p"))):
			if m.cursor > 0 {
				m.cursor--
				return m, m.refreshPreview()
			}
			return m, nil

		case key.Matches(msg, key.NewBinding(key.WithKeys("down", "ctrl+n"))):
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
				return m, m.refreshPreview()
			}
			return m, nil

		default:
			var cmd tea.Cmd
			m.filter, cmd = m.filter.Update(msg)
			m.applyFilter()
			return m, tea.Batch(cmd, m.refreshPreview())
		}

	case previewMsg:
		// Only update if the preview is for the currently focused item
		if len(m.filtered) > 0 && m.items[m.filtered[m.cursor]].Label == msg.label {
			m.preview = msg.content
			m.viewport.SetContent(m.preview)
		}
		return m, nil
	}

	return m, nil
}

func (m *model) applyFilter() {
	query := m.filter.Value()
	m.matchMap = make(map[int][]int)

	if query == "" {
		m.filtered = make([]int, len(m.items))
		for i := range m.items {
			m.filtered[i] = i
		}
	} else {
		labels := make([]string, len(m.items))
		for i, item := range m.items {
			labels[i] = item.Label
		}
		matches := fuzzy.Find(query, labels)
		m.filtered = make([]int, len(matches))
		for i, match := range matches {
			m.filtered[i] = match.Index
			m.matchMap[match.Index] = match.MatchedIndexes
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m model) refreshPreview() tea.Cmd {
	if m.previewFunc == nil || len(m.filtered) == 0 {
		return nil
	}
	item := m.items[m.filtered[m.cursor]]
	return fetchPreview(m.previewFunc, item)
}

func (m model) collectResult() Result {
	if m.multi {
		var selected []Item
		for _, item := range m.items {
			if item.selected {
				selected = append(selected, item)
			}
		}
		// If nothing was explicitly selected, use the focused item
		if len(selected) == 0 && len(m.filtered) > 0 {
			selected = []Item{m.items[m.filtered[m.cursor]]}
		}
		return Result{Items: selected}
	}
	if len(m.filtered) > 0 {
		return Result{Items: []Item{m.items[m.filtered[m.cursor]]}}
	}
	return Result{Canceled: true}
}

// renderLabel renders an item label with fuzzy match characters highlighted.
func (m model) renderLabel(itemIdx int, label string) string {
	positions, ok := m.matchMap[itemIdx]
	if !ok || len(positions) == 0 {
		return label
	}

	posSet := make(map[int]struct{}, len(positions))
	for _, p := range positions {
		posSet[p] = struct{}{}
	}

	var b strings.Builder
	for i, ch := range label {
		if _, match := posSet[i]; match {
			b.WriteString(matchStyle.Render(string(ch)))
		} else {
			b.WriteRune(ch)
		}
	}
	return b.String()
}

func (m model) View() string {
	if m.done {
		return ""
	}

	var b strings.Builder

	if m.header != "" {
		b.WriteString(headerStyle.Render(m.header))
		b.WriteString("\n")
	}

	b.WriteString(filterPrompt.Render("/"))
	b.WriteString(" ")
	b.WriteString(m.filter.View())
	b.WriteString("\n")

	// Calculate available height for the list
	listHeight := m.height - 4
	if m.header != "" {
		listHeight -= strings.Count(m.header, "\n") + 1
	}
	if listHeight < 1 {
		listHeight = 10
	}

	// Determine visible window
	start := 0
	if m.cursor >= listHeight {
		start = m.cursor - listHeight + 1
	}
	end := start + listHeight
	if end > len(m.filtered) {
		end = len(m.filtered)
	}

	listLines := make([]string, 0, end-start)
	for vi := start; vi < end; vi++ {
		idx := m.filtered[vi]
		item := m.items[idx]
		var line string

		prefix := "  "
		if vi == m.cursor {
			prefix = cursorStyle.Render("> ")
		}

		label := m.renderLabel(idx, item.Label)

		if m.multi {
			check := dimStyle.Render("○")
			if item.selected {
				check = selectedStyle.Render("◉")
			}
			line = fmt.Sprintf("%s %s %s", prefix, check, label)
		} else {
			line = fmt.Sprintf("%s %s", prefix, label)
		}

		if item.Desc != "" {
			line += " " + descStyle.Render("·") + " " + descStyle.Render(item.Desc)
		}

		listLines = append(listLines, line)
	}

	listContent := strings.Join(listLines, "\n")

	if m.previewFunc != nil && m.width > 0 {
		listWidth := m.width/2 - 2
		previewWidth := m.width - listWidth - 3
		m.viewport.Width = previewWidth
		m.viewport.Height = listHeight
		m.viewport.SetContent(m.preview)

		// Render side by side
		leftLines := strings.Split(listContent, "\n")
		rightLines := strings.Split(m.viewport.View(), "\n")
		sep := previewSep.Render("│")

		maxLines := max(len(leftLines), len(rightLines))
		var combined strings.Builder
		for i := 0; i < maxLines; i++ {
			left := ""
			if i < len(leftLines) {
				left = leftLines[i]
			}
			right := ""
			if i < len(rightLines) {
				right = rightLines[i]
			}
			// Pad left column
			left = padRight(left, listWidth)
			combined.WriteString(left)
			combined.WriteString(" ")
			combined.WriteString(sep)
			combined.WriteString(" ")
			combined.WriteString(right)
			if i < maxLines-1 {
				combined.WriteString("\n")
			}
		}
		b.WriteString(combined.String())
	} else {
		b.WriteString(listContent)
	}

	b.WriteString("\n")

	// Footer with styled keys
	if m.multi {
		b.WriteString(keyStyle.Render("TAB"))
		b.WriteString(dimStyle.Render(" select"))
		b.WriteString(dimStyle.Render(" · "))
		b.WriteString(keyStyle.Render("ENTER"))
		b.WriteString(dimStyle.Render(" confirm"))
		b.WriteString(dimStyle.Render(" · "))
		b.WriteString(keyStyle.Render("ESC"))
		b.WriteString(dimStyle.Render(" cancel"))
	} else {
		b.WriteString(keyStyle.Render("ENTER"))
		b.WriteString(dimStyle.Render(" select"))
		b.WriteString(dimStyle.Render(" · "))
		b.WriteString(keyStyle.Render("ESC"))
		b.WriteString(dimStyle.Render(" cancel"))
	}

	return b.String()
}

func padRight(s string, width int) string {
	// Account for ANSI escape sequences when calculating visible length
	visible := lipgloss.Width(s)
	if visible >= width {
		return s
	}
	return s + strings.Repeat(" ", width-visible)
}

// Run displays the picker and returns the result.
func Run(cfg Config) (Result, error) {
	if len(cfg.Items) == 0 {
		return Result{Canceled: true}, nil
	}

	m := newModel(cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return Result{}, err
	}

	return finalModel.(model).result, nil
}
