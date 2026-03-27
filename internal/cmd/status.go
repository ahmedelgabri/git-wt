package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// ---------- row ----------

type statusRow struct {
	entryPath string // absolute worktree path (for git commands)
	workspace string
	branch    string
	path      string // display path
	flags     string
	current   bool

	// populated asynchronously (or synchronously for non-TTY)
	loaded     bool
	dirty      bool
	upstream   string
	ahead      int
	behind     int
	lastCommit string
	fetchErr   error
}

// ---------- command ----------

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show a compact status dashboard for all worktrees",
	Long: `Show a repository-wide dashboard for linked worktrees.

The dashboard includes branch name, clean/dirty state, upstream sync status,
last commit age, and a repo-relative path for each worktree.`,
	Example:       `  git wt status`,
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		entries, err := worktree.List()
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			fmt.Println(ui.Subtle("No worktrees available"))
			return nil
		}

		currentRoot, _ := currentWorktreeRoot()
		rows := buildStatusRows(entries, currentRoot)

		if !term.IsTerminal(int(os.Stdout.Fd())) {
			return runStatusSync(rows)
		}
		return runStatusAsync(rows)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

// ---------- shared row building ----------

func buildStatusRows(entries []worktree.Entry, currentRoot string) []statusRow {
	rows := make([]statusRow, len(entries))
	for i, entry := range entries {
		current := samePath(entry.Path, currentRoot)
		branch := entry.Branch
		if entry.Detached {
			branch = "detached HEAD"
		}
		if branch == "" {
			branch = "no branch"
		}

		flags := make([]string, 0, 2)
		if current {
			flags = append(flags, ui.Accent("current"))
		}
		if entry.Locked {
			flags = append(flags, ui.Yellow("locked"))
		}
		if len(flags) == 0 {
			flags = append(flags, ui.Subtle("—"))
		}

		workspace := workspaceName(entry.Path)
		if current {
			workspace = ui.Accent(workspace)
		}

		rows[i] = statusRow{
			entryPath: entry.Path,
			workspace: workspace,
			branch:    branch,
			path:      displayWorktreePath(entry.Path),
			flags:     strings.Join(flags, ", "),
			current:   current,
		}
	}

	slices.SortFunc(rows, func(a, b statusRow) int {
		switch {
		case a.current && !b.current:
			return -1
		case !a.current && b.current:
			return 1
		default:
			return strings.Compare(ansiLess(a.workspace), ansiLess(b.workspace))
		}
	})

	return rows
}

func statusColumns() []ui.TableColumn {
	return []ui.TableColumn{
		{Title: "WORKTREE", MinWidth: 12},
		{Title: "BRANCH", MinWidth: 12},
		{Title: "STATE", MinWidth: 10},
		{Title: "SYNC", MinWidth: 10},
		{Title: "LAST COMMIT", MinWidth: 11},
		{Title: "FLAGS", MinWidth: 8},
		{Title: "PATH", MinWidth: 20},
	}
}

func statusSummaryLine(total, clean, dirty, errors int) string {
	parts := []string{
		ui.Subtle(fmt.Sprintf("%d worktree(s)", total)),
		ui.Green(fmt.Sprintf("%d clean", clean)),
		ui.Yellow(fmt.Sprintf("%d dirty", dirty)),
	}
	if errors > 0 {
		parts = append(parts, ui.Red(fmt.Sprintf("%d error", errors)))
	}
	return strings.Join(parts, " • ")
}

func statusCounts(rows []statusRow) (clean, dirty, errors int) {
	for _, row := range rows {
		switch {
		case row.fetchErr != nil:
			errors++
		case row.dirty:
			dirty++
		default:
			clean++
		}
	}
	return clean, dirty, errors
}

// ---------- sync path (non-TTY / piped) ----------

func runStatusSync(rows []statusRow) error {
	for i := range rows {
		fetchStatusInto(&rows[i])
	}
	return printStatusTable(rows)
}

func fetchStatusInto(row *statusRow) {
	statusOut, err := git.QueryIn(row.entryPath, "status", "--porcelain=v2", "--branch")
	if err != nil {
		row.loaded = true
		row.fetchErr = err
		return
	}
	upstream, ahead, behind, dirty := parseBranchStatus(statusOut)
	lastCommit := "n/a"
	if ts, err := git.QueryIn(row.entryPath, "log", "-1", "--format=%ct"); err == nil && ts != "" {
		lastCommit = humanizeCommitAge(ts)
	}
	row.loaded = true
	row.dirty = dirty
	row.upstream = upstream
	row.ahead = ahead
	row.behind = behind
	row.lastCommit = lastCommit
}

func printStatusTable(rows []statusRow) error {
	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		state := formatWorktreeState(row.dirty)
		sync := formatSyncState(row.upstream, row.ahead, row.behind)
		commit := row.lastCommit
		if row.fetchErr != nil {
			state = ui.Red("● error")
			sync = ui.Subtle("—")
			commit = ui.Subtle("—")
		}
		tableRows = append(tableRows, []string{
			row.workspace,
			row.branch,
			state,
			sync,
			commit,
			row.flags,
			row.path,
		})
	}

	body := ui.RenderTable(statusColumns(), tableRows)
	cleanCount, dirtyCount, errorCount := statusCounts(rows)
	summary := statusSummaryLine(len(rows), cleanCount, dirtyCount, errorCount)
	fmt.Println(ui.Section("", body, "", summary))
	return nil
}

// ---------- async path (TTY with bubbletea) ----------

type statusResultMsg struct {
	index      int
	dirty      bool
	upstream   string
	ahead      int
	behind     int
	lastCommit string
	err        error
}

type statusModel struct {
	spinner spinner.Model
	rows    []statusRow
	pending int
	err     error
}

func runStatusAsync(rows []statusRow) error {
	m := statusModel{
		spinner: spinner.New(
			spinner.WithSpinner(spinner.MiniDot),
			spinner.WithStyle(lipgloss.NewStyle().Foreground(ui.SubtleColor())),
		),
		rows:    rows,
		pending: len(rows),
	}
	p := tea.NewProgram(m)
	result, err := p.Run()
	if err != nil {
		return err
	}
	if sm, ok := result.(statusModel); ok && sm.err != nil {
		return sm.err
	}
	return nil
}

func (m statusModel) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.rows)+1)
	cmds = append(cmds, m.spinner.Tick)
	for i, row := range m.rows {
		cmds = append(cmds, fetchWorktreeStatusCmd(i, row.entryPath))
	}
	return tea.Batch(cmds...)
}

func fetchWorktreeStatusCmd(index int, path string) tea.Cmd {
	return func() tea.Msg {
		statusOut, err := git.QueryIn(path, "status", "--porcelain=v2", "--branch")
		if err != nil {
			return statusResultMsg{index: index, err: err}
		}
		upstream, ahead, behind, dirty := parseBranchStatus(statusOut)
		lastCommit := "n/a"
		if ts, err := git.QueryIn(path, "log", "-1", "--format=%ct"); err == nil && ts != "" {
			lastCommit = humanizeCommitAge(ts)
		}
		return statusResultMsg{
			index:      index,
			dirty:      dirty,
			upstream:   upstream,
			ahead:      ahead,
			behind:     behind,
			lastCommit: lastCommit,
		}
	}
}

func (m statusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusResultMsg:
		m.rows[msg.index].loaded = true
		m.pending--
		if msg.err != nil {
			m.rows[msg.index].fetchErr = msg.err
		} else {
			m.rows[msg.index].dirty = msg.dirty
			m.rows[msg.index].upstream = msg.upstream
			m.rows[msg.index].ahead = msg.ahead
			m.rows[msg.index].behind = msg.behind
			m.rows[msg.index].lastCommit = msg.lastCommit
		}
		if m.pending == 0 {
			return m, tea.Quit
		}
		return m, nil
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
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

func (m statusModel) View() string {
	tableRows := make([][]string, 0, len(m.rows))
	for _, row := range m.rows {
		tableRows = append(tableRows, []string{
			row.workspace,
			row.branch,
			m.stateCell(row),
			m.syncCell(row),
			m.commitCell(row),
			row.flags,
			row.path,
		})
	}

	body := ui.RenderTable(statusColumns(), tableRows)

	if m.pending == 0 {
		cleanCount, dirtyCount, errorCount := statusCounts(m.rows)
		summary := statusSummaryLine(len(m.rows), cleanCount, dirtyCount, errorCount)
		return ui.Section("", body, "", summary) + "\n"
	}

	loaded := len(m.rows) - m.pending
	progress := ui.Subtle(fmt.Sprintf("loading %d/%d…", loaded, len(m.rows)))
	return ui.Section("", body, "", progress) + "\n"
}

func (m statusModel) stateCell(row statusRow) string {
	if !row.loaded {
		return m.spinner.View() + ui.Subtle(" …")
	}
	if row.fetchErr != nil {
		return ui.Red("● error")
	}
	return formatWorktreeState(row.dirty)
}

func (m statusModel) syncCell(row statusRow) string {
	if !row.loaded {
		return m.spinner.View() + ui.Subtle(" …")
	}
	if row.fetchErr != nil {
		return ui.Subtle("—")
	}
	return formatSyncState(row.upstream, row.ahead, row.behind)
}

func (m statusModel) commitCell(row statusRow) string {
	if !row.loaded {
		return ui.Subtle("…")
	}
	if row.fetchErr != nil {
		return ui.Subtle("—")
	}
	return row.lastCommit
}

// ---------- formatting helpers ----------

func parseBranchStatus(output string) (upstream string, ahead, behind int, dirty bool) {
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "# branch.upstream "):
			upstream = strings.TrimPrefix(line, "# branch.upstream ")
		case strings.HasPrefix(line, "# branch.ab "):
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				ahead, _ = strconv.Atoi(strings.TrimPrefix(fields[2], "+"))
				behind, _ = strconv.Atoi(strings.TrimPrefix(fields[3], "-"))
			}
		case !strings.HasPrefix(line, "# "):
			dirty = true
		}
	}
	return upstream, ahead, behind, dirty
}

func formatWorktreeState(dirty bool) string {
	if dirty {
		return ui.Yellow("● dirty")
	}
	return ui.Green("● clean")
}

func formatSyncState(upstream string, ahead, behind int) string {
	if upstream == "" {
		return ui.Subtle("local only")
	}
	switch {
	case ahead == 0 && behind == 0:
		return ui.Green("✓ synced")
	case ahead > 0 && behind > 0:
		return ui.Yellow(fmt.Sprintf("↑%d ↓%d", ahead, behind))
	case ahead > 0:
		return ui.Accent(fmt.Sprintf("↑%d ahead", ahead))
	default:
		return ui.Yellow(fmt.Sprintf("↓%d behind", behind))
	}
}

func displayWorktreePath(path string) string {
	if bareRoot, err := worktree.BareRoot(); err == nil && bareRoot != "" {
		if rel, err := filepath.Rel(bareRoot, path); err == nil && rel != "" && !strings.HasPrefix(rel, "..") {
			if rel == "." {
				return ui.Path(".")
			}
			return ui.Path("./" + rel)
		}
	}
	return ui.Path(displayPath(path))
}

func ansiLess(s string) string {
	return ansi.Strip(s)
}

func workspaceName(path string) string {
	if bareRoot, err := worktree.BareRoot(); err == nil && bareRoot != "" {
		if rel, err := filepath.Rel(bareRoot, path); err == nil {
			return rel
		}
	}
	return filepath.Base(path)
}

func displayPath(path string) string {
	homeDir, _ := os.UserHomeDir()
	if homeDir != "" {
		return strings.Replace(path, homeDir, "~", 1)
	}
	return path
}

func humanizeCommitAge(ts string) string {
	unix, err := strconv.ParseInt(strings.TrimSpace(ts), 10, 64)
	if err != nil || unix <= 0 {
		return "n/a"
	}
	d := time.Since(time.Unix(unix, 0)).Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}
