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

	branch := DefaultBranch("")
	if branch != "" {
		t.Errorf("DefaultBranch(\"\") in non-git dir = %q, want empty", branch)
	}
}

// initTestRepo creates a git repo in dir with an initial commit and returns
// a cleanup function that restores the working directory.
func initTestRepo(t *testing.T, dir string) {
	t.Helper()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test User")
	run("commit", "--allow-empty", "-m", "initial commit")
}

func TestDefaultRemoteNoRemotes(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	got := DefaultRemote()
	if got != "" {
		t.Errorf("DefaultRemote() with no remotes = %q, want empty", got)
	}
}

func TestDefaultRemoteSingleNonOrigin(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)

	remoteDir := t.TempDir()
	cmd := exec.Command("git", "init", "--bare", remoteDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	cmd = exec.Command("git", "remote", "add", "upstream", remoteDir)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}

	got := DefaultRemote()
	if got != "upstream" {
		t.Errorf("DefaultRemote() with single remote 'upstream' = %q, want %q", got, "upstream")
	}
}

func TestDefaultRemoteMultipleWithBranchConfig(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)

	for _, name := range []string{"origin", "upstream"} {
		remoteDir := t.TempDir()
		cmd := exec.Command("git", "init", "--bare", remoteDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init --bare: %v\n%s", err, out)
		}
		cmd = exec.Command("git", "remote", "add", name, remoteDir)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git remote add %s: %v\n%s", name, err, out)
		}
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	// Configure branch.main.remote = upstream
	cmd := exec.Command("git", "config", "branch.main.remote", "upstream")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git config: %v\n%s", err, out)
	}

	got := DefaultRemote()
	if got != "upstream" {
		t.Errorf("DefaultRemote() with branch config = %q, want %q", got, "upstream")
	}
}

func TestDefaultRemoteMultipleOriginFallback(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)

	for _, name := range []string{"github", "origin"} {
		remoteDir := t.TempDir()
		cmd := exec.Command("git", "init", "--bare", remoteDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init --bare: %v\n%s", err, out)
		}
		cmd = exec.Command("git", "remote", "add", name, remoteDir)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git remote add %s: %v\n%s", name, err, out)
		}
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	got := DefaultRemote()
	if got != "origin" {
		t.Errorf("DefaultRemote() with multiple remotes including origin = %q, want %q", got, "origin")
	}
}

func TestDefaultRemoteMultipleNoOrigin(t *testing.T) {
	dir := t.TempDir()
	initTestRepo(t, dir)

	for _, name := range []string{"github", "gitlab"} {
		remoteDir := t.TempDir()
		cmd := exec.Command("git", "init", "--bare", remoteDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git init --bare: %v\n%s", err, out)
		}
		cmd = exec.Command("git", "remote", "add", name, remoteDir)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git remote add %s: %v\n%s", name, err, out)
		}
	}

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	got := DefaultRemote()
	// git remote returns alphabetically, so "github" comes first
	if got != "github" {
		t.Errorf("DefaultRemote() with no origin = %q, want %q", got, "github")
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
