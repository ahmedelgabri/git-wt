package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateSuccess(t *testing.T) {
	entries := []Entry{
		{Path: "/home/user/project/main", Branch: "main"},
		{Path: "/home/user/project/feature", Branch: "feature"},
	}
	if err := Validate(entries, "main"); err != nil {
		t.Errorf("Validate(main) = %v, want nil", err)
	}
}

func TestValidateFailure(t *testing.T) {
	entries := []Entry{
		{Path: "/home/user/project/main", Branch: "main"},
		{Path: "/home/user/project/feature", Branch: "feature"},
	}
	err := Validate(entries, "nonexistent")
	if err == nil {
		t.Fatal("Validate(nonexistent) should return error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not a valid worktree") {
		t.Errorf("error should contain 'not a valid worktree', got %q", msg)
	}
	if !strings.Contains(msg, "Available worktrees") {
		t.Errorf("error should contain 'Available worktrees', got %q", msg)
	}
}

func TestResolveAbsolutePathMatch(t *testing.T) {
	dir := t.TempDir()
	wtPath := filepath.Join(dir, "feature")
	os.MkdirAll(wtPath, 0o755)

	// Resolve symlinks so the entry path matches what EvalSymlinks returns
	resolved, err := filepath.EvalSymlinks(wtPath)
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}

	entries := []Entry{
		{Path: resolved, Branch: "feature"},
	}

	got, err := Resolve(entries, wtPath)
	if err != nil {
		t.Fatalf("Resolve(%s) error: %v", wtPath, err)
	}
	if got != resolved {
		t.Errorf("Resolve = %q, want %q", got, resolved)
	}
}

func TestDefaultBranchNoRepo(t *testing.T) {
	dir := t.TempDir()

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	branch := DefaultBranch()
	if branch != "" {
		t.Errorf("DefaultBranch() in non-git dir = %q, want empty", branch)
	}
}

func TestBareRootInBareStructure(t *testing.T) {
	dir := t.TempDir()

	// Create .bare as a real git bare repo so rev-parse works
	bareDir := filepath.Join(dir, ".bare")
	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}

	// Create .git file pointing to .bare
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	root, err := BareRoot()
	if err != nil {
		t.Fatalf("BareRoot() error: %v", err)
	}

	// Resolve symlinks for comparison (macOS /tmp -> /private/tmp)
	wantDir, _ := filepath.EvalSymlinks(dir)
	if root != wantDir {
		t.Errorf("BareRoot() = %q, want %q", root, wantDir)
	}
}
