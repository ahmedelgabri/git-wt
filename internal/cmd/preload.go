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

type preloadStatusMsg struct {
	phase   ui.AsyncPhase
	message string
}

type preloadDoneMsg[T any] struct {
	value T
	err   error
}

type preloadFunc[T any] func(ctx context.Context, update func(phase ui.AsyncPhase, message string)) (T, error)

type preloadModel[T any] struct {
	spinner spinner.Model
	phase   ui.AsyncPhase
	message string
	value   T
	err     error
	ctx     context.Context
	cancel  context.CancelFunc
	send    func(tea.Msg)
	load    preloadFunc[T]
}

func canUseSelectionPreloadUI() bool {
	return ui.CanPrompt() && ui.StdoutTTY()
}

func newPreloadModel[T any](ctx context.Context, message string, load preloadFunc[T]) *preloadModel[T] {
	baseCtx := ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(baseCtx)
	return &preloadModel[T]{
		spinner: spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
			spinner.WithStyle(ui.ForegroundStyle(ui.AccentColor())),
		),
		phase:   ui.AsyncLoading,
		message: message,
		ctx:     ctx,
		cancel:  cancel,
		load:    load,
	}
}

func (m *preloadModel[T]) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.loadCmd())
}

func (m *preloadModel[T]) loadCmd() tea.Cmd {
	ctx := m.ctx
	load := m.load
	return func() tea.Msg {
		value, err := load(ctx, func(phase ui.AsyncPhase, message string) {
			if m.send != nil {
				m.send(preloadStatusMsg{phase: phase, message: message})
			}
		})
		return preloadDoneMsg[T]{value: value, err: err}
	}
}

func (m *preloadModel[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case preloadStatusMsg:
		m.phase = msg.phase
		m.message = msg.message
		return m, nil
	case preloadDoneMsg[T]:
		m.value = msg.value
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
			m.phase = ui.AsyncCanceled
			m.err = context.Canceled
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

func (m *preloadModel[T]) View() string {
	prefix := m.spinner.View()
	switch m.phase {
	case ui.AsyncReady:
		prefix = ui.Green("●")
	case ui.AsyncError, ui.AsyncCanceled:
		prefix = ui.Red("●")
	}
	return fmt.Sprintf("%s %s\n", prefix, m.message)
}

func runPreload[T any](ctx context.Context, message string, load preloadFunc[T]) (T, error) {
	var zero T
	if !canUseSelectionPreloadUI() {
		return load(ctx, func(ui.AsyncPhase, string) {})
	}

	m := newPreloadModel(ctx, message, load)
	p := ui.NewProgram(m, os.Stderr)
	m.send = p.Send
	result, err := p.Run()
	m.cancel()
	if err != nil {
		return zero, err
	}
	r := result.(*preloadModel[T])
	if errors.Is(r.err, context.Canceled) {
		return zero, context.Canceled
	}
	return r.value, r.err
}
