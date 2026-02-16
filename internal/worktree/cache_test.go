package worktree

import (
	"testing"
)

const samplePorcelain = `worktree /home/user/project/.bare
HEAD abc1234567890abcdef1234567890abcdef123456
branch refs/heads/main
bare

worktree /home/user/project/main
HEAD abc1234567890abcdef1234567890abcdef123456
branch refs/heads/main

worktree /home/user/project/feature-a
HEAD def4567890abcdef1234567890abcdef12345678
branch refs/heads/feature-a

worktree /home/user/project/detached-wt
HEAD 999888777666555444333222111000aaabbbccc
detached

`

func TestParsePorcelain(t *testing.T) {
	entries := ParsePorcelain(samplePorcelain)

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries (excluding .bare), got %d", len(entries))
	}

	tests := []struct {
		idx    int
		path   string
		branch string
		head   string
	}{
		{0, "/home/user/project/main", "main", "abc1234"},
		{1, "/home/user/project/feature-a", "feature-a", "def4567"},
		{2, "/home/user/project/detached-wt", "(detached)", "9998887"},
	}

	for _, tt := range tests {
		e := entries[tt.idx]
		if e.Path != tt.path {
			t.Errorf("entry[%d].Path = %q, want %q", tt.idx, e.Path, tt.path)
		}
		if e.Branch != tt.branch {
			t.Errorf("entry[%d].Branch = %q, want %q", tt.idx, e.Branch, tt.branch)
		}
		if e.Head != tt.head {
			t.Errorf("entry[%d].Head = %q, want %q", tt.idx, e.Head, tt.head)
		}
	}
}

func TestParsePorcelainEmpty(t *testing.T) {
	entries := ParsePorcelain("")
	if entries != nil {
		t.Errorf("expected nil for empty input, got %v", entries)
	}
}

func TestParsePorcelainNoTrailingNewline(t *testing.T) {
	input := `worktree /home/user/project/main
HEAD abc1234567890abcdef1234567890abcdef123456
branch refs/heads/main`

	entries := ParsePorcelain(input)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Path != "/home/user/project/main" {
		t.Errorf("Path = %q, want /home/user/project/main", entries[0].Path)
	}
}

func TestFindByBranch(t *testing.T) {
	entries := ParsePorcelain(samplePorcelain)

	e := FindByBranch(entries, "feature-a")
	if e == nil {
		t.Fatal("FindByBranch returned nil for feature-a")
	}
	if e.Path != "/home/user/project/feature-a" {
		t.Errorf("Path = %q, want /home/user/project/feature-a", e.Path)
	}

	e = FindByBranch(entries, "nonexistent")
	if e != nil {
		t.Errorf("FindByBranch should return nil for nonexistent branch, got %v", e)
	}
}

func TestResolve(t *testing.T) {
	entries := ParsePorcelain(samplePorcelain)

	// Exact match
	path, err := Resolve(entries, "/home/user/project/main")
	if err != nil {
		t.Fatalf("Resolve exact match error: %v", err)
	}
	if path != "/home/user/project/main" {
		t.Errorf("Resolve = %q, want /home/user/project/main", path)
	}

	// Basename match
	path, err = Resolve(entries, "feature-a")
	if err != nil {
		t.Fatalf("Resolve basename match error: %v", err)
	}
	if path != "/home/user/project/feature-a" {
		t.Errorf("Resolve = %q, want /home/user/project/feature-a", path)
	}

	// Not found
	_, err = Resolve(entries, "nonexistent")
	if err == nil {
		t.Error("Resolve should return error for nonexistent worktree")
	}
}

func TestBranchFor(t *testing.T) {
	entries := ParsePorcelain(samplePorcelain)

	branch := BranchFor(entries, "/home/user/project/feature-a")
	if branch != "feature-a" {
		t.Errorf("BranchFor = %q, want feature-a", branch)
	}

	branch = BranchFor(entries, "main")
	if branch != "main" {
		t.Errorf("BranchFor basename = %q, want main", branch)
	}

	branch = BranchFor(entries, "nonexistent")
	if branch != "" {
		t.Errorf("BranchFor nonexistent = %q, want empty", branch)
	}
}
