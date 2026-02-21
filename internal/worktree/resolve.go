package worktree

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

// Resolve takes a user-provided worktree identifier (full path, workspace name,
// or relative path) and resolves it to the full worktree path from the cache.
func Resolve(entries []Entry, input string) (string, error) {
	// Exact match against cached paths
	for _, e := range entries {
		if e.Path == input {
			return e.Path, nil
		}
	}

	// Workspace name match (compare basename of cached paths)
	for _, e := range entries {
		if filepath.Base(e.Path) == input {
			return e.Path, nil
		}
	}

	// Relative-to-bare-root match (handles slash-containing paths like feature/my-thing)
	bareRoot, err := BareRoot()
	if err == nil {
		candidate := filepath.Join(bareRoot, input)
		for _, e := range entries {
			if e.Path == candidate {
				return e.Path, nil
			}
		}
	}

	// Realpath match (resolve relative/symlinked paths)
	resolved, err := filepath.EvalSymlinks(input)
	if err == nil {
		resolved, err = filepath.Abs(resolved)
		if err == nil {
			for _, e := range entries {
				if e.Path == resolved {
					return resolved, nil
				}
			}
		}
	}

	return "", fmt.Errorf("'%s' is not a valid worktree", input)
}

// Validate checks if the input identifies a valid worktree and returns an
// error with the list of available worktrees if not.
func Validate(entries []Entry, input string) error {
	_, err := Resolve(entries, input)
	if err == nil {
		return nil
	}

	bareRoot, _ := BareRoot()
	names := make([]string, len(entries))
	for i, e := range entries {
		if bareRoot != "" {
			names[i] = strings.TrimPrefix(e.Path, bareRoot+string(os.PathSeparator))
		} else {
			names[i] = filepath.Base(e.Path)
		}
	}
	return fmt.Errorf("'%s' is not a valid worktree. Available worktrees:\n  %s",
		input, strings.Join(names, "\n  "))
}

// BranchFor returns the branch name for the given worktree path.
func BranchFor(entries []Entry, path string) string {
	resolved, _ := Resolve(entries, path)
	if resolved == "" {
		resolved = path
	}
	for _, e := range entries {
		if e.Path == resolved {
			return e.Branch
		}
	}
	return ""
}

// BareRoot returns the root directory of the bare repo structure (parent of .bare/).
func BareRoot() (string, error) {
	commonDir, err := git.Query("rev-parse", "--git-common-dir")
	if err != nil {
		return "", fmt.Errorf("not in a git repository: %w", err)
	}

	// Resolve to absolute path
	absDir, err := filepath.Abs(commonDir)
	if err != nil {
		return "", err
	}

	// Resolve symlinks (macOS /tmp -> /private/tmp)
	absDir, err = filepath.EvalSymlinks(absDir)
	if err != nil {
		return "", err
	}

	// Strip trailing /.bare
	return strings.TrimSuffix(absDir, string(os.PathSeparator)+".bare"), nil
}

// DefaultBranch returns the default branch name, preferring local lookup over network.
func DefaultBranch() string {
	// Try local symbolic-ref first (instant, no network)
	ref, err := git.Query("symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil && ref != "" {
		return strings.TrimPrefix(ref, "refs/remotes/origin/")
	}

	// Fall back to network call
	out, err := git.QueryCombined("remote", "show", "origin")
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "HEAD branch:"); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}
