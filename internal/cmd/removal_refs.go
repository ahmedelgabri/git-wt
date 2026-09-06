package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

type remoteDeletion struct {
	url         string
	head        string
	overrideURL bool
}

func remoteDeletionDestinations(remote string) ([]remoteDeletion, error) {
	out, err := git.QueryRaw("remote", "get-url", "--push", "--all", remote)
	if err != nil {
		return nil, err
	}
	urls := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	fetch, err := git.QueryRaw("remote", "get-url", remote)
	if err != nil {
		return nil, err
	}
	overrideURL := len(urls) != 1 || urls[0] != strings.TrimSuffix(fetch, "\n")
	if overrideURL {
		reason := "has differing fetch and push URLs"
		if len(urls) > 1 {
			reason = "has multiple push URLs"
		}
		version, err := git.Query("--version")
		if err != nil {
			return nil, fmt.Errorf("remote %s %s; cannot check the Git 2.46 minimum for destination selection: %w", remote, reason, err)
		}
		if !supportsRemoteURLReset(version) {
			return nil, fmt.Errorf("remote %s %s; destination selection requires Git 2.46 or newer, found %q; refusing before local removal to avoid partial completion. Upgrade Git on PATH, remove without --delete-remote, or review and delete the remote branch with native Git", remote, reason, version)
		}
	}
	var deletions []remoteDeletion
	seen := make(map[string]bool)
	for _, url := range urls {
		if !seen[url] {
			deletions = append(deletions, remoteDeletion{url: url, overrideURL: overrideURL})
			seen[url] = true
		}
	}
	return deletions, nil
}

func supportsRemoteURLReset(version string) bool {
	fields := strings.Fields(version)
	if len(fields) < 3 || fields[0] != "git" || fields[1] != "version" {
		return false
	}
	parts := strings.Split(fields[2], ".")
	if len(parts) < 2 {
		return false
	}
	major, majorErr := strconv.Atoi(parts[0])
	minor, minorErr := strconv.Atoi(parts[1])
	return majorErr == nil && minorErr == nil && minor >= 0 && (major > 2 || major == 2 && minor >= 46)
}

// Check every push destination before local removal. Repeat URL discovery after
// hooks, since a hook may have changed the configuration checked by the plan.
func planRemoteDeletions(target removalTarget, branchHead string, force bool) ([]remoteDeletion, error) {
	deletions, err := remoteDeletionDestinations(target.remote)
	if err != nil {
		return nil, err
	}
	for i := range deletions {
		deletion := &deletions[i]
		var args []string
		if deletion.overrideURL {
			args = []string{"-c", "remote." + target.remote + ".url=", "-c", "remote." + target.remote + ".url=" + deletion.url}
			resolved, err := git.QueryRaw(append(args, "remote", "get-url", target.remote)...)
			if err != nil {
				return nil, err
			}
			if strings.TrimSuffix(resolved, "\n") != deletion.url {
				return nil, fmt.Errorf("URL rewrites change the push destination during verification; use native Git to review and delete this remote branch")
			}
		}
		out, err := git.Query(append(args, "ls-remote", "--heads", target.remote, "refs/heads/"+target.remoteBranch)...)
		if err != nil {
			return nil, fmt.Errorf("check remote branch %s/%s at %s: %w", target.remote, target.remoteBranch, deletion.url, err)
		}
		deletion.head, _, _ = strings.Cut(out, "\t")
		if deletion.head != "" && !force {
			if _, err := git.Query("merge-base", "--is-ancestor", deletion.head, branchHead); err != nil {
				return nil, fmt.Errorf("remote branch at %s has commits not preserved by the selected local branch; fetch and review before removal", deletion.url)
			}
		}
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
