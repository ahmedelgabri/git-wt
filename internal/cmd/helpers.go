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

// cleanupLocalBranchRefs removes all local branch refs that a bare clone creates
// as copies of remote branches.
func cleanupLocalBranchRefs(dir string) {
	refs, _ := git.QueryIn(dir, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	if refs == "" {
		return
	}
	for ref := range strings.SplitSeq(refs, "\n") {
		ref = strings.TrimSpace(ref)
		if ref != "" {
			git.RunInWithOutput(dir, "branch", "-D", ref)
		}
	}
}
