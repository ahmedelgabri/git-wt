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

// Cleanup has a branch default independent of the remote used by other commands.
// Return a full local ref so tags cannot shadow the selected branch.
func resolveCleanupBase(ctx context.Context) (string, error) {
	const remedy = "set an explicit local default, for example: git config wt.cleanupBase refs/heads/main"
	base, err := cleanupSetting(ctx, "wt.cleanupBase")
	if err != nil {
		return "", err
	}
	if base == "" {
		branch, err := git.QueryContext(ctx, "branch", "--show-current")
		if err != nil {
			return "", err
		}
		remote := ""
		if branch != "" {
			remote, err = cleanupSetting(ctx, "branch."+branch+".remote")
			if err != nil {
				return "", err
			}
		}
		if remote == "" {
			remote = worktree.DefaultRemoteInContext(ctx, "")
		}
		if remote == "." {
			return "", fmt.Errorf("cannot determine cleanup base for branch %q: branch.%s.remote is . and its HEAD is the current branch; %s", branch, branch, remedy)
		}
		// Raw URLs use network discovery with the same wt.remoteTimeout as
		// named remotes. An explicit local base above never needs the network.
		base = worktree.DefaultBranchInContext(ctx, "", remote)
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if base == "" {
			return "", fmt.Errorf("cannot determine cleanup base from remote %q; %s", remote, remedy)
		}
	}
	ref := base
	if !strings.HasPrefix(ref, "refs/heads/") {
		if strings.HasPrefix(ref, "refs/") {
			return "", fmt.Errorf("cleanup base %q must be a local branch; %s", base, remedy)
		}
		ref = "refs/heads/" + base
	}
	if _, err := git.QueryContext(ctx, "show-ref", "--verify", ref); err != nil {
		return "", fmt.Errorf("cleanup base %q is not an available local branch; %s: %w", base, remedy, err)
	}
	return ref, nil
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
