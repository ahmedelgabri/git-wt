package ui

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// TaskFunc runs work for a task UI. It receives a context for cancellation and
// an optional writer for streamed output.
type TaskFunc func(ctx context.Context, w io.Writer) error

// TaskConfig configures task rendering behavior.
type TaskConfig struct {
	Message    string
	ShowOutput bool
}

type taskLogMsg struct {
	text string
}

type taskFinishedMsg struct {
	err error
}

type taskModel struct {
	spinner    spinner.Model
	viewport   viewport.Model
	message    string
	showOutput bool
	output     strings.Builder
	phase      AsyncPhase
	fn         TaskFunc
	ctx        context.Context
	cancel     context.CancelFunc
	send       func(tea.Msg)
	err        error
}

func newTaskModel(cfg TaskConfig, fn TaskFunc) *taskModel {
	ctx, cancel := context.WithCancel(context.Background())
	m := &taskModel{
		spinner: spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
			spinner.WithStyle(ForegroundStyle(accentColor)),
		),
		viewport:   viewport.New(0, 0),
		message:    cfg.Message,
		showOutput: cfg.ShowOutput,
		phase:      AsyncLoading,
		fn:         fn,
		ctx:        ctx,
		cancel:     cancel,
	}
	m.setSize(80, 24)
	if width, height, ok := StderrSize(); ok {
		m.setSize(width, height)
	}
	return m
}

func (m *taskModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.runTask())
}

func (m *taskModel) runTask() tea.Cmd {
	fn := m.fn
	ctx := m.ctx
	writer := io.Discard
	if m.showOutput {
		writer = &taskLogWriter{send: func(msg tea.Msg) {
			if m.send != nil {
				m.send(msg)
			}
		}}
	}

	return func() tea.Msg {
		return taskFinishedMsg{err: fn(ctx, writer)}
	}
}

func (m *taskModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case taskLogMsg:
		if msg.text == "" {
			return m, nil
		}
		if m.phase == AsyncLoading {
			m.phase = AsyncPartial
		}
		m.output.WriteString(msg.text)
		m.viewport.SetContent(strings.TrimRight(m.output.String(), "\n"))
		m.viewport.GotoBottom()
		return m, nil
	case taskFinishedMsg:
		m.err = msg.err
		switch {
		case m.phase == AsyncCanceled:
			if m.err == nil {
				m.err = context.Canceled
			}
		case m.err != nil:
			m.phase = AsyncError
		default:
			m.phase = AsyncReady
		}
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			m.cancel()
			m.phase = AsyncCanceled
			m.err = context.Canceled
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.setSize(msg.Width, msg.Height)
		return m, nil
	case spinner.TickMsg:
		if m.phase.Active() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	if m.showOutput {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *taskModel) View() string {
	status := m.statusLine()
	if !m.showOutput || m.output.Len() == 0 {
		return status + "\n"
	}
	return status + "\n\n" + m.viewport.View() + "\n"
}

func (m *taskModel) statusLine() string {
	switch m.phase {
	case AsyncReady:
		return fmt.Sprintf("%s %s", Green("●"), m.message)
	case AsyncError, AsyncCanceled:
		return fmt.Sprintf("%s %s", Red("●"), m.message)
	default:
		return fmt.Sprintf("%s %s", m.spinner.View(), m.message)
	}
}

func (m *taskModel) setSize(width, height int) {
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}
	vpHeight := height - 4
	if vpHeight < 3 {
		vpHeight = 3
	}
	if vpHeight > 12 {
		vpHeight = 12
	}
	vpWidth := width - 2
	if vpWidth < 20 {
		vpWidth = 20
	}
	m.viewport.Width = vpWidth
	m.viewport.Height = vpHeight
	m.viewport.SetContent(strings.TrimRight(m.output.String(), "\n"))
}

type taskLogWriter struct {
	send func(tea.Msg)
}

func (w *taskLogWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if w.send != nil {
		w.send(taskLogMsg{text: string(p)})
	}
	return len(p), nil
}

// RunTask runs a task using the shared task UI on TTYs and a plain stderr
// fallback otherwise.
func RunTask(cfg TaskConfig, fn TaskFunc) error {
	if useSimpleIO() {
		return runTaskSimple(cfg, fn)
	}

	m := newTaskModel(cfg, fn)
	p := NewProgram(m, os.Stderr)
	m.send = p.Send
	result, err := p.Run()
	m.cancel()
	if err != nil {
		return err
	}
	return result.(*taskModel).err
}

func runTaskSimple(cfg TaskConfig, fn TaskFunc) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Fprintf(os.Stderr, "%s %s\n", Accent("●"), cfg.Message)
	writer := io.Discard
	if cfg.ShowOutput {
		writer = os.Stderr
	}
	if err := fn(ctx, writer); err != nil {
		fmt.Fprintf(os.Stderr, "%s %s\n", Red("●"), cfg.Message)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s %s\n", Green("●"), cfg.Message)
	return nil
}

// SpinContext runs a task without streamed output.
func SpinContext(msg string, fn func(ctx context.Context) error) error {
	return RunTask(TaskConfig{Message: msg}, func(ctx context.Context, _ io.Writer) error {
		return fn(ctx)
	})
}

// SpinWithOutputContext runs a task with streamed output.
func SpinWithOutputContext(msg string, fn func(ctx context.Context, w io.Writer) error) error {
	return RunTask(TaskConfig{Message: msg, ShowOutput: true}, fn)
}
