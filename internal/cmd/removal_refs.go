package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

type remoteDeletion struct {
	url  string
	head string
}

// Check every push destination before local removal. Fetch and push URLs need
// not expose the same tip, and each destination needs its own deletion lease.
func planRemoteDeletions(target removalTarget, branchHead string, force bool) ([]remoteDeletion, error) {
	out, err := git.QueryRaw("remote", "get-url", "--push", "--all", target.remote)
	if err != nil {
		return nil, err
	}
	var deletions []remoteDeletion
	seen := make(map[string]bool)
	for url := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if seen[url] {
			continue
		}
		seen[url] = true
		args := []string{"-c", "remote." + target.remote + ".url=", "-c", "remote." + target.remote + ".url=" + url}
		resolved, err := git.QueryRaw(append(args, "remote", "get-url", target.remote)...)
		if err != nil {
			return nil, err
		}
		if strings.TrimSuffix(resolved, "\n") != url {
			return nil, fmt.Errorf("URL rewrites change the push destination during verification; use native Git to review and delete this remote branch")
		}
		out, err := git.Query(append(args, "ls-remote", "--heads", target.remote, "refs/heads/"+target.remoteBranch)...)
		if err != nil {
			return nil, fmt.Errorf("check remote branch %s/%s at %s: %w", target.remote, target.remoteBranch, url, err)
		}
		head, _, _ := strings.Cut(out, "\t")
		if head != "" && !force {
			if _, err := git.Query("merge-base", "--is-ancestor", head, branchHead); err != nil {
				return nil, fmt.Errorf("remote branch at %s has commits not preserved by the selected local branch; fetch and review before removal", url)
			}
		}
		deletions = append(deletions, remoteDeletion{url: url, head: head})
	}
	return deletions, nil
}

func deleteLocalBranch(branch, expectedHead string) error {
	if out, err := git.RunWithOutput("update-ref", "-d", "refs/heads/"+branch, expectedHead); err != nil {
		return fmt.Errorf("delete local branch %s: %s: %w", branch, out, err)
	}
	// Like git branch -D, remove repository-local branch settings. Inherited
	// settings remain the user's defaults. A failed ref deletion never gets here.
	_, err := git.Query("config", "--local", "--get-regexp", `^branch\.`+regexp.QuoteMeta(branch)+`\.`)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		return nil
	}
	if err == nil {
		_, err = git.RunWithOutput("config", "--local", "--remove-section", "branch."+branch)
	}
	if err != nil {
		return fmt.Errorf("local branch %s deleted, but configuration cleanup failed: %w", branch, err)
	}
	return nil
}
