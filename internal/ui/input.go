package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type inputModel struct {
	message   string
	prefix    string
	textInput textinput.Model
	submitted bool
	canceled  bool
}

func newInputModel(message, prefix, placeholder string) inputModel {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Focus()
	ti.PromptStyle = accentStyle
	ti.Prompt = ""

	return inputModel{
		message:   message,
		prefix:    prefix,
		textInput: ti,
	}
}

func (m inputModel) Init() tea.Cmd {
	return textinput.Blink
}

func (m inputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.submitted = true
			return m, tea.Quit
		case "esc", "ctrl+c":
			m.canceled = true
			return m, tea.Quit
		}
	}

	var cmd tea.Cmd
	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m inputModel) View() string {
	if m.submitted || m.canceled {
		val := m.textInput.Value()
		if m.canceled {
			val = ""
		}
		return fmt.Sprintf("%s %s %s\n", m.prefix, m.message, Bold(val))
	}

	return fmt.Sprintf("%s %s %s", m.prefix, m.message, m.textInput.View())
}

func (m inputModel) Value() string {
	if m.canceled {
		return ""
	}
	return m.textInput.Value()
}
