package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type confirmModel struct {
	message   string
	confirmed bool
	done      bool
}

func newConfirmModel(message string) confirmModel {
	return confirmModel{message: normalizeConfirmMessage(message)}
}

func normalizeConfirmMessage(message string) string {
	message = strings.TrimSpace(message)
	for _, suffix := range []string{"[y/N]:", "[y/N]", "[y/n]:", "[y/n]", "(y/n):", "(y/n)"} {
		if strings.HasSuffix(message, suffix) {
			return strings.TrimSpace(strings.TrimSuffix(message, suffix))
		}
	}
	return message
}

func confirmPromptSuffix() string {
	return Muted("[y/N]")
}

func confirmPromptSuffixPlain() string {
	return "[y/N]"
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

	return fmt.Sprintf("%s %s %s ", Accent("?"), m.message, confirmPromptSuffix())
}
