package ui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type confirmModel struct {
	message   string
	confirmed bool
	done      bool
}

func newConfirmModel(message string) confirmModel {
	return confirmModel{message: message}
}

func (m confirmModel) Init() tea.Cmd {
	return nil
}

func (m confirmModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.confirmed = true
			m.done = true
			return m, tea.Quit
		case "n", "N", "esc", "ctrl+c":
			m.confirmed = false
			m.done = true
			return m, tea.Quit
		case "enter":
			m.done = true
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m confirmModel) View() string {
	if m.done {
		choice := "n"
		if m.confirmed {
			choice = "y"
		}
		return fmt.Sprintf("%s %s %s\n", Accent("?"), m.message, Bold(choice))
	}

	yn := Muted("(y/n)")
	return fmt.Sprintf("%s %s %s ", Accent("?"), m.message, yn)
}
