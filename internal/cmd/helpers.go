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
// exact duplicates of remote-tracking refs, while preserving local branches
// whose commits differ from the remote.
func cleanupLocalBranchRefs(dir string) {
	localRefs, _ := git.QueryIn(dir, "for-each-ref", "--format=%(refname:short)\t%(objectname)", "refs/heads")
	if localRefs == "" {
		return
	}

	remoteRefs, _ := git.QueryIn(dir, "for-each-ref", "--format=%(refname:short)\t%(objectname)", "refs/remotes")
	remoteBranches := make(map[string]map[string]bool)
	for line := range strings.SplitSeq(remoteRefs, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		remoteRef, oid, ok := strings.Cut(line, "\t")
		if !ok || remoteRef == "" || oid == "" || strings.HasSuffix(remoteRef, "/HEAD") {
			continue
		}
		if _, branch, ok := strings.Cut(remoteRef, "/"); ok && branch != "" {
			if remoteBranches[branch] == nil {
				remoteBranches[branch] = make(map[string]bool)
			}
			remoteBranches[branch][oid] = true
		}
	}

	for line := range strings.SplitSeq(localRefs, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		ref, oid, ok := strings.Cut(line, "\t")
		if !ok || ref == "" || oid == "" || !remoteBranches[ref][oid] {
			continue
		}
		git.RunInWithOutput(dir, "branch", "-D", ref)
	}
}
