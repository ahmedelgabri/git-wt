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
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
)

type worktreeStatusSummary struct {
	Workspace  string
	Path       string
	Branch     string
	Dirty      bool
	Upstream   string
	Ahead      int
	Behind     int
	LastCommit string
	Flags      string
	Current    bool
}

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
		summaries := make([]worktreeStatusSummary, 0, len(entries))
		for _, entry := range entries {
			summary, err := summarizeWorktreeStatus(entry, currentRoot)
			if err != nil {
				return err
			}
			summaries = append(summaries, summary)
		}

		slices.SortFunc(summaries, compareStatusSummaries)
		return printWorktreeStatuses(summaries)
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func summarizeWorktreeStatus(entry worktree.Entry, currentRoot string) (worktreeStatusSummary, error) {
	statusOut, err := git.QueryIn(entry.Path, "status", "--porcelain=v2", "--branch")
	if err != nil {
		return worktreeStatusSummary{}, err
	}

	upstream, ahead, behind, dirty := parseBranchStatus(statusOut)
	lastCommit := "n/a"
	if ts, err := git.QueryIn(entry.Path, "log", "-1", "--format=%ct"); err == nil && ts != "" {
		lastCommit = humanizeCommitAge(ts)
	}

	branch := entry.Branch
	if entry.Detached {
		branch = "detached HEAD"
	}
	if branch == "" {
		branch = "no branch"
	}

	current := samePath(entry.Path, currentRoot)
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

	return worktreeStatusSummary{
		Workspace:  workspace,
		Path:       displayWorktreePath(entry.Path),
		Branch:     branch,
		Dirty:      dirty,
		Upstream:   upstream,
		Ahead:      ahead,
		Behind:     behind,
		LastCommit: lastCommit,
		Flags:      strings.Join(flags, ", "),
		Current:    current,
	}, nil
}

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

func printWorktreeStatuses(summaries []worktreeStatusSummary) error {
	rows := make([][]string, 0, len(summaries))
	dirtyCount := 0
	for _, summary := range summaries {
		if summary.Dirty {
			dirtyCount++
		}
		rows = append(rows, []string{
			summary.Workspace,
			summary.Branch,
			formatWorktreeState(summary.Dirty),
			formatSyncState(summary.Upstream, summary.Ahead, summary.Behind),
			summary.LastCommit,
			summary.Flags,
			summary.Path,
		})
	}

	body := ui.RenderTable([]ui.TableColumn{
		{Title: "WORKTREE", MinWidth: 12},
		{Title: "BRANCH", MinWidth: 12},
		{Title: "STATE", MinWidth: 10},
		{Title: "SYNC", MinWidth: 10},
		{Title: "LAST COMMIT", MinWidth: 11},
		{Title: "FLAGS", MinWidth: 8},
		{Title: "PATH", MinWidth: 20},
	}, rows)

	cleanCount := len(summaries) - dirtyCount
	summaryLine := strings.Join([]string{
		ui.Subtle(fmt.Sprintf("%d worktree(s)", len(summaries))),
		ui.Green(fmt.Sprintf("%d clean", cleanCount)),
		ui.Yellow(fmt.Sprintf("%d dirty", dirtyCount)),
	}, " • ")
	fmt.Println(ui.Section("", body, "", summaryLine))
	return nil
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

func compareStatusSummaries(a, b worktreeStatusSummary) int {
	switch {
	case a.Current && !b.Current:
		return -1
	case !a.Current && b.Current:
		return 1
	case a.Dirty && !b.Dirty:
		return -1
	case !a.Dirty && b.Dirty:
		return 1
	default:
		return strings.Compare(ansiLess(a.Workspace), ansiLess(b.Workspace))
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
