package worktree

import (
	"path/filepath"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

// Entry represents a single worktree from git worktree list --porcelain.
type Entry struct {
	Path           string
	Branch         string
	Head           string // short SHA (7 chars)
	Detached       bool
	Locked         bool
	LockedReason   string
	Prunable       bool
	PrunableReason string
}

// List returns all worktrees (excluding the .bare entry) by parsing
// git worktree list --porcelain.
func List() ([]Entry, error) {
	out, err := git.Query("worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return ParsePorcelain(out), nil
}

// ParsePorcelain parses the output of git worktree list --porcelain into
// a slice of Entry, excluding the .bare entry.
func ParsePorcelain(output string) []Entry {
	if output == "" {
		return nil
	}

	var entries []Entry
	var current Entry

	for line := range strings.SplitSeq(output, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			sha := strings.TrimPrefix(line, "HEAD ")
			if len(sha) > 7 {
				sha = sha[:7]
			}
			current.Head = sha
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
			current.Detached = false
		case line == "detached":
			current.Branch = ""
			current.Detached = true
		case strings.HasPrefix(line, "locked"):
			current.Locked = true
			current.LockedReason = strings.TrimSpace(strings.TrimPrefix(line, "locked"))
		case strings.HasPrefix(line, "prunable"):
			current.Prunable = true
			current.PrunableReason = strings.TrimSpace(strings.TrimPrefix(line, "prunable"))
		case line == "":
			if current.Path != "" && filepath.Base(current.Path) != ".bare" {
				entries = append(entries, current)
			}
			current = Entry{}
		}
	}

	// Handle last entry (no trailing blank line)
	if current.Path != "" && filepath.Base(current.Path) != ".bare" {
		entries = append(entries, current)
	}

	return entries
}

// FindByBranch returns the entry for the given branch name, or nil if not found.
func FindByBranch(entries []Entry, branch string) *Entry {
	for i := range entries {
		if entries[i].Branch == branch {
			return &entries[i]
		}
	}
	return nil
}
