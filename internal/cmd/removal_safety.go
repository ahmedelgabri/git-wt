package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
)

func removalUpstream(branch string) (remote, branchName, trackingRef string) {
	if branch == "" {
		return
	}
	out, err := git.Query("for-each-ref", "--format=%(upstream:remotename)\t%(upstream:remoteref)\t%(upstream)", "refs/heads/"+branch)
	if err != nil {
		return
	}
	fields := strings.Split(out, "\t")
	if len(fields) != 3 || fields[0] == "." || !strings.HasPrefix(fields[1], "refs/heads/") {
		return
	}
	return fields[0], strings.TrimPrefix(fields[1], "refs/heads/"), fields[2]
}

func validateRemovalSafety(target removalTarget, deleteRemote, cleanup bool) error {
	if _, err := os.Stat(target.path); err == nil {
		dirty, err := worktreeDirty(target.path)
		if err != nil {
			return err
		}
		if dirty {
			return fmt.Errorf("worktree %s contains local files or changes; use --force with an explicit target to discard them", target.path)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	var err error
	if cleanup {
		defaultBranch := worktree.DefaultBranch(worktree.DefaultRemote())
		if defaultBranch == "" || !branchMergedIntoDefault(target.branch, defaultBranch) {
			return fmt.Errorf("branch %s is no longer merged into the default branch", target.branch)
		}
	}
	ref := "refs/heads/" + target.branch
	if target.detached {
		ref, err = git.QueryIn(target.path, "rev-parse", "HEAD")
		if err != nil {
			return err
		}
	}
	refs, err := git.Query("for-each-ref", "--format=%(refname)\t%(symref)", "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return err
	}
	args := []string{"rev-list", "--max-count=1", ref, "--not"}
	for _, line := range strings.Split(refs, "\n") {
		retained, symbolic, _ := strings.Cut(line, "\t")
		if symbolic != "" {
			continue
		}
		if retained == "" || retained == ref {
			continue
		}
		if deleteRemote && retained == target.upstreamRef {
			continue
		}
		args = append(args, retained)
	}
	unique, err := git.Query(args...)
	if err != nil {
		return err
	}
	if unique != "" {
		return fmt.Errorf("%s has commits without another retained branch or tag, including %s; use --force with an explicit target to discard them", target.path, unique)
	}
	return nil
}
