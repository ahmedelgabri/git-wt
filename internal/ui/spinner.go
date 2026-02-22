package ui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type taskDoneMsg struct {
	err error
}

type spinnerModel struct {
	spinner spinner.Model
	message string
	fn      func() error
	err     error
	done    bool
}

func newSpinnerModel(message string, fn func() error) spinnerModel {
	s := spinner.New(
		spinner.WithSpinner(spinner.MiniDot),
		spinner.WithStyle(accentStyle),
	)
	return spinnerModel{
		spinner: s,
		message: message,
		fn:      fn,
	}
}

func (m spinnerModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.runTask())
}

func (m spinnerModel) runTask() tea.Cmd {
	fn := m.fn
	return func() tea.Msg {
		return taskDoneMsg{err: fn()}
	}
}

func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case taskDoneMsg:
		m.done = true
		m.err = msg.err
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.done = true
			m.err = fmt.Errorf("interrupted")
			return m, tea.Quit
		}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m spinnerModel) View() string {
	if m.done {
		if m.err != nil {
			return fmt.Sprintf("%s %s\n", Red("●"), m.message)
		}
		return fmt.Sprintf("%s %s\n", Green("●"), m.message)
	}
	return fmt.Sprintf("%s %s\n", m.spinner.View(), m.message)
}
