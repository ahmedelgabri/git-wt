package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
)

func findRemovalCandidates(entries []worktree.Entry, filters removeFilters) ([]removalItem, error) {
	defaultBranch := worktree.DefaultBranch(worktree.DefaultRemote())
	currentRoot, _ := currentWorktreeRoot()

	candidates := make([]removalItem, 0)
	seenPrune := make(map[string]bool)
	for _, entry := range entries {
		reason, prune := pruneReason(entry)
		if prune {
			if filters.stale && !seenPrune[entry.Path] {
				seenPrune[entry.Path] = true
				candidates = append(candidates, removalItem{
					Action: removalActionPrune,
					Target: removalTarget{path: entry.Path, branch: entry.Branch, detached: entry.Detached},
					Reason: reason,
				})
			}
			continue
		}

		if !filters.merged && !filters.gone {
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

		if filters.gone {
			gone, err := upstreamGone(entry.Path, entry.Branch)
			if err != nil {
				return nil, err
			}
			if gone {
				candidates = append(candidates, removalItem{
					Action: removalActionRemove,
					Target: removalTarget{path: entry.Path, branch: entry.Branch},
					Reason: "upstream is gone",
				})
				continue
			}
		}

		if filters.merged && defaultBranch != "" && branchHasRemoteUpstream(entry.Branch) && branchMergedIntoDefault(entry.Branch, defaultBranch) {
			candidates = append(candidates, removalItem{
				Action: removalActionRemove,
				Target: removalTarget{path: entry.Path, branch: entry.Branch},
				Reason: fmt.Sprintf("fully merged into %s", defaultBranch),
			})
		}
	}

	slices.SortFunc(candidates, func(a, b removalItem) int {
		if a.Action != b.Action {
			if a.Action == removalActionRemove {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Target.path, b.Target.path)
	})
	return candidates, nil
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

func branchHasRemoteUpstream(branch string) bool {
	out, err := git.Query("for-each-ref", "--format=%(upstream)", "refs/heads/"+branch)
	if err != nil {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(out), "refs/remotes/")
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
