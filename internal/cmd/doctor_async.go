package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type doctorCheckMsg struct {
	check doctorCheck
}

type doctorDoneMsg struct {
	hasErrors bool
	err       error
}

type doctorModel struct {
	spinner   spinner.Model
	repoRoot  string
	checks    []doctorCheck
	hasErrors bool
	err       error
	phase     ui.AsyncPhase
	ctx       context.Context
	cancel    context.CancelFunc
	send      func(tea.Msg)
}

func newDoctorModel(repoRoot string) *doctorModel {
	ctx, cancel := context.WithCancel(context.Background())
	return &doctorModel{
		spinner: spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
			spinner.WithStyle(ui.ForegroundStyle(ui.SubtleColor())),
		),
		repoRoot: repoRoot,
		phase:    ui.AsyncLoading,
		ctx:      ctx,
		cancel:   cancel,
	}
}

func (m *doctorModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.runChecks())
}

func (m *doctorModel) runChecks() tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		hasErrors := walkDoctorChecks(ctx, m.repoRoot, func(check doctorCheck) {
			if m.send != nil {
				m.send(doctorCheckMsg{check: check})
			}
		})
		return doctorDoneMsg{hasErrors: hasErrors, err: ctx.Err()}
	}
}

func (m *doctorModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case doctorCheckMsg:
		m.checks = append(m.checks, msg.check)
		if msg.check.Level == doctorError {
			m.hasErrors = true
		}
		if len(m.checks) > 0 {
			m.phase = ui.AsyncPartial
		}
		return m, nil
	case doctorDoneMsg:
		m.hasErrors = m.hasErrors || msg.hasErrors
		m.err = msg.err
		switch {
		case errors.Is(m.err, context.Canceled):
			m.phase = ui.AsyncCanceled
		case m.err != nil:
			m.phase = ui.AsyncError
		default:
			m.phase = ui.AsyncReady
		}
		return m, tea.Quit
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.cancel()
			m.err = context.Canceled
			m.phase = ui.AsyncCanceled
			return m, tea.Quit
		}
	case spinner.TickMsg:
		if m.phase.Active() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *doctorModel) View() string {
	footer := ""
	if m.phase.Active() {
		footer = fmt.Sprintf("%s %s", m.spinner.View(), ui.Subtle(fmt.Sprintf("running checks… %d completed", len(m.checks))))
	}
	return renderDoctorChecksWithFooter(m.checks, footer) + "\n"
}

func runDoctorAsync(repoRoot string) error {
	m := newDoctorModel(repoRoot)
	p := ui.NewProgram(m, os.Stdout)
	m.send = p.Send
	result, err := p.Run()
	m.cancel()
	if err != nil {
		return err
	}
	r := result.(*doctorModel)
	if errors.Is(r.err, context.Canceled) {
		return nil
	}
	if r.hasErrors {
		return fmt.Errorf("doctor found issues")
	}
	return nil
}
