package cmd

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
	"github.com/ahmedelgabri/git-wt/internal/worktree"
)

type cleanupBase struct {
	ref             string
	protectedBranch string // May lag the remote-tracking ref.
}

// Cleanup has a branch default independent of the remote used by other commands.
// Full refs avoid tag ambiguity; explicit remote-tracking refs need no network.
func resolveCleanupBase(ctx context.Context) (cleanupBase, error) {
	const remedy = "set an explicit base, for example: git config wt.cleanupBase refs/heads/main or git config wt.cleanupBase refs/remotes/origin/main"
	base, err := cleanupSetting(ctx, "wt.cleanupBase")
	if err != nil {
		return cleanupBase{}, err
	}
	if base == "" {
		branch, err := git.QueryContext(ctx, "branch", "--show-current")
		if err != nil {
			return cleanupBase{}, err
		}
		remote := ""
		if branch != "" {
			remote, err = cleanupSetting(ctx, "branch."+branch+".remote")
			if err != nil {
				return cleanupBase{}, err
			}
		}
		if remote == "" {
			remote = worktree.DefaultRemoteInContext(ctx, "")
		}
		if remote == "." {
			return cleanupBase{}, fmt.Errorf("cannot determine cleanup base for branch %q: branch.%s.remote is . and its HEAD is the current branch; %s", branch, branch, remedy)
		}
		base = worktree.DefaultBranchInContext(ctx, "", remote)
		if err := ctx.Err(); err != nil {
			return cleanupBase{}, err
		}
		if base == "" {
			return cleanupBase{}, fmt.Errorf("cannot determine cleanup base from remote %q; %s", remote, remedy)
		}
	}
	ref := base
	if !strings.HasPrefix(ref, "refs/") {
		ref = "refs/heads/" + ref
	}
	if strings.HasPrefix(ref, "refs/remotes/") {
		// In particular, origin/HEAD must protect main, not a branch named HEAD.
		resolved, err := git.QueryContext(ctx, "symbolic-ref", "--quiet", ref)
		var exitErr *exec.ExitError
		if err == nil {
			ref = resolved
		} else if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
			return cleanupBase{}, fmt.Errorf("resolve cleanup base %q: %w", base, err)
		}
	}
	if !strings.HasPrefix(ref, "refs/heads/") && !strings.HasPrefix(ref, "refs/remotes/") {
		return cleanupBase{}, fmt.Errorf("cleanup base %q must be a local branch or remote-tracking branch; %s", base, remedy)
	}
	if _, err := git.QueryContext(ctx, "show-ref", "--verify", ref); err != nil {
		return cleanupBase{}, fmt.Errorf("cleanup base %q is not an available local branch or remote-tracking branch; %s: %w", base, remedy, err)
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	if name, remote := strings.CutPrefix(ref, "refs/remotes/"); remote {
		_, branch, _ = strings.Cut(name, "/")
		if branch == "" {
			return cleanupBase{}, fmt.Errorf("cleanup base %q must have the form refs/remotes/<remote>/<branch>; %s", base, remedy)
		}
		// Use the longest configured remote prefix, since remote names may
		// contain slashes. Otherwise retain the conventional first-component split.
		remotes, err := git.QueryContext(ctx, "remote")
		if err != nil {
			return cleanupBase{}, err
		}
		longest := 0
		for remote := range strings.SplitSeq(remotes, "\n") {
			if suffix, ok := strings.CutPrefix(name, remote+"/"); ok && len(remote) > longest {
				branch, longest = suffix, len(remote)
			}
		}
	}
	return cleanupBase{ref: ref, protectedBranch: branch}, nil
}

func cleanupSetting(ctx context.Context, key string) (string, error) {
	value, err := git.QueryContext(ctx, "config", "--get", key)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", key, err)
	}
	return value, nil
}
