package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/ahmedelgabri/git-wt/internal/git"
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
			fmt.Println("No worktrees available")
			return nil
		}

		summaries := make([]worktreeStatusSummary, 0, len(entries))
		for _, entry := range entries {
			summary, err := summarizeWorktreeStatus(entry)
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

func summarizeWorktreeStatus(entry worktree.Entry) (worktreeStatusSummary, error) {
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

	return worktreeStatusSummary{
		Workspace:  workspaceName(entry.Path),
		Path:       displayPath(entry.Path),
		Branch:     branch,
		Dirty:      dirty,
		Upstream:   upstream,
		Ahead:      ahead,
		Behind:     behind,
		LastCommit: lastCommit,
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
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "WORKTREE\tBRANCH\tSTATE\tSYNC\tLAST COMMIT\tPATH"); err != nil {
		return err
	}
	for _, summary := range summaries {
		state := "clean"
		if summary.Dirty {
			state = "dirty"
		}
		sync := "local"
		if summary.Upstream != "" {
			sync = fmt.Sprintf("+%d/-%d", summary.Ahead, summary.Behind)
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			summary.Workspace,
			summary.Branch,
			state,
			sync,
			summary.LastCommit,
			summary.Path,
		); err != nil {
			return err
		}
	}
	return tw.Flush()
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
