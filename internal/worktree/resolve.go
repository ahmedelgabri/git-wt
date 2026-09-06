package worktree

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

type ambiguousWorktreeError struct {
	input      string
	candidates []string
}

func (e *ambiguousWorktreeError) Error() string {
	return fmt.Sprintf("'%s' is ambiguous. Matches multiple worktrees:\n  %s",
		e.input, strings.Join(e.candidates, "\n  "))
}

// Resolve takes a user-provided worktree identifier (full path, workspace name,
// or relative path) and resolves it to the full worktree path from the cache.
func Resolve(entries []Entry, input string) (string, error) {
	// Exact match against cached paths.
	for _, e := range entries {
		if e.Path == input {
			return e.Path, nil
		}
	}

	// Relative-to-bare-root match (handles slash-containing paths like feature/my-thing).
	if bareRoot, err := BareRoot(); err == nil {
		candidate := filepath.Join(bareRoot, input)
		for _, e := range entries {
			if e.Path == candidate {
				return e.Path, nil
			}
		}
	}

	// Realpath match (resolve relative/symlinked paths).
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

	// Workspace name match (compare basename of cached paths) only if unique.
	var basenameMatches []Entry
	for _, e := range entries {
		if filepath.Base(e.Path) == input {
			basenameMatches = append(basenameMatches, e)
		}
	}
	if len(basenameMatches) == 1 {
		return basenameMatches[0].Path, nil
	}
	if len(basenameMatches) > 1 {
		return "", &ambiguousWorktreeError{
			input:      input,
			candidates: displayNames(basenameMatches),
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

	var ambiguous *ambiguousWorktreeError
	if errors.As(err, &ambiguous) {
		return err
	}

	return fmt.Errorf("'%s' is not a valid worktree. Available worktrees:\n  %s",
		input, strings.Join(displayNames(entries), "\n  "))
}

// FindByPath returns the worktree entry for the given path or identifier.
func FindByPath(entries []Entry, path string) *Entry {
	resolved, _ := Resolve(entries, path)
	if resolved == "" {
		resolved = path
	}
	for i := range entries {
		if entries[i].Path == resolved {
			return &entries[i]
		}
	}
	return nil
}

// BranchFor returns the branch name for the given worktree path.
func BranchFor(entries []Entry, path string) string {
	entry := FindByPath(entries, path)
	if entry == nil {
		return ""
	}
	return entry.Branch
}

func displayNames(entries []Entry) []string {
	bareRoot, _ := BareRoot()
	out := make([]string, len(entries))
	for i, e := range entries {
		if bareRoot != "" {
			out[i] = strings.TrimPrefix(e.Path, bareRoot+string(os.PathSeparator))
			continue
		}
		out[i] = e.Path
	}
	return out
}

// BareRoot returns the root directory of the bare repo structure (parent of .bare/).
func BareRoot() (string, error) {
	commonDir, err := git.QueryPath("rev-parse", "--git-common-dir")
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

	if filepath.Base(absDir) != ".bare" {
		return "", fmt.Errorf("unsupported repository layout: expected a .bare directory; use git wt migrate for a standard repository")
	}
	return filepath.Dir(absDir), nil
}

// DefaultRemote returns the best remote name for the current repository.
// If exactly one remote exists, it is returned. With multiple remotes, it
// checks the current branch's configured remote, then falls back to "origin"
// if present, then the first remote alphabetically. Returns "" if no remotes.
func DefaultRemote() string { return DefaultRemoteIn("") }

func DefaultRemoteIn(dir string) string { return DefaultRemoteInContext(context.Background(), dir) }

func DefaultRemoteInContext(ctx context.Context, dir string) string {
	out, err := git.QueryInContext(ctx, dir, "remote")
	if err != nil || out == "" {
		return ""
	}

	var remotes []string
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			remotes = append(remotes, line)
		}
	}

	if len(remotes) == 0 {
		return ""
	}
	if len(remotes) == 1 {
		return remotes[0]
	}

	// Check the current branch's configured remote
	branch, err := git.QueryInContext(ctx, dir, "branch", "--show-current")
	if err == nil && branch != "" {
		configured, err := git.QueryInContext(ctx, dir, "config", fmt.Sprintf("branch.%s.remote", branch))
		if err == nil && configured != "" {
			for _, remote := range remotes {
				if configured == remote {
					return remote
				}
			}
		}
	}

	// Fall back to "origin" if it exists
	for _, r := range remotes {
		if r == "origin" {
			return r
		}
	}

	return remotes[0]
}

// DefaultBranch returns the default branch name, preferring local lookup over network.
func DefaultBranch(remote string) string { return DefaultBranchIn("", remote) }

func DefaultBranchIn(dir, remote string) string {
	return DefaultBranchInContext(context.Background(), dir, remote)
}

func DefaultBranchInContext(ctx context.Context, dir, remote string) string {
	if remote == "" {
		return ""
	}

	// Try local symbolic-ref first (instant, no network)
	ref, err := git.QueryInContext(ctx, dir, "symbolic-ref", fmt.Sprintf("refs/remotes/%s/HEAD", remote))
	if err == nil && ref != "" {
		return strings.TrimPrefix(ref, fmt.Sprintf("refs/remotes/%s/", remote))
	}

	// Bound network discovery, including calls from non-interactive commands.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := git.QueryInContext(ctx, dir, "ls-remote", "--symref", remote, "HEAD")
	if err != nil {
		return ""
	}
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "ref: refs/heads/"); ok {
			branch, _, _ := strings.Cut(after, "\t")
			return branch
		}
	}
	return ""
}
