package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
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
}

var statusCmd = &cobra.Command{
	Use:           "status",
	Short:         "Show a compact status dashboard for all worktrees",
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

	flags := make([]string, 0, 2)
	if samePath(entry.Path, currentRoot) {
		flags = append(flags, ui.Accent("current"))
	}
	if entry.Locked {
		flags = append(flags, ui.Yellow("locked"))
	}
	if len(flags) == 0 {
		flags = append(flags, ui.Subtle("—"))
	}

	return worktreeStatusSummary{
		Workspace:  workspaceName(entry.Path),
		Path:       displayPath(entry.Path),
		Branch:     branch,
		Dirty:      dirty,
		Upstream:   upstream,
		Ahead:      ahead,
		Behind:     behind,
		LastCommit: lastCommit,
		Flags:      strings.Join(flags, ", "),
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
		{Title: "WORKTREE", MinWidth: 12, MaxWidth: 24},
		{Title: "BRANCH", MinWidth: 12, MaxWidth: 26},
		{Title: "STATE", MinWidth: 10, MaxWidth: 12},
		{Title: "SYNC", MinWidth: 10, MaxWidth: 18},
		{Title: "LAST COMMIT", MinWidth: 11, MaxWidth: 14},
		{Title: "FLAGS", MinWidth: 8, MaxWidth: 18},
		{Title: "PATH", MinWidth: 20, MaxWidth: 48},
	}, rows)

	cleanCount := len(summaries) - dirtyCount
	summaryLine := ui.Subtle(fmt.Sprintf("%d worktree(s) • %d clean • %d dirty", len(summaries), cleanCount, dirtyCount))
	fmt.Println(ui.Section("Worktree status", body, summaryLine))
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
		return ui.Subtle("local")
	}
	switch {
	case ahead == 0 && behind == 0:
		return ui.Green("✓ up to date")
	case ahead > 0 && behind > 0:
		return ui.Yellow(fmt.Sprintf("↑%d ↓%d", ahead, behind))
	case ahead > 0:
		return ui.Accent(fmt.Sprintf("↑%d", ahead))
	default:
		return ui.Yellow(fmt.Sprintf("↓%d", behind))
	}
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
