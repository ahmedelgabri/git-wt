package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
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
		if target.prunable {
			return fmt.Errorf("worktree %s is marked prunable but its path still exists; inspect its files and run git wt repair %s before removal", target.path, shellQuote(target.path))
		}
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
		base, err := resolveCleanupBase(context.Background())
		if err != nil {
			return err
		}
		if base == "refs/heads/"+target.branch {
			return fmt.Errorf("cannot remove cleanup base %s", base)
		}
		if !branchMergedIntoDefault(target.branch, base) {
			return fmt.Errorf("branch %s is no longer merged into cleanup base %s", target.branch, base)
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
	var exclusions strings.Builder
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
		fmt.Fprintf(&exclusions, "^%s\n", retained)
	}
	// Keep the symbolic-ref exclusions above without putting every retained
	// ref in argv. Large repositories can exceed the process argument limit.
	unique, err := git.QueryWithInput(strings.NewReader(exclusions.String()), "rev-list", "--max-count=1", ref, "--stdin")
	if err != nil {
		return err
	}
	if unique != "" {
		return fmt.Errorf("%s has commits without another retained branch or tag, including %s; use --force with an explicit target to discard them", target.path, unique)
	}
	return nil
}
