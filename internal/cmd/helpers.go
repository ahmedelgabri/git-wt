package cmd

import (
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

// configureBareRepo sets the git config keys needed after creating a bare repo
// structure (.bare/ + .git file).
func configureBareRepo(dir string) error {
	if _, err := git.RunInWithOutput(dir, "config", "remote.origin.fetch", "+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return err
	}
	if _, err := git.RunInWithOutput(dir, "config", "core.logallrefupdates", "true"); err != nil {
		return err
	}
	if _, err := git.RunInWithOutput(dir, "config", "worktree.useRelativePaths", "true"); err != nil {
		return err
	}
	return nil
}

// cleanupLocalBranchRefs removes local branch refs that a bare clone creates as
// copies of remote branches, while preserving branches that exist only locally.
func cleanupLocalBranchRefs(dir string) {
	refs, _ := git.QueryIn(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if refs == "" {
		return
	}

	remoteRefs, _ := git.QueryIn(dir, "for-each-ref", "--format=%(refname:short)", "refs/remotes")
	remoteBranches := make(map[string]bool)
	for remoteRef := range strings.SplitSeq(remoteRefs, "\n") {
		remoteRef = strings.TrimSpace(remoteRef)
		if remoteRef == "" || strings.HasSuffix(remoteRef, "/HEAD") {
			continue
		}
		if _, branch, ok := strings.Cut(remoteRef, "/"); ok && branch != "" {
			remoteBranches[branch] = true
		}
	}

	for ref := range strings.SplitSeq(refs, "\n") {
		ref = strings.TrimSpace(ref)
		if ref == "" || !remoteBranches[ref] {
			continue
		}
		git.RunInWithOutput(dir, "branch", "-D", ref)
	}
}
