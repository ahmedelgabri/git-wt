package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
	"github.com/spf13/cobra"
)

type cleanAction string

const (
	cleanActionRemove cleanAction = "remove"
	cleanActionPrune  cleanAction = "prune"
)

type cleanCandidate struct {
	Action cleanAction
	Target removalTarget
	Reason string
}

var cleanCmd = &cobra.Command{
	Use:           "clean",
	Short:         "Clean safe worktree cleanup candidates",
	SilenceUsage:  true,
	SilenceErrors: true,
	RunE:          runClean,
}

func init() {
	cleanCmd.Flags().BoolP("dry-run", "n", false, "Preview what would be cleaned without making changes")
	rootCmd.AddCommand(cleanCmd)
}

func runClean(cmd *cobra.Command, args []string) error {
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	entries, err := worktree.List()
	if err != nil {
		return err
	}

	candidates, err := findCleanCandidates(entries)
	if err != nil {
		return err
	}
	slices.SortFunc(candidates, func(a, b cleanCandidate) int {
		if a.Action != b.Action {
			if a.Action == cleanActionRemove {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Target.path, b.Target.path)
	})
	if len(candidates) == 0 {
		fmt.Println(ui.Subtle("No cleanable worktrees found"))
		return nil
	}

	fmt.Println(renderCleanCandidates(candidates))

	if dryRun {
		fmt.Printf("%s No changes made\n", ui.Yellow("[DRY RUN]"))
		return nil
	}

	if !ui.Confirm("Continue cleaning these worktrees? [y/N]:") {
		fmt.Println("Cancelled")
		return nil
	}

	return executeClean(candidates)
}

func renderCleanCandidates(candidates []cleanCandidate) string {
	rows := make([][]string, 0, len(candidates))
	removeCount, pruneCount := 0, 0
	for _, candidate := range candidates {
		switch candidate.Action {
		case cleanActionRemove:
			removeCount++
		case cleanActionPrune:
			pruneCount++
		}
		rows = append(rows, []string{
			renderCleanAction(candidate.Action),
			filepath.Base(candidate.Target.path),
			candidate.Target.branchLabel(),
			candidate.Reason,
		})
	}

	body := ui.RenderTable([]ui.TableColumn{
		{Title: "ACTION", MinWidth: 8, MaxWidth: 10},
		{Title: "WORKTREE", MinWidth: 12, MaxWidth: 24},
		{Title: "BRANCH", MinWidth: 12, MaxWidth: 24},
		{Title: "REASON", MinWidth: 24, MaxWidth: 72},
	}, rows)
	summary := strings.Join([]string{
		ui.Subtle(fmt.Sprintf("%d candidate(s)", len(candidates))),
		ui.Red(fmt.Sprintf("%d remove", removeCount)),
		ui.Yellow(fmt.Sprintf("%d prune", pruneCount)),
	}, " • ")
	return ui.Section("Cleanup candidates", body, summary)
}

func renderCleanAction(action cleanAction) string {
	switch action {
	case cleanActionPrune:
		return ui.Yellow("prune")
	default:
		return ui.Red("remove")
	}
}

func findCleanCandidates(entries []worktree.Entry) ([]cleanCandidate, error) {
	defaultBranch := worktree.DefaultBranch(worktree.DefaultRemote())
	currentRoot, _ := currentWorktreeRoot()

	candidates := make([]cleanCandidate, 0)
	seenPrune := make(map[string]bool)
	for _, entry := range entries {
		reason, prune := pruneReason(entry)
		if prune {
			if !seenPrune[entry.Path] {
				seenPrune[entry.Path] = true
				candidates = append(candidates, cleanCandidate{
					Action: cleanActionPrune,
					Target: removalTarget{path: entry.Path, branch: entry.Branch, detached: entry.Detached},
					Reason: reason,
				})
			}
			continue
		}

		if entry.Locked || entry.Detached || entry.Branch == "" {
			continue
		}
		if defaultBranch != "" && entry.Branch == defaultBranch {
			continue
		}
		if samePath(entry.Path, currentRoot) {
			continue
		}

		dirty, err := worktreeDirty(entry.Path)
		if err != nil {
			return nil, err
		}
		if dirty {
			continue
		}

		if gone, err := upstreamGone(entry.Path, entry.Branch); err != nil {
			return nil, err
		} else if gone {
			candidates = append(candidates, cleanCandidate{
				Action: cleanActionRemove,
				Target: removalTarget{path: entry.Path, branch: entry.Branch},
				Reason: "upstream is gone",
			})
			continue
		}

		if defaultBranch != "" && branchMergedIntoDefault(entry.Branch, defaultBranch) {
			candidates = append(candidates, cleanCandidate{
				Action: cleanActionRemove,
				Target: removalTarget{path: entry.Path, branch: entry.Branch},
				Reason: fmt.Sprintf("fully merged into %s", defaultBranch),
			})
		}
	}

	return candidates, nil
}

func executeClean(candidates []cleanCandidate) error {
	var removeTargets []removalTarget
	var pruneTargets []removalTarget
	for _, candidate := range candidates {
		switch candidate.Action {
		case cleanActionRemove:
			removeTargets = append(removeTargets, candidate.Target)
		case cleanActionPrune:
			pruneTargets = append(pruneTargets, candidate.Target)
		}
	}

	if len(removeTargets) > 0 {
		if err := executeRemoval(removeTargets, "remove", ""); err != nil {
			return err
		}
	}

	for _, target := range pruneTargets {
		name := filepath.Base(target.path)
		if err := ui.SpinWithOutput(fmt.Sprintf("Pruning stale metadata for %s", ui.Accent(name)), func(w io.Writer) error {
			return git.RunTo(w, "worktree", "remove", "--force", target.path)
		}); err != nil {
			return err
		}
	}

	return nil
}

func pruneReason(entry worktree.Entry) (string, bool) {
	if entry.Prunable {
		reason := entry.PrunableReason
		if reason == "" {
			reason = "prunable metadata"
		}
		return reason, true
	}
	if _, err := os.Stat(entry.Path); err != nil {
		return "missing worktree path", true
	}
	return "", false
}

func worktreeDirty(path string) (bool, error) {
	out, err := git.QueryIn(path, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "", nil
}

func upstreamGone(path, branch string) (bool, error) {
	out, err := git.QueryIn(path, "for-each-ref", "--format=%(upstream:track)", "refs/heads/"+branch)
	if err != nil {
		return false, err
	}
	return strings.Contains(out, "[gone]"), nil
}

func branchMergedIntoDefault(branch, defaultBranch string) bool {
	_, err := git.Query("merge-base", "--is-ancestor", branch, defaultBranch)
	return err == nil
}

func currentWorktreeRoot() (string, error) {
	root, err := git.Query("rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return filepath.EvalSymlinks(root)
}

func samePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ra, errA := filepath.EvalSymlinks(a)
	rb, errB := filepath.EvalSymlinks(b)
	if errA != nil || errB != nil {
		return a == b
	}
	return ra == rb
}
