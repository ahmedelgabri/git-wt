package worktree

import (
	"context"
	"strings"

	"github.com/ahmedelgabri/git-wt/internal/git"
)

// Entry represents a single worktree from git worktree list --porcelain.
type Entry struct {
	Path           string
	Branch         string
	Head           string // full object ID as reported by Git
	Detached       bool
	Locked         bool
	LockedReason   string
	Prunable       bool
	PrunableReason string
}

// List returns all worktrees (excluding the .bare entry) by parsing
// git worktree list --porcelain.
func List() ([]Entry, error) { return ListContext(context.Background()) }

func ListContext(ctx context.Context) ([]Entry, error) {
	out, err := git.QueryRawContext(ctx, "worktree", "list", "--porcelain", "-z")
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
	bare := false
	separator := "\n"
	if strings.Contains(output, "\x00") {
		separator = "\x00"
	}

	for line := range strings.SplitSeq(output, separator) {
		key, value, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			current.Path = value
		case "HEAD":
			current.Head = value
		case "branch":
			current.Branch = strings.TrimPrefix(value, "refs/heads/")
			current.Detached = false
		case "bare":
			bare = true
		case "detached":
			current.Branch = ""
			current.Detached = true
		case "locked":
			current.Locked = true
			current.LockedReason = value
		case "prunable":
			current.Prunable = true
			current.PrunableReason = value
		case "":
			if !bare && current.Path != "" {
				entries = append(entries, current)
			}
			current = Entry{}
			bare = false
		}
	}

	// Handle last entry (no trailing blank line)
	if !bare && current.Path != "" {
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
