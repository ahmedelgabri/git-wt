package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/picker"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type remoteBranchCandidate struct {
	RemoteRef string
	Remote    string
	Branch    string
	Age       string
	Author    string
	Subject   string
}

func (c remoteBranchCandidate) pickerItem() picker.Item {
	return picker.Item{
		Label: fmt.Sprintf("%s [%s]", c.Branch, c.Remote),
		Value: c.RemoteRef,
		Desc:  branchCandidateDesc(c.Age, c.Author, c.Subject),
	}
}

func branchCandidateDesc(age, author, subject string) string {
	parts := make([]string, 0, 3)
	if age != "" && age != "n/a" {
		parts = append(parts, age)
	}
	if author != "" {
		parts = append(parts, author)
	}
	if subject != "" {
		parts = append(parts, subject)
	}
	return strings.Join(parts, " · ")
}

func checkedOutBranches() map[string]bool {
	checkedOut := make(map[string]bool)
	if entries, err := worktree.List(); err == nil {
		for _, e := range entries {
			if e.Branch != "" && !e.Detached {
				checkedOut[e.Branch] = true
			}
		}
	}
	return checkedOut
}

func parseRemoteBranchCandidates(output string, checkedOut map[string]bool) []remoteBranchCandidate {
	if strings.TrimSpace(output) == "" {
		return nil
	}

	candidates := make([]remoteBranchCandidate, 0)
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.SplitN(line, "\t", 4)
		remoteRef := strings.TrimSpace(fields[0])
		if remoteRef == "" || strings.HasSuffix(remoteRef, "/HEAD") {
			continue
		}

		remote, branch := splitRemoteBranchRef(remoteRef)
		if remote == "" || branch == "" || checkedOut[branch] {
			continue
		}

		candidate := remoteBranchCandidate{
			RemoteRef: remoteRef,
			Remote:    remote,
			Branch:    branch,
		}
		if len(fields) > 1 {
			candidate.Age = humanizeCommitAge(fields[1])
		}
		if len(fields) > 2 {
			candidate.Author = strings.TrimSpace(fields[2])
		}
		if len(fields) > 3 {
			candidate.Subject = strings.TrimSpace(fields[3])
		}

		candidates = append(candidates, candidate)
	}
	return candidates
}

func loadInteractiveAddItems(ctx context.Context) ([]picker.Item, error) {
	if canUseAddPreloadUI() {
		return runAddPreload(ctx)
	}

	if err := fetchInteractiveBranches(); err != nil {
		return nil, err
	}
	return buildInteractiveAddItems(ctx)
}

func buildInteractiveAddItems(ctx context.Context) ([]picker.Item, error) {
	output, err := git.QueryContext(ctx,
		"for-each-ref",
		"--sort=-committerdate",
		"--format=%(refname:short)\t%(committerdate:unix)\t%(authorname)\t%(contents:subject)",
		"refs/remotes",
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list remote branches: %w", err)
	}

	items := []picker.Item{{
		Label: "➕ Create new branch",
		Value: createNewBranchValue,
	}}
	for _, candidate := range parseRemoteBranchCandidates(output, checkedOutBranches()) {
		items = append(items, candidate.pickerItem())
	}
	return items, nil
}

func canUseAddPreloadUI() bool {
	return ui.CanPrompt() && ui.StdoutTTY()
}

func fetchInteractiveBranchesQuiet(ctx context.Context) error {
	remotes, err := git.QueryContext(ctx, "remote")
	if err != nil || strings.TrimSpace(remotes) == "" {
		return nil
	}
	return git.RunToContext(ctx, io.Discard, "fetch", "--all", "--prune")
}

type addPreloadStatusMsg struct {
	phase   ui.AsyncPhase
	message string
}

type addPreloadDoneMsg struct {
	items []picker.Item
	err   error
}

type addPreloadModel struct {
	spinner spinner.Model
	phase   ui.AsyncPhase
	message string
	items   []picker.Item
	err     error
	ctx     context.Context
	cancel  context.CancelFunc
	send    func(tea.Msg)
}

func newAddPreloadModel(ctx context.Context) *addPreloadModel {
	baseCtx := ctx
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithCancel(baseCtx)
	return &addPreloadModel{
		spinner: spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
			spinner.WithStyle(ui.ForegroundStyle(ui.AccentColor())),
		),
		phase:   ui.AsyncLoading,
		message: "Fetching from all remotes…",
		ctx:     ctx,
		cancel:  cancel,
	}
}

func (m *addPreloadModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.load())
}

func (m *addPreloadModel) load() tea.Cmd {
	ctx := m.ctx
	return func() tea.Msg {
		if m.send != nil {
			m.send(addPreloadStatusMsg{phase: ui.AsyncLoading, message: "Fetching from all remotes…"})
		}
		if err := fetchInteractiveBranchesQuiet(ctx); err != nil {
			return addPreloadDoneMsg{err: err}
		}
		if m.send != nil {
			m.send(addPreloadStatusMsg{phase: ui.AsyncPartial, message: "Loading remote branches…"})
		}
		items, err := buildInteractiveAddItems(ctx)
		return addPreloadDoneMsg{items: items, err: err}
	}
}

func (m *addPreloadModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case addPreloadStatusMsg:
		m.phase = msg.phase
		m.message = msg.message
		return m, nil
	case addPreloadDoneMsg:
		m.items = msg.items
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

func (m *addPreloadModel) View() string {
	prefix := m.spinner.View()
	switch m.phase {
	case ui.AsyncReady:
		prefix = ui.Green("●")
	case ui.AsyncError, ui.AsyncCanceled:
		prefix = ui.Red("●")
	}
	return fmt.Sprintf("%s %s\n", prefix, m.message)
}

func runAddPreload(ctx context.Context) ([]picker.Item, error) {
	m := newAddPreloadModel(ctx)
	p := ui.NewProgram(m, os.Stderr)
	m.send = p.Send
	result, err := p.Run()
	m.cancel()
	if err != nil {
		return nil, err
	}
	r := result.(*addPreloadModel)
	if errors.Is(r.err, context.Canceled) {
		return nil, nil
	}
	return r.items, r.err
}
