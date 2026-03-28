package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

type listEntry struct {
	Path     string
	Branch   string
	Head     string
	Detached bool
	Locked   bool
	Prunable bool
	Bare     bool
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all worktrees",
	Long: `List all worktrees. With no flags it renders a richer table view.

All native 'git worktree list' flags are still supported (e.g. --porcelain),
and will be passed through directly.

Use --json for structured output.`,
	FParseErrWhitelist: cobra.FParseErrWhitelist{UnknownFlags: true},
	SilenceUsage:       true,
	SilenceErrors:      true,
	RunE: func(cmd *cobra.Command, args []string) error {
		jsonOutput, _ := cmd.Flags().GetBool("json")
		if jsonOutput {
			if len(args) > 0 {
				return fmt.Errorf("--json cannot be combined with git worktree list flags")
			}
			entries, err := worktree.List()
			if err != nil {
				return err
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(entries)
		}

		if len(os.Args) > 2 {
			fullArgs := append([]string{"worktree", "list"}, os.Args[2:]...)
			return git.QueryRun(fullArgs...)
		}

		return renderStyledWorktreeList()
	},
}

func init() {
	listCmd.Flags().Bool("json", false, "Output worktrees as JSON")
	rootCmd.AddCommand(listCmd)
}

func renderStyledWorktreeList() error {
	raw, err := git.Query("worktree", "list", "--porcelain")
	if err != nil {
		return err
	}

	entries := parseListEntries(raw)
	if len(entries) == 0 {
		fmt.Println(ui.Subtle("No worktrees available"))
		return nil
	}

	currentRoot, _ := currentWorktreeRoot()
	slices.SortFunc(entries, func(a, b listEntry) int {
		switch {
		case a.Bare && !b.Bare:
			return -1
		case !a.Bare && b.Bare:
			return 1
		case samePath(a.Path, currentRoot) && !samePath(b.Path, currentRoot):
			return -1
		case !samePath(a.Path, currentRoot) && samePath(b.Path, currentRoot):
			return 1
		default:
			return strings.Compare(a.Path, b.Path)
		}
	})
	rows := make([][]string, 0, len(entries))
	worktreeCount := 0
	hasBare := false
	for _, entry := range entries {
		if entry.Bare {
			hasBare = true
		} else {
			worktreeCount++
		}
		rows = append(rows, []string{
			listWorkspaceName(entry, currentRoot),
			listBranchLabel(entry),
			listHeadLabel(entry),
			listFlags(entry, currentRoot),
			displayWorktreePath(entry.Path),
		})
	}

	summaryParts := []string{ui.Subtle(fmt.Sprintf("%d linked worktree(s)", worktreeCount))}
	if hasBare {
		summaryParts = append(summaryParts, ui.Accent("bare root present"))
	}
	fmt.Println(renderTableSection([]ui.TableColumn{
		{Title: "WORKTREE", MinWidth: 12, MaxWidth: 24},
		{Title: "BRANCH", MinWidth: 12, MaxWidth: 28},
		{Title: "HEAD", MinWidth: 8, MaxWidth: 8},
		{Title: "FLAGS", MinWidth: 8, MaxWidth: 24},
		{Title: "PATH", MinWidth: 20},
	}, rows, nil, strings.Join(summaryParts, " • ")))
	return nil
}

func parseListEntries(output string) []listEntry {
	if output == "" {
		return nil
	}

	var entries []listEntry
	var current listEntry
	for line := range strings.SplitSeq(output, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			sha := strings.TrimPrefix(line, "HEAD ")
			if len(sha) > 7 {
				sha = sha[:7]
			}
			current.Head = sha
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
			current.Detached = false
		case line == "detached":
			current.Branch = ""
			current.Detached = true
		case line == "bare":
			current.Bare = true
		case strings.HasPrefix(line, "locked"):
			current.Locked = true
		case strings.HasPrefix(line, "prunable"):
			current.Prunable = true
		case line == "":
			if current.Path != "" {
				current.Bare = current.Bare || filepath.Base(current.Path) == ".bare"
				entries = append(entries, current)
			}
			current = listEntry{}
		}
	}
	if current.Path != "" {
		current.Bare = current.Bare || filepath.Base(current.Path) == ".bare"
		entries = append(entries, current)
	}
	return entries
}

func listWorkspaceName(entry listEntry, currentRoot string) string {
	name := workspaceName(entry.Path)
	switch {
	case entry.Bare:
		return ui.Subtle(".bare")
	case samePath(entry.Path, currentRoot):
		return ui.Accent(name)
	default:
		return name
	}
}

func listBranchLabel(entry listEntry) string {
	switch {
	case entry.Bare:
		return ui.Subtle("git database")
	case entry.Detached:
		return "detached HEAD"
	case entry.Branch == "":
		return ui.Subtle("no branch")
	default:
		return entry.Branch
	}
}

func listHeadLabel(entry listEntry) string {
	if entry.Head == "" {
		return ui.Subtle("—")
	}
	return ui.Subtle(entry.Head)
}

func listFlags(entry listEntry, currentRoot string) string {
	flags := make([]string, 0, 4)
	if entry.Bare {
		flags = append(flags, ui.Accent("bare"))
	}
	if samePath(entry.Path, currentRoot) {
		flags = append(flags, ui.Accent("current"))
	}
	if entry.Detached {
		flags = append(flags, ui.Yellow("detached"))
	}
	if entry.Locked {
		flags = append(flags, ui.Yellow("locked"))
	}
	if entry.Prunable {
		flags = append(flags, ui.Yellow("prunable"))
	}
	if len(flags) == 0 {
		return ui.Subtle("—")
	}
	return strings.Join(flags, ", ")
}
