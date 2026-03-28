package cmd

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/picker"
	"github.com/ahmedelgabri/git-wt/internal/ui"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
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

func remoteBranchCandidatesToPickerItems(candidates []remoteBranchCandidate) []picker.Item {
	items := make([]picker.Item, 0, len(candidates)+1)
	items = append(items, picker.Item{
		Label: "➕ Create new branch",
		Value: createNewBranchValue,
	})
	for _, candidate := range candidates {
		items = append(items, candidate.pickerItem())
	}
	return items
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

	candidates := parseRemoteBranchCandidates(output, checkedOutBranches())
	return remoteBranchCandidatesToPickerItems(candidates), nil
}

func canUseAddPreloadUI() bool {
	return canUseSelectionPreloadUI()
}

func fetchInteractiveBranchesQuiet(ctx context.Context) error {
	remotes, err := git.QueryContext(ctx, "remote")
	if err != nil || strings.TrimSpace(remotes) == "" {
		return nil
	}
	return git.RunToContext(ctx, io.Discard, "fetch", "--all", "--prune")
}

func runAddPreload(ctx context.Context) ([]picker.Item, error) {
	return runPreload(ctx, "Fetching from all remotes…", func(ctx context.Context, update func(phase ui.AsyncPhase, message string)) ([]picker.Item, error) {
		update(ui.AsyncLoading, "Fetching from all remotes…")
		if err := fetchInteractiveBranchesQuiet(ctx); err != nil {
			return nil, err
		}
		update(ui.AsyncPartial, "Loading remote branches…")
		return buildInteractiveAddItems(ctx)
	})
}
