package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ahmedelgabri/git-wt/internal/worktree"
)

func TestMoveContents(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o644)
	os.MkdirAll(filepath.Join(src, "subdir"), 0o755)
	os.WriteFile(filepath.Join(src, "subdir", "nested.txt"), []byte("world"), 0o644)

	if err := moveContents(src, dst); err != nil {
		t.Fatalf("moveContents error: %v", err)
	}

	// Entries should exist in dst
	if _, err := os.Stat(filepath.Join(dst, "file.txt")); err != nil {
		t.Error("file.txt should exist in dst")
	}
	if _, err := os.Stat(filepath.Join(dst, "subdir", "nested.txt")); err != nil {
		t.Error("subdir/nested.txt should exist in dst")
	}

	// Entries should be absent from src
	entries, _ := os.ReadDir(src)
	if len(entries) != 0 {
		t.Errorf("src should be empty, got %d entries", len(entries))
	}
}

func TestMoveContentsEmptySrc(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	if err := moveContents(src, dst); err != nil {
		t.Fatalf("moveContents empty src error: %v", err)
	}

	entries, _ := os.ReadDir(dst)
	if len(entries) != 0 {
		t.Errorf("dst should be empty, got %d entries", len(entries))
	}
}

func TestMoveContentsNonExistentSrc(t *testing.T) {
	err := moveContents(filepath.Join(t.TempDir(), "nonexistent"), t.TempDir())
	if err == nil {
		t.Error("moveContents with nonexistent src should return error")
	}
}

func TestCopyFileSimple(t *testing.T) {
	src := filepath.Join(t.TempDir(), "src.txt")
	dst := filepath.Join(t.TempDir(), "dst.txt")

	os.WriteFile(src, []byte("content"), 0o644)

	if err := copyFileSimple(src, dst); err != nil {
		t.Fatalf("copyFileSimple error: %v", err)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(data) != "content" {
		t.Errorf("dst content = %q, want %q", data, "content")
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("permissions = %o, want 644", info.Mode().Perm())
	}
}

func TestCopyFileSimpleNonExistent(t *testing.T) {
	err := copyFileSimple(filepath.Join(t.TempDir(), "nonexistent"), filepath.Join(t.TempDir(), "dst"))
	if err == nil {
		t.Error("copyFileSimple with nonexistent src should return error")
	}
}

func TestRestoreBackup(t *testing.T) {
	backup := t.TempDir()
	repoRoot := t.TempDir()

	os.WriteFile(filepath.Join(backup, "file.txt"), []byte("backup"), 0o644)
	os.MkdirAll(filepath.Join(backup, "subdir"), 0o755)

	restoreBackup(backup, repoRoot)

	if _, err := os.Stat(filepath.Join(repoRoot, "file.txt")); err != nil {
		t.Error("file.txt should exist in repoRoot after restore")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "subdir")); err != nil {
		t.Error("subdir should exist in repoRoot after restore")
	}

	// Backup dir should be removed
	if _, err := os.Stat(backup); !os.IsNotExist(err) {
		t.Error("backup dir should be removed after restore")
	}
}

func TestRestoreBackupNonExistent(t *testing.T) {
	// Should return silently without error
	restoreBackup(filepath.Join(t.TempDir(), "nonexistent"), t.TempDir())
}

func TestIsKnownCommand(t *testing.T) {
	known := []string{"add", "clone", "help", "--help", "-h"}
	for _, name := range known {
		if !isKnownCommand(name) {
			t.Errorf("isKnownCommand(%q) = false, want true", name)
		}
	}

	unknown := []string{"nonexistent", ""}
	for _, name := range unknown {
		if isKnownCommand(name) {
			t.Errorf("isKnownCommand(%q) = true, want false", name)
		}
	}

	// Test alias
	if !isKnownCommand("rm") {
		t.Error("isKnownCommand(rm) = false, want true (alias for remove)")
	}
}

func TestEntriesToPickerItems(t *testing.T) {
	entries := []worktree.Entry{
		{Path: "/tmp/project/main", Branch: "main", Head: "abc1234"},
		{Path: "/tmp/project/detached-wt", Branch: "(detached)", Head: "def5678"},
		{Path: "/tmp/project/no-branch", Branch: "", Head: "111aaaa"},
	}

	items := entriesToPickerItems(entries)

	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// Normal branch: label should contain branch in brackets
	if items[0].Value != "/tmp/project/main" {
		t.Errorf("item[0].Value = %q, want /tmp/project/main", items[0].Value)
	}
	if items[0].Label == "" {
		t.Error("item[0].Label should not be empty")
	}

	// Detached HEAD: label should contain "detached HEAD"
	if items[1].Value != "/tmp/project/detached-wt" {
		t.Errorf("item[1].Value = %q, want /tmp/project/detached-wt", items[1].Value)
	}

	// Empty branch
	if items[2].Value != "/tmp/project/no-branch" {
		t.Errorf("item[2].Value = %q, want /tmp/project/no-branch", items[2].Value)
	}

	// All items should have a Desc
	for i, item := range items {
		if item.Desc == "" {
			t.Errorf("item[%d].Desc should not be empty", i)
		}
	}
}

func TestPreviewWorktreeCmdStr(t *testing.T) {
	got := previewWorktreeCmdStr("remove")
	if !strings.Contains(got, "_preview worktree {1} remove") {
		t.Errorf("previewWorktreeCmdStr(remove) = %q, want to contain '_preview worktree {1} remove'", got)
	}

	got = previewWorktreeCmdStr("destroy")
	if !strings.Contains(got, "_preview worktree {1} destroy") {
		t.Errorf("previewWorktreeCmdStr(destroy) = %q, want to contain '_preview worktree {1} destroy'", got)
	}
}

func TestPreviewBranchCmdStr(t *testing.T) {
	got := previewBranchCmdStr()
	if !strings.Contains(got, "_preview branch {1}") {
		t.Errorf("previewBranchCmdStr() = %q, want to contain '_preview branch {1}'", got)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmd := exec.Command("git", "init", "-b", "main", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	// Create an initial commit so HEAD exists
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("init"), 0o644)
	run("add", "README.md")
	run("-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-m", "init")
	return dir
}

func TestCheckGitDiffClean(t *testing.T) {
	repo := initGitRepo(t)
	if err := checkGitDiff(repo); err != nil {
		t.Errorf("checkGitDiff on clean repo = %v, want nil", err)
	}
}

func TestCheckGitDiffDirty(t *testing.T) {
	repo := initGitRepo(t)
	// Modify a tracked file to create a diff
	os.WriteFile(filepath.Join(repo, "README.md"), []byte("modified"), 0o644)
	if err := checkGitDiff(repo); err == nil {
		t.Error("checkGitDiff on dirty repo should return error")
	}
}

func TestMoveContentsRenameFail(t *testing.T) {
	src := t.TempDir()
	os.WriteFile(filepath.Join(src, "file.txt"), []byte("hello"), 0o644)

	// dst is a file, not a directory - rename into it will fail
	dstFile := filepath.Join(t.TempDir(), "not-a-dir")
	os.WriteFile(dstFile, []byte("x"), 0o644)

	err := moveContents(src, dstFile)
	if err == nil {
		t.Error("moveContents to a file dst should return error")
	}
}

func TestEntriesToPickerItemsWithBareRoot(t *testing.T) {
	// Set up a bare repo structure so entriesToPickerItems uses relative paths
	dir := t.TempDir()
	bareDir := filepath.Join(dir, ".bare")
	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	// Resolve symlinks for path comparison
	resolved, _ := filepath.EvalSymlinks(dir)

	entries := []worktree.Entry{
		{Path: filepath.Join(resolved, "main"), Branch: "main", Head: "abc1234"},
		{Path: filepath.Join(resolved, "feat", "login"), Branch: "feat/login", Head: "def5678"},
	}

	items := entriesToPickerItems(entries)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// With a bare root, labels should use relative paths, not just basename
	if !strings.Contains(items[0].Label, "main") {
		t.Errorf("item[0].Label = %q, want to contain 'main'", items[0].Label)
	}
	if !strings.Contains(items[1].Label, "feat/login") {
		t.Errorf("item[1].Label = %q, want to contain 'feat/login'", items[1].Label)
	}
}

func TestGenerateWorktreePreviewRemoveMode(t *testing.T) {
	// Set up a bare repo with a worktree so generateWorktreePreview can query it
	dir := t.TempDir()
	bareDir := filepath.Join(dir, ".bare")

	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	// Create a worktree
	wtPath := filepath.Join(dir, "main")
	run("worktree", "add", "-b", "main", wtPath)

	// Create a commit inside the worktree
	os.WriteFile(filepath.Join(wtPath, "file.txt"), []byte("hello"), 0o644)
	c := exec.Command("git", "add", "file.txt")
	c.Dir = wtPath
	c.CombinedOutput()
	c = exec.Command("git", "-c", "user.name=Test", "-c", "user.email=test@test.com", "commit", "-m", "init")
	c.Dir = wtPath
	c.CombinedOutput()

	out := generateWorktreePreview(wtPath, "remove")
	if !strings.Contains(out, "Worktree") {
		t.Errorf("preview should contain 'Worktree', got %q", out)
	}
	if !strings.Contains(out, "Status") {
		t.Errorf("preview should contain 'Status', got %q", out)
	}
	if !strings.Contains(out, "Recent Commits") {
		t.Errorf("preview should contain 'Recent Commits', got %q", out)
	}
	// Should NOT contain destroy mode header
	if strings.Contains(out, "DESTROY MODE") {
		t.Error("remove mode should not contain 'DESTROY MODE'")
	}
}

func TestGenerateWorktreePreviewDestroyMode(t *testing.T) {
	dir := t.TempDir()
	bareDir := filepath.Join(dir, ".bare")

	cmd := exec.Command("git", "init", "--bare", bareDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %v\n%s", err, out)
	}
	os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ./.bare\n"), 0o644)

	orig, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
	os.Chdir(dir)

	wtPath := filepath.Join(dir, "feat")
	c := exec.Command("git", "worktree", "add", "-b", "feat", wtPath)
	c.Dir = dir
	c.CombinedOutput()

	out := generateWorktreePreview(wtPath, "destroy")
	if !strings.Contains(out, "DESTROY MODE") {
		t.Errorf("destroy mode should contain 'DESTROY MODE', got %q", out)
	}
	if !strings.Contains(out, "Delete remote branch") {
		t.Errorf("destroy mode should contain 'Delete remote branch', got %q", out)
	}
}
